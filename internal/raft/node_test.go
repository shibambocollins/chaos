package raft

import (
	"math/rand"
	"testing"
)

func TestStep_StaleTimerGenerationIsIgnored(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	rng := rand.New(rand.NewSource(1))

	// Simulate a reset having already bumped the node's generation to 5.
	// An event scheduled under an older generation (3) must be dropped.
	n.timerGen[TimerElection] = 5

	stale := Event{
		Kind:           EventTimerFire,
		TimerKindField: TimerElection,
		TimerGen:       3,
	}

	out := n.Step(stale, rng)
	if out != nil {
		t.Fatalf("expected nil Outbound for stale timer generation, got %v", out)
	}
}

func TestTimerGeneration_ReflectsBumpsFromHandlers(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	rng := rand.New(rand.NewSource(1))

	if n.TimerGeneration(TimerElection) != 0 {
		t.Fatalf("expected initial generation 0, got %d", n.TimerGeneration(TimerElection))
	}

	n.Step(Event{Kind: EventTimerFire, TimerKindField: TimerElection, TimerGen: 0}, rng)

	if n.TimerGeneration(TimerElection) != 1 {
		t.Fatalf("expected generation 1 after an election timeout bumped it, got %d", n.TimerGeneration(TimerElection))
	}
}

func TestRestart_ResetsVolatileStateButPreservesPersistent(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.CurrentTerm = 7
	n.VotedFor = 2
	n.Log = []LogEntry{{Term: 7, Index: 1, Command: []byte("a")}}

	n.Role = Leader
	n.CommitIndex = 1
	n.LastApplied = 1
	n.NextIndex = map[int]uint64{2: 2, 3: 2}
	n.MatchIndex = map[int]uint64{2: 1, 3: 1}
	beforeElectionGen := n.TimerGeneration(TimerElection)
	beforeHeartbeatGen := n.TimerGeneration(TimerHeartbeat)

	rng := rand.New(rand.NewSource(1))
	out := n.Restart(rng)

	if n.CurrentTerm != 7 || n.VotedFor != 2 || len(n.Log) != 1 {
		t.Fatalf("expected persistent state untouched, got term=%d votedFor=%d logLen=%d", n.CurrentTerm, n.VotedFor, len(n.Log))
	}

	if n.Role != Follower {
		t.Fatalf("expected Role reset to Follower, got %v", n.Role)
	}
	if n.CommitIndex != 0 || n.LastApplied != 0 {
		t.Fatalf("expected CommitIndex/LastApplied reset to 0, got %d/%d", n.CommitIndex, n.LastApplied)
	}
	if n.NextIndex != nil || n.MatchIndex != nil || n.VotesReceived != nil {
		t.Fatalf("expected leader/candidate-only volatile maps reset to nil")
	}

	if n.TimerGeneration(TimerElection) == beforeElectionGen || n.TimerGeneration(TimerHeartbeat) == beforeHeartbeatGen {
		t.Fatalf("expected both timer generations bumped on restart")
	}

	if len(out) != 1 || out[0].Kind != OutResetTimer || out[0].TimerKindField != TimerElection {
		t.Fatalf("expected a single OutResetTimer for TimerElection, got %+v", out)
	}
}

func TestRole_JSONRoundTrip(t *testing.T) {
	for _, want := range []Role{Follower, Candidate, Leader} {
		data, err := want.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON(%v): %v", want, err)
		}
		var got Role
		if err := got.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON(%s): %v", data, err)
		}
		if got != want {
			t.Fatalf("round trip: want %v, got %v", want, got)
		}
	}
}

func TestNewNodeState_PeersAreSortedAndExcludeSelf(t *testing.T) {
	n := NewNodeState(2, []int{5, 1, 3})

	want := []int{1, 3, 5}
	if len(n.Peers) != len(want) {
		t.Fatalf("expected %d peers, got %d", len(want), len(n.Peers))
	}
	for i, id := range want {
		if n.Peers[i] != id {
			t.Fatalf("expected sorted peers %v, got %v", want, n.Peers)
		}
	}
}
