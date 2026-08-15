package sim

import (
	"fmt"
	"strings"

	"chaos/internal/raft"
)

// Violation records one instance where a run failed to preserve one of the
// five safety properties from the context doc's §3.
type Violation struct {
	Property string
	Detail   string
}

// SafetyMonitor watches a Simulator's history and checks all five safety
// properties. Two of them — Election Safety and Leader Append-Only — are
// properties of the whole timeline, not just wherever a run happens to end
// up, so Observe must be called after every single Simulator.Step; a
// violation that self-corrects before the run ends would be invisible to a
// final-snapshot-only check. The other three — Log Matching, Leader
// Completeness, State Machine Safety — are checked from accumulated
// history by CheckFinal, meant to be called once after the run is done.
//
// A violation is recorded, not fatal: SafetyMonitor only observes, it never
// stops the Simulator itself. The caller decides what a recorded violation
// means — a test fails on it, a report just lists it.
type SafetyMonitor struct {
	s *Simulator

	violations []Violation

	leaderForTerm map[uint64]int
	lastLeaderLog map[int][]raft.LogEntry
	committed     map[uint64][]byte
}

// NewSafetyMonitor creates a monitor watching s.
func NewSafetyMonitor(s *Simulator) *SafetyMonitor {
	return &SafetyMonitor{
		s:             s,
		leaderForTerm: make(map[uint64]int),
		lastLeaderLog: make(map[int][]raft.LogEntry),
		committed:     make(map[uint64][]byte),
	}
}

func (m *SafetyMonitor) violate(property, detail string) {
	m.violations = append(m.violations, Violation{Property: property, Detail: detail})
}

// Observe checks Election Safety and Leader Append-Only against the
// Simulator's current state, and records every OutApply seen so far for
// the later State Machine Safety / Leader Completeness checks.
func (m *SafetyMonitor) Observe() {
	for _, id := range m.s.nodeIDs {
		n := m.s.nodes[id]

		if n.Role != raft.Leader {
			delete(m.lastLeaderLog, id) // tenure ended; next leadership starts a fresh baseline
			continue
		}

		if existing, ok := m.leaderForTerm[n.CurrentTerm]; ok {
			if existing != id {
				m.violate("Election Safety", fmt.Sprintf("term %d has leaders %d and %d", n.CurrentTerm, existing, id))
			}
		} else {
			m.leaderForTerm[n.CurrentTerm] = id
		}

		if prev, ok := m.lastLeaderLog[id]; ok {
			if len(n.Log) < len(prev) {
				m.violate("Leader Append-Only", fmt.Sprintf("node %d log shrank from %d to %d entries while leader", id, len(prev), len(n.Log)))
			} else {
				for i, e := range prev {
					if n.Log[i].Term != e.Term || string(n.Log[i].Command) != string(e.Command) {
						m.violate("Leader Append-Only", fmt.Sprintf("node %d entry %d changed while leader", id, i+1))
						break
					}
				}
			}
		}
		m.lastLeaderLog[id] = append([]raft.LogEntry(nil), n.Log...)
	}

	for _, id := range m.s.nodeIDs {
		for _, a := range m.s.applied[id] {
			if want, ok := m.committed[a.Index]; ok {
				if string(want) != string(a.Command) {
					m.violate("State Machine Safety", fmt.Sprintf("index %d applied as both %q and %q", a.Index, want, a.Command))
				}
			} else {
				m.committed[a.Index] = a.Command
			}
		}
	}
}

// CheckFinal runs the two checks that only make sense against a completed
// run's accumulated state: Log Matching and Leader Completeness.
func (m *SafetyMonitor) CheckFinal() {
	m.checkLogMatching()
	m.checkLeaderCompleteness()
}

// checkLogMatching verifies, across every pair of nodes, that wherever two
// logs share an (index, term), the command at that index is identical.
func (m *SafetyMonitor) checkLogMatching() {
	for i, a := range m.s.nodeIDs {
		for _, b := range m.s.nodeIDs[i+1:] {
			logA, logB := m.s.nodes[a].Log, m.s.nodes[b].Log
			n := len(logA)
			if len(logB) < n {
				n = len(logB)
			}
			for idx := 0; idx < n; idx++ {
				if logA[idx].Term == logB[idx].Term && string(logA[idx].Command) != string(logB[idx].Command) {
					m.violate("Log Matching", fmt.Sprintf("nodes %d/%d differ in command at index %d despite matching term %d", a, b, idx+1, logA[idx].Term))
				}
			}
		}
	}
}

// checkLeaderCompleteness verifies the final leader's log (if any) still
// contains every entry that was ever applied anywhere — applying only
// happens post-commit, so "ever applied" is a safe stand-in for "committed."
func (m *SafetyMonitor) checkLeaderCompleteness() {
	leader := m.s.Leader()
	if leader == nil {
		return
	}
	have := make(map[uint64][]byte, len(leader.Log))
	for _, e := range leader.Log {
		have[e.Index] = e.Command
	}
	for index, cmd := range m.committed {
		got, ok := have[index]
		if !ok || string(got) != string(cmd) {
			m.violate("Leader Completeness", fmt.Sprintf("committed index %d (%q) missing/differs in final leader %d's log", index, cmd, leader.ID))
		}
	}
}

// Violations returns every violation recorded so far.
func (m *SafetyMonitor) Violations() []Violation {
	return m.violations
}

// NodeSummary is one node's final state, as captured by BuildReport.
type NodeSummary struct {
	ID           int
	Alive        bool
	Role         raft.Role
	CurrentTerm  uint64
	LogLen       int
	CommitIndex  uint64
	AppliedCount int
	Group        int // see Simulator.Group: -1 means no active partition
}

// Report summarizes one completed simulation run: final per-node state,
// plus whatever the SafetyMonitor recorded along the way.
type Report struct {
	Seed            int64
	EventsProcessed int
	FinalTick       raft.Time
	Nodes           []NodeSummary
	Violations      []Violation
}

// Passed reports whether the run ended with no recorded safety violations.
func (r Report) Passed() bool {
	return len(r.Violations) == 0
}

// BuildReport snapshots s's final state alongside whatever monitor recorded
// over the run. eventsProcessed is the caller's own count of Step calls —
// the Simulator doesn't track this itself, since nothing else needs it.
func BuildReport(seed int64, s *Simulator, monitor *SafetyMonitor, eventsProcessed int) Report {
	nodes := make([]NodeSummary, 0, len(s.nodeIDs))
	for _, id := range s.nodeIDs {
		n := s.nodes[id]
		nodes = append(nodes, NodeSummary{
			ID:           id,
			Alive:        s.alive[id],
			Role:         n.Role,
			CurrentTerm:  n.CurrentTerm,
			LogLen:       len(n.Log),
			CommitIndex:  n.CommitIndex,
			AppliedCount: len(s.applied[id]),
			Group:        s.Group(id),
		})
	}

	return Report{
		Seed:            seed,
		EventsProcessed: eventsProcessed,
		FinalTick:       s.now,
		Nodes:           nodes,
		Violations:      monitor.Violations(),
	}
}

// String renders a human-readable summary: pass/fail, per-node final
// state, and every recorded violation.
func (r Report) String() string {
	var b strings.Builder

	status := "PASS"
	if !r.Passed() {
		status = "FAIL"
	}
	fmt.Fprintf(&b, "seed=%d %s (tick=%d, events=%d)\n", r.Seed, status, r.FinalTick, r.EventsProcessed)

	for _, n := range r.Nodes {
		alive := "alive"
		if !n.Alive {
			alive = "dead"
		}
		fmt.Fprintf(&b, "  node %d: %-6s role=%-9v term=%-4d logLen=%-4d commitIndex=%-4d applied=%d\n",
			n.ID, alive, n.Role, n.CurrentTerm, n.LogLen, n.CommitIndex, n.AppliedCount)
	}

	for _, v := range r.Violations {
		fmt.Fprintf(&b, "  VIOLATION [%s]: %s\n", v.Property, v.Detail)
	}

	return b.String()
}
