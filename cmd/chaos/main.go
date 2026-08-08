// Command chaos runs randomized fault-injection scenarios against the
// deterministic Raft simulator and reports whether all five safety
// properties (context doc §3) held throughout each run.
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"

	"chaos/internal/raft"
	"chaos/internal/sim"
)

func main() {
	nodes := flag.Int("nodes", 5, "cluster size")
	seeds := flag.Int("seeds", 50, "number of seeds to run")
	startSeed := flag.Int64("start-seed", 1, "first seed value; seeds run consecutively from here")
	rounds := flag.Int("rounds", 200, "fault-injection rounds per seed")
	drop := flag.Float64("drop", 0.05, "per-message drop probability")
	duplicate := flag.Float64("duplicate", 0.05, "per-message duplicate probability")
	verbose := flag.Bool("verbose", false, "print the full report for every seed, not just failures")
	flag.Parse()

	failed := 0
	for i := 0; i < *seeds; i++ {
		seed := *startSeed + int64(i)
		report := runScenario(seed, *nodes, *rounds, *drop, *duplicate)

		if report.Passed() {
			if *verbose {
				fmt.Print(report)
			} else {
				fmt.Printf("seed=%d PASS\n", seed)
			}
			continue
		}

		failed++
		fmt.Print(report)
	}

	fmt.Printf("\n%d/%d seeds passed\n", *seeds-failed, *seeds)
	if failed > 0 {
		os.Exit(1)
	}
}

// runScenario builds a fresh cluster for seed and drives it through the
// same randomized kill/restart/partition/heal/client-request mix the
// property-based test suite uses, checking all five safety properties via
// a SafetyMonitor the whole way through.
func runScenario(seed int64, nodeCount, rounds int, dropProb, dupProb float64) sim.Report {
	ids := make([]int, nodeCount)
	for i := range ids {
		ids[i] = i + 1
	}

	s := sim.NewSimulator(buildCluster(ids), rand.New(rand.NewSource(seed)))
	s.SetFaultConfig(sim.FaultConfig{DropProbability: dropProb, DuplicateProbability: dupProb})
	for _, id := range ids {
		s.Schedule(raft.Event{At: 0, NodeID: id, Kind: raft.EventTimerFire, TimerKindField: raft.TimerElection, TimerGen: 0})
	}

	monitor := sim.NewSafetyMonitor(s)
	actions := rand.New(rand.NewSource(seed ^ 0x5bd1e995))
	cmdCounter := 0
	eventsProcessed := 0

	for round := 0; round < rounds; round++ {
		switch actions.Intn(5) {
		case 0: // kill a random alive node, only if a majority would still survive
			if alive := aliveIDs(s, ids); len(alive) > len(ids)/2+1 {
				s.Kill(alive[actions.Intn(len(alive))])
			}
		case 1: // restart a random dead node
			if dead := deadIDs(s, ids); len(dead) > 0 {
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
				s.Schedule(raft.Event{At: s.Now(), NodeID: leader.ID, Kind: raft.EventClientRequest, Command: []byte(fmt.Sprintf("cmd-%d", cmdCounter))})
			}
		}

		eventsProcessed += s.RunObserving(monitor, s.Now()+raft.Time(1+actions.Intn(10)))
	}

	s.Heal()
	eventsProcessed += s.RunObserving(monitor, s.Now()+2000)

	monitor.CheckFinal()
	return sim.BuildReport(seed, s, monitor, eventsProcessed)
}

func buildCluster(ids []int) map[int]*raft.NodeState {
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

func aliveIDs(s *sim.Simulator, ids []int) []int {
	var out []int
	for _, id := range ids {
		if s.IsAlive(id) {
			out = append(out, id)
		}
	}
	return out
}

func deadIDs(s *sim.Simulator, ids []int) []int {
	var out []int
	for _, id := range ids {
		if !s.IsAlive(id) {
			out = append(out, id)
		}
	}
	return out
}
