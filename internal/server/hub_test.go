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
	nodes := map[int]*raft.NodeState{
		1: raft.NewNodeState(1, []int{2, 3}),
		2: raft.NewNodeState(2, []int{1, 3}),
		3: raft.NewNodeState(3, []int{1, 2}),
	}
	s := sim.NewSimulator(nodes, rand.New(rand.NewSource(1)))
	for id := range nodes {
		s.Schedule(raft.Event{At: 0, NodeID: id, Kind: raft.EventTimerFire, TimerKindField: raft.TimerElection, TimerGen: 0})
	}

	hub := NewHub(s, 5, 5*time.Millisecond)
	go hub.Run()
	t.Cleanup(hub.Stop)
	return hub
}

// readUntil reads snapshots from ch until cond reports true, or fails the
// test after timeout. Tests can't safely read the Hub's Simulator directly
// while Run's goroutine owns it — this is the same path a real client
// uses, which is the point: it proves the Hub's concurrency story actually
// works, not just that the underlying Simulator logic does (already proven
// exhaustively in internal/sim's own tests).
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
	readUntil(t, ch, 5*time.Second, func(r sim.Report) bool {
		for _, n := range r.Nodes {
			if n.Role == raft.Leader {
				return true
			}
		}
		return false
	})

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

	// Let an initial leader settle before partitioning, otherwise every
	// node is symmetric and this test can't tell partition from noise.
	readUntil(t, ch, 5*time.Second, func(r sim.Report) bool {
		for _, n := range r.Nodes {
			if n.Role == raft.Leader {
				return true
			}
		}
		return false
	})

	hub.submit(action{kind: actionPartition, groups: [][]int{{1}, {2, 3}}})
	// No direct observable signal from partitioning alone in this
	// snapshot shape beyond it not crashing — Heal should still bring
	// everything back to a single, functioning cluster afterward.
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

	readUntil(t, ch, 5*time.Second, func(r sim.Report) bool {
		for _, n := range r.Nodes {
			if n.Role == raft.Leader {
				return true
			}
		}
		return false
	})
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
