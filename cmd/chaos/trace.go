package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"

	"chaos/internal/raft"
	"chaos/internal/sim"
)

// traceScenarios maps a scenario name (the -trace flag's value) to the
// function that builds it. Each one tells one clear story end to end —
// unlike the randomized fault-injection report, which deliberately isn't
// traced: 200 rounds of random actions doesn't narrate cleanly.
var traceScenarios = map[string]func(seed int64) sim.Trace{
	"election":       electionScenario,
	"partition-heal": partitionHealScenario,
	"kill-restart":   killRestartScenario,
}

// scenarioNames lists traceScenarios' keys, sorted, for the -trace flag's
// usage text.
func scenarioNames() string {
	names := make([]string, 0, len(traceScenarios))
	for name := range traceScenarios {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// runTraceExport builds the named curated scenario and writes it as JSON
// to outPath.
func runTraceExport(name, outPath string, seed int64) {
	build, ok := traceScenarios[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown -trace scenario %q (want one of: %s)\n", name, scenarioNames())
		os.Exit(1)
	}
	if outPath == "" {
		fmt.Fprintln(os.Stderr, "-trace-out is required with -trace")
		os.Exit(1)
	}

	trace := build(seed)

	data, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal trace: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", outPath, err)
		os.Exit(1)
	}

	fmt.Printf("wrote %s (%d ticks) to %s\n", name, len(trace.Ticks), outPath)
}

func newTracedCluster(seed int64, nodeCount int) (*sim.Simulator, []int) {
	ids := make([]int, nodeCount)
	for i := range ids {
		ids[i] = i + 1
	}
	s := sim.NewSimulator(buildCluster(ids), rand.New(rand.NewSource(seed)))
	for _, id := range ids {
		s.Schedule(raft.Event{At: 0, NodeID: id, Kind: raft.EventTimerFire, TimerKindField: raft.TimerElection, TimerGen: 0})
	}
	return s, ids
}

// electionScenario: a clean election from a cold start, then one client
// write replicated and committed.
func electionScenario(seed int64) sim.Trace {
	s, _ := newTracedCluster(seed, 5)
	rec := sim.NewTraceRecorder(s, "election")
	rec.Observe()

	s.RunEach(500, rec.Observe)

	if leader := s.Leader(); leader != nil {
		s.Schedule(raft.Event{At: s.Now(), NodeID: leader.ID, Kind: raft.EventClientRequest, Command: []byte("set x=1")})
		s.RunEach(s.Now()+200, rec.Observe)
	}

	return rec.Build()
}

// partitionHealScenario is the context doc's §3 worked example: a leader
// commits normally, gets split from a minority by a partition, the
// majority keeps making progress while the minority correctly stalls, then
// Heal lets the minority catch back up.
func partitionHealScenario(seed int64) sim.Trace {
	s, ids := newTracedCluster(seed, 5)
	rec := sim.NewTraceRecorder(s, "partition-heal")
	rec.Observe()

	s.RunEach(500, rec.Observe)

	leader := s.Leader()
	if leader == nil {
		return rec.Build()
	}
	s.Schedule(raft.Event{At: s.Now(), NodeID: leader.ID, Kind: raft.EventClientRequest, Command: []byte("set x=1")})
	s.RunEach(s.Now()+200, rec.Observe)

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

	s.Schedule(raft.Event{At: s.Now(), NodeID: leader.ID, Kind: raft.EventClientRequest, Command: []byte("set y=2")})
	s.RunEach(s.Now()+400, rec.Observe)

	s.Heal()
	s.RunEach(s.Now()+400, rec.Observe)

	return rec.Build()
}

// killRestartScenario: the leader is killed, a replacement is elected from
// the surviving majority and keeps making progress, then the original
// leader restarts as a Follower and catches up.
func killRestartScenario(seed int64) sim.Trace {
	s, _ := newTracedCluster(seed, 5)
	rec := sim.NewTraceRecorder(s, "kill-restart")
	rec.Observe()

	s.RunEach(500, rec.Observe)

	firstLeader := s.Leader()
	if firstLeader == nil {
		return rec.Build()
	}
	firstLeaderID := firstLeader.ID
	s.Kill(firstLeaderID)
	rec.Observe()

	s.RunEach(s.Now()+500, rec.Observe)

	if leader := s.Leader(); leader != nil {
		s.Schedule(raft.Event{At: s.Now(), NodeID: leader.ID, Kind: raft.EventClientRequest, Command: []byte("set z=3")})
		s.RunEach(s.Now()+300, rec.Observe)
	}

	s.Restart(firstLeaderID)
	rec.Observe()
	s.RunEach(s.Now()+300, rec.Observe)

	return rec.Build()
}
