package sim

import (
	"math/rand"
	"strings"
	"testing"

	"chaos/internal/raft"
)

func TestSafetyMonitor_NoViolationsOnCleanRun(t *testing.T) {
	s := NewSimulator(newThreeNodeCluster(), rand.New(rand.NewSource(42)))
	seedElectionTimers(s)
	monitor := NewSafetyMonitor(s)

	steps := s.RunObserving(monitor, 500)

	leader := s.Leader()
	if leader == nil {
		t.Fatalf("expected a leader to be elected within 500 ticks")
	}

	s.schedule(raft.Event{At: s.now, NodeID: leader.ID, Kind: raft.EventClientRequest, Command: []byte("cmd")})
	steps += s.RunObserving(monitor, s.now+200)

	monitor.CheckFinal()

	if violations := monitor.Violations(); len(violations) != 0 {
		t.Fatalf("expected no violations on a clean run, got %+v", violations)
	}

	report := BuildReport(1, s, monitor, steps)
	if !report.Passed() {
		t.Fatalf("expected report.Passed() true, got false: %s", report)
	}
	if report.EventsProcessed != steps {
		t.Fatalf("expected EventsProcessed %d, got %d", steps, report.EventsProcessed)
	}
	if len(report.Nodes) != 3 {
		t.Fatalf("expected 3 node summaries, got %d", len(report.Nodes))
	}
	if !strings.Contains(report.String(), "PASS") {
		t.Fatalf("expected report string to contain PASS, got %q", report.String())
	}
}

func TestSafetyMonitor_ElectionSafetyCatchesTwoLeadersSameTerm(t *testing.T) {
	s := NewSimulator(newThreeNodeCluster(), rand.New(rand.NewSource(1)))
	monitor := NewSafetyMonitor(s)

	// Force an impossible state directly: two nodes claiming Leader in the
	// same term. This is exactly what Observe must catch.
	s.nodes[1].Role = raft.Leader
	s.nodes[1].CurrentTerm = 5
	s.nodes[2].Role = raft.Leader
	s.nodes[2].CurrentTerm = 5

	monitor.Observe()

	violations := monitor.Violations()
	if len(violations) != 1 {
		t.Fatalf("expected exactly 1 violation, got %d: %+v", len(violations), violations)
	}
	if violations[0].Property != "Election Safety" {
		t.Fatalf("expected an Election Safety violation, got %q", violations[0].Property)
	}
}

func TestReport_StringIncludesViolations(t *testing.T) {
	r := Report{
		Seed:            7,
		EventsProcessed: 10,
		FinalTick:       100,
		Nodes:           []NodeSummary{{ID: 1, Alive: true, Role: raft.Leader, CurrentTerm: 2, LogLen: 1, CommitIndex: 1, AppliedCount: 1}},
		Violations:      []Violation{{Property: "Election Safety", Detail: "term 2 has leaders 1 and 2"}},
	}

	if r.Passed() {
		t.Fatalf("expected Passed() false when Violations is non-empty")
	}

	out := r.String()
	if !strings.Contains(out, "FAIL") {
		t.Fatalf("expected report string to contain FAIL, got %q", out)
	}
	if !strings.Contains(out, "Election Safety") {
		t.Fatalf("expected report string to include the violation's property, got %q", out)
	}
}
