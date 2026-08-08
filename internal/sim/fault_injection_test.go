package sim

import (
	"math/rand"
	"testing"

	"chaos/internal/raft"
)

// TestSimulator_ConvergesUnderRandomDropAndDuplicate is a smoke test that
// the whole loop still reaches a correct end state — leader elected,
// command committed and applied by a majority — with a nontrivial rate of
// dropped and duplicated messages in play, not just the fault-free path.
func TestSimulator_ConvergesUnderRandomDropAndDuplicate(t *testing.T) {
	s := NewSimulator(newThreeNodeCluster(), rand.New(rand.NewSource(11)))
	s.SetFaultConfig(FaultConfig{DropProbability: 0.1, DuplicateProbability: 0.1})
	seedElectionTimers(s)
	s.Run(2000)

	leader := findLeader(s)
	if leader == nil {
		t.Fatalf("expected a leader to be elected within 2000 ticks despite drop/duplicate")
	}

	const cmd = "set w=4"
	s.schedule(raft.Event{At: s.now, NodeID: leader.ID, Kind: raft.EventClientRequest, Command: []byte(cmd)})
	s.Run(s.now + 2000)

	appliedCount := 0
	for _, id := range s.nodeIDs {
		for _, a := range s.applied[id] {
			if string(a.Command) == cmd {
				appliedCount++
				break
			}
		}
	}
	if appliedCount < 2 {
		t.Fatalf("expected a majority (>=2/3) of nodes to have applied %q despite faults, got %d", cmd, appliedCount)
	}
}
