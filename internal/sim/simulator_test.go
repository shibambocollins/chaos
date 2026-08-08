package sim

import (
	"math/rand"
	"testing"

	"chaos/internal/raft"
)

func TestNewSimulator_SortsNodeIDsAndMarksAllAlive(t *testing.T) {
	nodes := map[int]*raft.NodeState{
		3: raft.NewNodeState(3, []int{1, 2}),
		1: raft.NewNodeState(1, []int{2, 3}),
		2: raft.NewNodeState(2, []int{1, 3}),
	}

	s := NewSimulator(nodes, rand.New(rand.NewSource(1)))

	wantIDs := []int{1, 2, 3}
	if len(s.nodeIDs) != len(wantIDs) {
		t.Fatalf("expected %d node IDs, got %d", len(wantIDs), len(s.nodeIDs))
	}
	for i, id := range wantIDs {
		if s.nodeIDs[i] != id {
			t.Fatalf("expected nodeIDs %v, got %v", wantIDs, s.nodeIDs)
		}
	}

	for _, id := range wantIDs {
		if !s.alive[id] {
			t.Fatalf("expected node %d to start alive", id)
		}
	}
}
