package sim

import (
	"math/rand"
	"testing"

	"chaos/internal/raft"
)

// TestSimulator_PartitionMajorityCommitsMinorityStallsThenHeals is the
// context doc's §3 worked example, encoded as a replayable test: split a
// 5-node cluster into a 3-node majority and a 2-node minority. The
// majority must keep electing/committing normally; the minority must
// correctly stall — a candidate there can never reach a majority of 5.
// On Heal, the minority must catch up to the majority's committed log.
func TestSimulator_PartitionMajorityCommitsMinorityStallsThenHeals(t *testing.T) {
	ids := []int{1, 2, 3, 4, 5}
	s := NewSimulator(newCluster(ids), rand.New(rand.NewSource(3)))
	seedElectionTimers(s)
	s.Run(500)

	leader := findLeader(s)
	if leader == nil {
		t.Fatalf("expected an initial leader to be elected")
	}

	minority := make([]int, 0, 2)
	majority := make([]int, 0, 3)
	majority = append(majority, leader.ID)
	for _, id := range ids {
		if id == leader.ID {
			continue
		}
		if len(minority) < 2 {
			minority = append(minority, id)
		} else {
			majority = append(majority, id)
		}
	}

	s.Partition([][]int{majority, minority})

	const cmd = "set z=3"
	s.schedule(raft.Event{At: s.now, NodeID: leader.ID, Kind: raft.EventClientRequest, Command: []byte(cmd)})
	s.Run(s.now + 500)

	for _, id := range majority {
		found := false
		for _, a := range s.applied[id] {
			if string(a.Command) == cmd {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected majority node %d to have applied %q", id, cmd)
		}
	}

	for _, id := range minority {
		if s.nodes[id].Role == raft.Leader {
			t.Fatalf("expected minority node %d to never become leader while partitioned", id)
		}
		if len(s.applied[id]) != 0 {
			t.Fatalf("expected minority node %d to have applied nothing while partitioned, got %v", id, s.applied[id])
		}
	}

	s.Heal()
	s.Run(s.now + 500)

	for _, id := range minority {
		found := false
		for _, a := range s.applied[id] {
			if string(a.Command) == cmd {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected minority node %d to catch up on %q after Heal", id, cmd)
		}
	}
}
