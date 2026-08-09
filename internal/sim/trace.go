package sim

import "fmt"

// TraceNodeState is one node's visible state at one recorded tick. Role is
// a plain string (via raft.Role.String()), not the raft.Role type itself —
// keeps this package's JSON output self-contained without adding
// marshaling methods to internal/raft for a single downstream consumer.
type TraceNodeState struct {
	ID          int    `json:"id"`
	Alive       bool   `json:"alive"`
	Role        string `json:"role"`
	CurrentTerm uint64 `json:"currentTerm"`
	LogLen      int    `json:"logLen"`
	CommitIndex uint64 `json:"commitIndex"`
}

// TraceTick is one recorded moment in a run: every node's state, plus
// human-readable narration lines for whatever changed since the previous
// tick (a role transition, a kill/restart, a commit advancing).
type TraceTick struct {
	At        uint64           `json:"at"`
	Nodes     []TraceNodeState `json:"nodes"`
	Narration []string         `json:"narration,omitempty"`
}

// TraceMeta identifies what scenario a Trace came from.
type TraceMeta struct {
	NodeCount int    `json:"nodeCount"`
	Scenario  string `json:"scenario"`
}

// Trace is a full recorded timeline, meant to be exported as JSON and
// played back by the frontend's timeline scrubber — a replay of a real,
// already-verified run, not a live simulation reimplemented client-side.
type Trace struct {
	Meta  TraceMeta   `json:"meta"`
	Ticks []TraceTick `json:"ticks"`
}

// TraceRecorder watches a Simulator's history and accumulates a Trace.
// Observe must be called once before driving the Simulator at all (to
// capture the starting state) and again after every Step — same
// per-event-visibility requirement as SafetyMonitor, for the same reason:
// a node's state at tick N only exists to be captured right after tick N
// is processed, not retroactively.
type TraceRecorder struct {
	s        *Simulator
	scenario string

	prev  map[int]TraceNodeState
	ticks []TraceTick
}

// NewTraceRecorder creates a recorder watching s, labeling the eventual
// Trace with scenario (just a name for the output file/UI, not consumed by
// any logic).
func NewTraceRecorder(s *Simulator, scenario string) *TraceRecorder {
	return &TraceRecorder{
		s:        s,
		scenario: scenario,
		prev:     make(map[int]TraceNodeState, len(s.nodeIDs)),
	}
}

// Observe captures the current state of every node as a new TraceTick,
// deriving narration lines from whatever changed since the last Observe.
func (r *TraceRecorder) Observe() {
	nodes := make([]TraceNodeState, 0, len(r.s.nodeIDs))
	var narration []string

	for _, id := range r.s.nodeIDs {
		n := r.s.nodes[id]
		cur := TraceNodeState{
			ID:          id,
			Alive:       r.s.alive[id],
			Role:        n.Role.String(),
			CurrentTerm: n.CurrentTerm,
			LogLen:      len(n.Log),
			CommitIndex: n.CommitIndex,
		}

		if prev, ok := r.prev[id]; ok {
			narration = append(narration, diffNarration(prev, cur)...)
		}
		r.prev[id] = cur
		nodes = append(nodes, cur)
	}

	r.ticks = append(r.ticks, TraceTick{At: uint64(r.s.now), Nodes: nodes, Narration: narration})
}

// diffNarration turns the difference between two consecutive observations
// of the same node into human-readable lines, in a fixed priority order
// (alive/dead first — it makes everything else about that node moot).
func diffNarration(prev, cur TraceNodeState) []string {
	var lines []string

	switch {
	case prev.Alive && !cur.Alive:
		lines = append(lines, fmt.Sprintf("node %d killed", cur.ID))
		return lines
	case !prev.Alive && cur.Alive:
		lines = append(lines, fmt.Sprintf("node %d restarted", cur.ID))
	}

	if cur.Alive && prev.Role != cur.Role {
		lines = append(lines, fmt.Sprintf("node %d became %s (term %d)", cur.ID, cur.Role, cur.CurrentTerm))
	}
	if cur.Alive && cur.CommitIndex > prev.CommitIndex {
		lines = append(lines, fmt.Sprintf("node %d committed index %d", cur.ID, cur.CommitIndex))
	}

	return lines
}

// Build returns the accumulated Trace.
func (r *TraceRecorder) Build() Trace {
	return Trace{
		Meta:  TraceMeta{NodeCount: len(r.s.nodeIDs), Scenario: r.scenario},
		Ticks: r.ticks,
	}
}
