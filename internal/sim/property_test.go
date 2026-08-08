package sim

import (
	"fmt"
	"math/rand"
	"testing"

	"chaos/internal/raft"
)

func aliveNodeIDs(s *Simulator) []int {
	var out []int
	for _, id := range s.nodeIDs {
		if s.alive[id] {
			out = append(out, id)
		}
	}
	return out
}

func deadNodeIDs(s *Simulator) []int {
	var out []int
	for _, id := range s.nodeIDs {
		if !s.alive[id] {
			out = append(out, id)
		}
	}
	return out
}

// runRandomFaultScenario builds a 5-node cluster and, for a given seed,
// drives 200 rounds of a random mix of kill/restart/partition/heal/client
// requests under a nonzero drop/duplicate rate, checking all five safety
// properties throughout via the production SafetyMonitor. A failure names
// the exact seed to replay.
func runRandomFaultScenario(t *testing.T, seed int64) {
	ids := []int{1, 2, 3, 4, 5}
	s := NewSimulator(newCluster(ids), rand.New(rand.NewSource(seed)))
	s.SetFaultConfig(FaultConfig{DropProbability: 0.05, DuplicateProbability: 0.05})
	seedElectionTimers(s)

	monitor := NewSafetyMonitor(s)
	actions := rand.New(rand.NewSource(seed ^ 0x5bd1e995))
	cmdCounter := 0
	eventsProcessed := 0

	for round := 0; round < 200; round++ {
		switch actions.Intn(5) {
		case 0: // kill a random alive node, only if a majority would still survive
			alive := aliveNodeIDs(s)
			if len(alive) > len(ids)/2+1 {
				s.Kill(alive[actions.Intn(len(alive))])
			}
		case 1: // restart a random dead node
			if dead := deadNodeIDs(s); len(dead) > 0 {
				s.Restart(dead[actions.Intn(len(dead))])
			}
		case 2: // partition into two random groups
			perm := actions.Perm(len(ids))
			split := 1 + actions.Intn(len(ids)-1)
			var g1, g2 []int
			for i, idx := range perm {
				if i < split {
					g1 = append(g1, ids[idx])
				} else {
					g2 = append(g2, ids[idx])
				}
			}
			s.Partition([][]int{g1, g2})
		case 3: // heal
			s.Heal()
		case 4: // client request to the current leader, if any
			if leader := s.Leader(); leader != nil {
				cmdCounter++
				s.schedule(raft.Event{At: s.now, NodeID: leader.ID, Kind: raft.EventClientRequest, Command: []byte(fmt.Sprintf("cmd-%d", cmdCounter))})
			}
		}

		eventsProcessed += s.RunObserving(monitor, s.now+raft.Time(1+actions.Intn(10)))
	}

	// Let everything settle fully connected before the final checks.
	s.Heal()
	eventsProcessed += s.RunObserving(monitor, s.now+2000)

	monitor.CheckFinal()

	if violations := monitor.Violations(); len(violations) > 0 {
		report := BuildReport(seed, s, monitor, eventsProcessed)
		t.Fatalf("safety violations for seed %d:\n%s", seed, report)
	}
}

func TestSimulator_RandomFaultInjectionPreservesSafetyProperties(t *testing.T) {
	for seed := int64(1); seed <= 50; seed++ {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			runRandomFaultScenario(t, seed)
		})
	}
}
