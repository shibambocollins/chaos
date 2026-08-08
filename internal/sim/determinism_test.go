package sim

import (
	"math/rand"
	"testing"

	"chaos/internal/raft"
)

// runScenario drives a fixed sequence of scenario events (election, a
// client write, a kill, a partition/heal, faults) through a fresh
// Simulator built from seed, and returns a snapshot cheap enough to
// compare between runs.
func runScenario(seed int64) (leaderID int, finalTerm uint64, logLen int, appliedPerNode map[int]int) {
	s := NewSimulator(newCluster([]int{1, 2, 3, 4, 5}), rand.New(rand.NewSource(seed)))
	s.SetFaultConfig(FaultConfig{DropProbability: 0.05, DuplicateProbability: 0.05})
	seedElectionTimers(s)
	s.Run(500)

	leader := findLeader(s)
	if leader == nil {
		return -1, 0, 0, nil
	}

	s.schedule(raft.Event{At: s.now, NodeID: leader.ID, Kind: raft.EventClientRequest, Command: []byte("det-cmd")})
	s.Run(s.now + 300)

	victim := leader.ID % 5
	if victim == 0 {
		victim = 5
	}
	if victim != leader.ID {
		s.Kill(victim)
		s.Run(s.now + 300)
		s.Restart(victim)
		s.Run(s.now + 300)
	}

	applied := make(map[int]int, len(s.nodeIDs))
	for _, id := range s.nodeIDs {
		applied[id] = len(s.applied[id])
	}

	return leader.ID, leader.CurrentTerm, len(leader.Log), applied
}

func TestSimulator_SameSeedProducesIdenticalRun(t *testing.T) {
	const seed = 99

	leaderA, termA, logLenA, appliedA := runScenario(seed)
	leaderB, termB, logLenB, appliedB := runScenario(seed)

	if leaderA != leaderB {
		t.Fatalf("expected identical leader across runs, got %d vs %d", leaderA, leaderB)
	}
	if termA != termB {
		t.Fatalf("expected identical final term across runs, got %d vs %d", termA, termB)
	}
	if logLenA != logLenB {
		t.Fatalf("expected identical log length across runs, got %d vs %d", logLenA, logLenB)
	}
	if len(appliedA) != len(appliedB) {
		t.Fatalf("expected identical node count in applied snapshot, got %d vs %d", len(appliedA), len(appliedB))
	}
	for id, countA := range appliedA {
		if countB := appliedB[id]; countA != countB {
			t.Fatalf("node %d: expected identical applied count across runs, got %d vs %d", id, countA, countB)
		}
	}
}
