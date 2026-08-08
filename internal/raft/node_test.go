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
