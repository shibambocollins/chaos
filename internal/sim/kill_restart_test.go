package sim

import (
	"math/rand"
	"testing"

	"chaos/internal/raft"
)

// TestSimulator_KillLeaderElectsReplacementThenRestartCatchesUp exercises
// the doc's own checklist item: killed/restarted nodes must correctly
// split persistent from volatile state, and the cluster must keep making
// progress on a live majority while the dead node is down.
func TestSimulator_KillLeaderElectsReplacementThenRestartCatchesUp(t *testing.T) {
	s := NewSimulator(newThreeNodeCluster(), rand.New(rand.NewSource(7)))
	seedElectionTimers(s)
	s.Run(500)

	firstLeader := findLeader(s)
	if firstLeader == nil {
		t.Fatalf("expected an initial leader to be elected")
	}
	firstLeaderID := firstLeader.ID

	s.Kill(firstLeaderID)
	s.Run(s.now + 500)

	secondLeader := findLeader(s)
	if secondLeader == nil {
		t.Fatalf("expected a replacement leader to be elected among the surviving majority")
	}
	if secondLeader.ID == firstLeaderID {
		t.Fatalf("expected a different node to become leader (old leader is dead)")
	}

	const cmd = "set y=2"
	s.schedule(raft.Event{At: s.now, NodeID: secondLeader.ID, Kind: raft.EventClientRequest, Command: []byte(cmd)})
	s.Run(s.now + 200)

	aliveApplied := 0
	for _, id := range s.nodeIDs {
		if id == firstLeaderID {
			continue
		}
		for _, a := range s.applied[id] {
			if string(a.Command) == cmd {
				aliveApplied++
				break
			}
		}
	}
	if aliveApplied != 2 {
		t.Fatalf("expected both surviving nodes to have applied %q, got %d", cmd, aliveApplied)
	}

	s.Restart(firstLeaderID)
	s.Run(s.now + 300)

	if firstLeader.Role != raft.Follower {
		t.Fatalf("expected restarted node to be a Follower, got %v", firstLeader.Role)
	}

	caughtUp := false
	for _, a := range s.applied[firstLeaderID] {
		if string(a.Command) == cmd {
			caughtUp = true
			break
		}
	}
	if !caughtUp {
		t.Fatalf("expected restarted node to eventually catch up and apply %q", cmd)
	}
}
