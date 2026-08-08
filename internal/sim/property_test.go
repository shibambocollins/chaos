package sim

import (
	"fmt"
	"math/rand"
	"testing"

	"chaos/internal/raft"
)

// propertyChecker accumulates history across a scenario's Step calls and
// fails the test the instant any of the context doc's five safety
// properties is violated, rather than only inspecting a final snapshot —
// several of these (Election Safety, Leader Append-Only) are properties of
// the whole timeline, not just where a run happens to end up.
type propertyChecker struct {
	t *testing.T
	s *Simulator

	leaderForTerm map[uint64]int          // Election Safety
	lastLeaderLog map[int][]raft.LogEntry // Leader Append-Only, reset each time a node's leadership tenure ends
	committed     map[uint64][]byte       // index -> command, from every OutApply ever observed (State Machine Safety + Leader Completeness basis)
}

func newPropertyChecker(t *testing.T, s *Simulator) *propertyChecker {
	return &propertyChecker{
		t:             t,
		s:             s,
		leaderForTerm: make(map[uint64]int),
		lastLeaderLog: make(map[int][]raft.LogEntry),
		committed:     make(map[uint64][]byte),
	}
}

// observe must be called after every single Step() — it's the only way to
// catch a transient state a batched Run() would skip straight past.
func (p *propertyChecker) observe() {
	for _, id := range p.s.nodeIDs {
		n := p.s.nodes[id]

		if n.Role != raft.Leader {
			delete(p.lastLeaderLog, id) // tenure ended; next leadership starts a fresh baseline
			continue
		}

		if existing, ok := p.leaderForTerm[n.CurrentTerm]; ok {
			if existing != id {
				p.t.Fatalf("Election Safety violated: term %d has leaders %d and %d", n.CurrentTerm, existing, id)
			}
		} else {
			p.leaderForTerm[n.CurrentTerm] = id
		}

		if prev, ok := p.lastLeaderLog[id]; ok {
			if len(n.Log) < len(prev) {
				p.t.Fatalf("Leader Append-Only violated: node %d log shrank from %d to %d entries while leader", id, len(prev), len(n.Log))
			}
			for i, e := range prev {
				if n.Log[i].Term != e.Term || string(n.Log[i].Command) != string(e.Command) {
					p.t.Fatalf("Leader Append-Only violated: node %d entry %d changed while leader", id, i+1)
				}
			}
		}
		p.lastLeaderLog[id] = append([]raft.LogEntry(nil), n.Log...)
	}

	for _, id := range p.s.nodeIDs {
		for _, a := range p.s.applied[id] {
			if want, ok := p.committed[a.Index]; ok {
				if string(want) != string(a.Command) {
					p.t.Fatalf("State Machine Safety violated: index %d applied as both %q and %q", a.Index, want, a.Command)
				}
			} else {
				p.committed[a.Index] = a.Command
			}
		}
	}
}

// checkLogMatching verifies, across every pair of nodes, that wherever two
// logs share an (index, term), the command at that index is identical.
func (p *propertyChecker) checkLogMatching() {
	for i, a := range p.s.nodeIDs {
		for _, b := range p.s.nodeIDs[i+1:] {
			logA, logB := p.s.nodes[a].Log, p.s.nodes[b].Log
			n := len(logA)
			if len(logB) < n {
				n = len(logB)
			}
			for idx := 0; idx < n; idx++ {
				if logA[idx].Term == logB[idx].Term && string(logA[idx].Command) != string(logB[idx].Command) {
					p.t.Fatalf("Log Matching violated: nodes %d/%d differ in command at index %d despite matching term %d", a, b, idx+1, logA[idx].Term)
				}
			}
		}
	}
}

// checkLeaderCompleteness verifies the final leader's log (if any) still
// contains every entry that was ever applied anywhere — applying only
// happens post-commit, so "ever applied" is a safe stand-in for "committed."
func (p *propertyChecker) checkLeaderCompleteness() {
	leader := findLeader(p.s)
	if leader == nil {
		return
	}
	have := make(map[uint64][]byte, len(leader.Log))
	for _, e := range leader.Log {
		have[e.Index] = e.Command
	}
	for index, cmd := range p.committed {
		got, ok := have[index]
		if !ok || string(got) != string(cmd) {
			p.t.Fatalf("Leader Completeness violated: committed index %d (%q) missing/differs in final leader %d's log", index, cmd, leader.ID)
		}
	}
}

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

// runObserving advances the simulator one event at a time up to and
// including until, calling pc.observe() after each — the property-checking
// equivalent of Run, which does not offer per-event visibility.
func runObserving(s *Simulator, pc *propertyChecker, until raft.Time) {
	for s.queue.Len() > 0 && s.queue[0].At <= until {
		s.Step()
		pc.observe()
	}
}

// runRandomFaultScenario builds a 5-node cluster and, for a given seed,
// drives 200 rounds of a random mix of kill/restart/partition/heal/client
// requests under a nonzero drop/duplicate rate, checking all five safety
// properties throughout. A failure names the exact seed to replay.
func runRandomFaultScenario(t *testing.T, seed int64) {
	ids := []int{1, 2, 3, 4, 5}
	s := NewSimulator(newCluster(ids), rand.New(rand.NewSource(seed)))
	s.SetFaultConfig(FaultConfig{DropProbability: 0.05, DuplicateProbability: 0.05})
	seedElectionTimers(s)

	pc := newPropertyChecker(t, s)
	actions := rand.New(rand.NewSource(seed ^ 0x5bd1e995))
	cmdCounter := 0

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
			if leader := findLeader(s); leader != nil {
				cmdCounter++
				s.schedule(raft.Event{At: s.now, NodeID: leader.ID, Kind: raft.EventClientRequest, Command: []byte(fmt.Sprintf("cmd-%d", cmdCounter))})
			}
		}

		runObserving(s, pc, s.now+raft.Time(1+actions.Intn(10)))
	}

	// Let everything settle fully connected before the final checks.
	s.Heal()
	runObserving(s, pc, s.now+2000)

	pc.checkLogMatching()
	pc.checkLeaderCompleteness()
}

func TestSimulator_RandomFaultInjectionPreservesSafetyProperties(t *testing.T) {
	for seed := int64(1); seed <= 50; seed++ {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			runRandomFaultScenario(t, seed)
		})
	}
}
