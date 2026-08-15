package sim

import (
	"math/rand"
	"testing"

	"chaos/internal/raft"
)

func TestTraceRecorder_CapturesTicksAndNarratesTransitions(t *testing.T) {
	s := NewSimulator(newThreeNodeCluster(), rand.New(rand.NewSource(42)))
	seedElectionTimers(s)

	rec := NewTraceRecorder(s, "test-scenario")
	rec.Observe() // baseline before anything runs

	for i := 0; i < 4; i++ {
		s.Step()
		rec.Observe()
	}

	trace := rec.Build()

	if trace.Meta.Scenario != "test-scenario" || trace.Meta.NodeCount != 3 {
		t.Fatalf("expected meta {test-scenario, 3}, got %+v", trace.Meta)
	}
	// Only the baseline plus ticks that actually changed something get
	// kept — not one entry per Step, which would be mostly redundant
	// no-op frames (see Observe's doc comment).
	if len(trace.Ticks) == 0 {
		t.Fatalf("expected at least the baseline tick, got 0")
	}
	if len(trace.Ticks[0].Nodes) != 3 {
		t.Fatalf("expected 3 nodes per tick, got %d", len(trace.Ticks[0].Nodes))
	}
	for _, n := range trace.Ticks[0].Nodes {
		if n.Role != "Follower" || !n.Alive {
			t.Fatalf("expected baseline tick to show every node as an alive Follower, got %+v", n)
		}
	}
	for i, tick := range trace.Ticks {
		if i == 0 {
			continue // baseline is kept unconditionally, even with no narration
		}
		if len(tick.Narration) == 0 {
			t.Fatalf("expected every non-baseline tick to carry narration (no-op ticks should be dropped), tick %d: %+v", i, tick)
		}
	}

	foundCandidateNarration := false
	for _, tick := range trace.Ticks {
		for _, line := range tick.Narration {
			if line == "node 1 became Candidate (term 1)" {
				foundCandidateNarration = true
			}
		}
	}
	if !foundCandidateNarration {
		t.Fatalf("expected narration for node 1 becoming Candidate somewhere in the trace, got ticks: %+v", trace.Ticks)
	}
}

func TestTraceRecorder_NarratesKillAndRestart(t *testing.T) {
	nodes := map[int]*raft.NodeState{
		1: raft.NewNodeState(1, []int{2, 3}),
		2: raft.NewNodeState(2, []int{1, 3}),
		3: raft.NewNodeState(3, []int{1, 2}),
	}
	s := NewSimulator(nodes, rand.New(rand.NewSource(1)))
	rec := NewTraceRecorder(s, "kill-restart")
	rec.Observe()

	s.Kill(2)
	rec.Observe()
	s.Restart(2)
	rec.Observe()

	trace := rec.Build()

	var all []string
	for _, tick := range trace.Ticks {
		all = append(all, tick.Narration...)
	}

	wantKilled, wantRestarted := false, false
	for _, line := range all {
		if line == "node 2 killed" {
			wantKilled = true
		}
		if line == "node 2 restarted" {
			wantRestarted = true
		}
	}
	if !wantKilled {
		t.Fatalf("expected 'node 2 killed' narration, got %v", all)
	}
	if !wantRestarted {
		t.Fatalf("expected 'node 2 restarted' narration, got %v", all)
	}
}

func TestTraceRecorder_NarratesPartitionAndHeal(t *testing.T) {
	nodes := map[int]*raft.NodeState{
		1: raft.NewNodeState(1, []int{2, 3}),
		2: raft.NewNodeState(2, []int{1, 3}),
		3: raft.NewNodeState(3, []int{1, 2}),
	}
	s := NewSimulator(nodes, rand.New(rand.NewSource(1)))
	rec := NewTraceRecorder(s, "partition-heal")
	rec.Observe()

	s.Partition([][]int{{1}, {2, 3}})
	rec.Observe()
	s.Heal()
	rec.Observe()

	trace := rec.Build()

	// The very first tick after Partition should show every node's Group
	// field distinguishing the two sides — this is the whole point of the
	// field, so check it directly rather than only its narration.
	afterPartition := trace.Ticks[1]
	groups := make(map[int]int, len(afterPartition.Nodes))
	for _, n := range afterPartition.Nodes {
		groups[n.ID] = n.Group
	}
	if groups[1] == groups[2] || groups[2] != groups[3] {
		t.Fatalf("expected node 1 in its own group, nodes 2 and 3 sharing one, got %+v", groups)
	}

	var all []string
	for _, tick := range trace.Ticks {
		all = append(all, tick.Narration...)
	}

	wantLostContact, wantReconnected := false, false
	for _, line := range all {
		if line == "node 1 lost contact with part of the cluster" {
			wantLostContact = true
		}
		if line == "node 1 reconnected to the rest of the cluster" {
			wantReconnected = true
		}
	}
	if !wantLostContact {
		t.Fatalf("expected 'node 1 lost contact with part of the cluster' narration, got %v", all)
	}
	if !wantReconnected {
		t.Fatalf("expected 'node 1 reconnected to the rest of the cluster' narration, got %v", all)
	}

	// After Heal, every node's Group must read -1 — the sentinel "no
	// active partition" value, not merely "same as everyone else's."
	afterHeal := trace.Ticks[len(trace.Ticks)-1]
	for _, n := range afterHeal.Nodes {
		if n.Group != -1 {
			t.Fatalf("expected Group -1 for every node after Heal, node %d has %d", n.ID, n.Group)
		}
	}
}
