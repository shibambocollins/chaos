package sim

import (
	"math/rand"
	"testing"

	"chaos/internal/raft"
)

func newThreeNodeCluster() map[int]*raft.NodeState {
	ids := []int{1, 2, 3}
	nodes := make(map[int]*raft.NodeState, len(ids))
	for _, id := range ids {
		var peers []int
		for _, other := range ids {
			if other != id {
				peers = append(peers, other)
			}
		}
		nodes[id] = raft.NewNodeState(id, peers)
	}
	return nodes
}

func findLeader(s *Simulator) *raft.NodeState {
	for _, id := range s.nodeIDs {
		if s.nodes[id].Role == raft.Leader {
			return s.nodes[id]
		}
	}
	return nil
}

// TestSimulator_ElectsLeaderReplicatesAndApplies is the checkpoint the
// context doc's own roadmap calls for before layering on fault injection:
// a full run through the real event loop — election, a client write,
// replication, commit-index advancement, and apply — with no fault
// injection at all yet.
func TestSimulator_ElectsLeaderReplicatesAndApplies(t *testing.T) {
	s := NewSimulator(newThreeNodeCluster(), rand.New(rand.NewSource(42)))

	// Kick off every node's first election timer at generation 0, matching
	// a freshly constructed NodeState's starting timerGen.
	for _, id := range s.nodeIDs {
		s.schedule(raft.Event{At: 0, NodeID: id, Kind: raft.EventTimerFire, TimerKindField: raft.TimerElection, TimerGen: 0})
	}

	s.Run(500)

	leader := findLeader(s)
	if leader == nil {
		t.Fatalf("expected a leader to be elected within 500 ticks")
	}

	const cmd = "set x=1"
	s.schedule(raft.Event{At: s.now, NodeID: leader.ID, Kind: raft.EventClientRequest, Command: []byte(cmd)})
	s.Run(s.now + 200)

	if len(leader.Log) != 1 {
		t.Fatalf("expected leader log to have 1 entry, got %d", len(leader.Log))
	}

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
		t.Fatalf("expected a majority (>=2/3) of nodes to have applied %q, got %d", cmd, appliedCount)
	}
}
