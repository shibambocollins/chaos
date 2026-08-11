package server

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	"chaos/internal/raft"
	"chaos/internal/sim"
)

func newTestHub(t *testing.T) *Hub {
	t.Helper()
	return newTestHubN(t, 3, 300*time.Millisecond)
}

// newTestHubN builds an n-node cluster (IDs 1..n) behind a fresh Hub and
// starts Run in the background.
func newTestHubN(t *testing.T, n int, favorDeadline time.Duration) *Hub {
	t.Helper()
	ids := make([]int, n)
	nodes := make(map[int]*raft.NodeState, n)
	for i := range ids {
		ids[i] = i + 1
	}
	for _, id := range ids {
		var peers []int
		for _, other := range ids {
			if other != id {
				peers = append(peers, other)
			}
		}
		nodes[id] = raft.NewNodeState(id, peers)
	}
	s := sim.NewSimulator(nodes, rand.New(rand.NewSource(1)))
	for _, id := range ids {
		s.Schedule(raft.Event{At: 0, NodeID: id, Kind: raft.EventTimerFire, TimerKindField: raft.TimerElection, TimerGen: 0})
	}

	hub := NewHub(s, ids, 5, 5*time.Millisecond, favorDeadline)
	go hub.Run()
	t.Cleanup(hub.Stop)
	return hub
}

// readUntil reads snapshots from ch until cond reports true, or fails the
// test after timeout. Tests can't safely read the Hub's Simulator directly
// while Run's goroutine owns it — reading through the channel is the same
// path a real client uses, which is the point: it proves the Hub's
// concurrency story actually works, not just that the underlying Simulator
// logic does (already proven exhaustively in internal/sim's own tests).
func readUntil(t *testing.T, ch chan []byte, timeout time.Duration, cond func(sim.Report) bool) sim.Report {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case data := <-ch:
			var report sim.Report
			if err := json.Unmarshal(data, &report); err != nil {
				t.Fatalf("failed to decode snapshot: %v", err)
			}
			if cond(report) {
				return report
			}
		case <-deadline:
			t.Fatalf("timed out after %s waiting for expected condition", timeout)
			return sim.Report{}
		}
	}
}

func nodeAlive(r sim.Report, id int) bool {
	for _, n := range r.Nodes {
		if n.ID == id {
			return n.Alive
		}
	}
	return false
}

func hasLeader(r sim.Report) bool {
	for _, n := range r.Nodes {
		if n.Role == raft.Leader {
			return true
		}
	}
	return false
}

func TestHub_KillIsReflectedInSnapshot(t *testing.T) {
	hub := newTestHub(t)
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	hub.submit(action{kind: actionKill, nodeID: 2})

	readUntil(t, ch, 2*time.Second, func(r sim.Report) bool {
		return !nodeAlive(r, 2)
	})
}

func TestHub_RestartBringsNodeBackAlive(t *testing.T) {
	hub := newTestHub(t)
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	hub.submit(action{kind: actionKill, nodeID: 2})
	readUntil(t, ch, 2*time.Second, func(r sim.Report) bool { return !nodeAlive(r, 2) })

	hub.submit(action{kind: actionRestart, nodeID: 2})
	readUntil(t, ch, 2*time.Second, func(r sim.Report) bool { return nodeAlive(r, 2) })
}

func TestHub_ClientRequestEventuallyApplied(t *testing.T) {
	hub := newTestHub(t)
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	// Wait for a leader to emerge from the natural election timers first —
	// no leader means the client request has nowhere to go.
	readUntil(t, ch, 5*time.Second, hasLeader)

	hub.submit(action{kind: actionClientRequest, command: []byte("x=1")})

	readUntil(t, ch, 5*time.Second, func(r sim.Report) bool {
		for _, n := range r.Nodes {
			if n.AppliedCount > 0 {
				return true
			}
		}
		return false
	})
}

func TestHub_PartitionAndHeal(t *testing.T) {
	hub := newTestHub(t)
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	readUntil(t, ch, 5*time.Second, hasLeader)

	hub.submit(action{kind: actionPartition, groups: [][]int{{1}, {2, 3}}})
	// No direct observable signal from partitioning alone in this snapshot
	// shape beyond it not crashing — Heal should still bring everything
	// back to a single, functioning cluster afterward.
	hub.submit(action{kind: actionHeal})

	hub.submit(action{kind: actionClientRequest, command: []byte("after-heal")})
	readUntil(t, ch, 5*time.Second, func(r sim.Report) bool {
		for _, n := range r.Nodes {
			if n.AppliedCount > 0 {
				return true
			}
		}
		return false
	})
}

// TestHub_ElectionProgressesAcrossAQuietGap is a regression test for a
// real bug: the ticker horizon must advance independently of sim.Now(),
// not be recomputed as sim.Now()+tickIncrement each tick. That formula
// stalls forever the moment the Simulator goes quiet for longer than one
// tickIncrement (e.g. every node mid-election-timeout, nothing due for
// another 15-30 ticks) — sim.Now() doesn't move without an event to
// process, so the recomputed horizon never reaches far enough ahead to
// find the next one. A real election has exactly this kind of gap, so
// reaching a leader here exercises the fix directly.
func TestHub_ElectionProgressesAcrossAQuietGap(t *testing.T) {
	hub := newTestHub(t)
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	readUntil(t, ch, 5*time.Second, hasLeader)
}

func TestHub_MultipleSubscribersAllReceiveUpdates(t *testing.T) {
	hub := newTestHub(t)
	chA := hub.Subscribe()
	chB := hub.Subscribe()
	defer hub.Unsubscribe(chA)
	defer hub.Unsubscribe(chB)

	hub.submit(action{kind: actionKill, nodeID: 3})

	readUntil(t, chA, 2*time.Second, func(r sim.Report) bool { return !nodeAlive(r, 3) })
	readUntil(t, chB, 2*time.Second, func(r sim.Report) bool { return !nodeAlive(r, 3) })
}

// TestHub_FavorShrinksTheRacingFieldToQuorum is the core claim behind the
// "improve this computer's odds" action, stated precisely: on a 5-node
// cluster (quorum 3), favoring node 4 should down exactly two of its
// rivals, leaving node 4 to race only the (quorum-1) = 2 it kept alive
// instead of all 4 — better odds (1-in-3 instead of 1-in-5), not a
// guaranteed win. Real independent election timers still decide who among
// the survivors actually wins, and this Hub is wall-clock-driven, so this
// test asserts the mechanism (field shrinks, target stays a candidate,
// everyone is eventually restored), not a specific winner — asserting a
// specific winner here would be asserting something the design doesn't
// promise, not verifying it.
func TestHub_FavorShrinksTheRacingFieldToQuorum(t *testing.T) {
	hub := newTestHubN(t, 5, 300*time.Millisecond)
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	hub.submit(action{kind: actionFavor, nodeID: 4})

	r := readUntil(t, ch, 2*time.Second, func(r sim.Report) bool {
		alive := 0
		for _, n := range r.Nodes {
			if n.Alive {
				alive++
			}
		}
		return alive == 3
	})
	if !nodeAlive(r, 4) {
		t.Fatalf("favor downed its own target: %+v", r.Nodes)
	}

	// Someone among the surviving three (not necessarily node 4) wins.
	readUntil(t, ch, 3*time.Second, hasLeader)

	// The favor should have resolved itself: both downed rivals restored.
	readUntil(t, ch, 2*time.Second, func(r sim.Report) bool {
		alive := 0
		for _, n := range r.Nodes {
			if n.Alive {
				alive++
			}
		}
		return alive == 5
	})
}

// TestHub_FavorGivesUpAtDeadline exercises the give-up path directly: with
// favorDeadline set to almost nothing, it will virtually always expire
// before any election resolves, so resolveFavor must restore the downed
// rivals on the timeout branch, not just the "target won" branch — a
// favor must never strand nodes offline just because the target lost or
// the race never finished.
func TestHub_FavorGivesUpAtDeadline(t *testing.T) {
	hub := newTestHubN(t, 5, 1*time.Millisecond)
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	hub.submit(action{kind: actionFavor, nodeID: 2})

	readUntil(t, ch, 2*time.Second, func(r sim.Report) bool {
		alive := 0
		for _, n := range r.Nodes {
			if n.Alive {
				alive++
			}
		}
		return alive == 5
	})
}

// TestHub_FavorGivesUpAndRestoresIfTargetIsAlreadyDead is a regression
// guard on the early-return in startFavor: favoring a dead node must not
// down anyone (there is nothing to gain), and must not leave a dangling
// pendingFavor that resolveFavor would spin on forever.
func TestHub_FavorGivesUpAndRestoresIfTargetIsAlreadyDead(t *testing.T) {
	hub := newTestHubN(t, 5, 300*time.Millisecond)
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	hub.submit(action{kind: actionKill, nodeID: 4})
	readUntil(t, ch, 2*time.Second, func(r sim.Report) bool { return !nodeAlive(r, 4) })

	hub.submit(action{kind: actionFavor, nodeID: 4})

	// Every other node should stay alive throughout — nothing to favor.
	readUntil(t, ch, 1*time.Second, func(r sim.Report) bool {
		for _, id := range []int{1, 2, 3, 5} {
			if !nodeAlive(r, id) {
				t.Fatalf("favoring an already-dead node downed node %d, which should never happen", id)
			}
		}
		return true
	})
}
