package sim

import (
	"container/heap"
	"math/rand"
	"sort"

	"chaos/internal/raft"
)

// Simulator drives a set of raft.NodeState instances through a single
// deterministic event loop. Nothing here touches a real clock, a real
// goroutine, or Go's package-level math/rand — every source of
// nondeterminism is the one *rand.Rand passed in at construction, threaded
// through to every node.Step call.
type Simulator struct {
	nodes   map[int]*raft.NodeState
	nodeIDs []int // sorted — never range nodes directly when iterating

	queue   eventQueue
	nextSeq uint64
	now     raft.Time

	rng *rand.Rand

	// alive tracks whether each node is currently up. A killed node is
	// simply excluded from delivery and scheduling, never sent an event —
	// per context doc §4, a real crashed process doesn't get a heads-up
	// either, so neither does a simulated one.
	alive map[int]bool

	// groupOf assigns each node a partition group when a partition is
	// active; nodes in different groups can't reach each other. nil means
	// no partition — everyone connected. Set by Partition, cleared by Heal.
	groupOf map[int]int

	// faults holds the current random drop/duplicate rates. Zero value
	// (both 0) means no faults — existing tests built before this existed
	// are unaffected.
	faults FaultConfig

	// persisted/applied record what each node's OutPersist/OutApply
	// outbounds reported. Modeled as an ordering guarantee the simulator
	// enforces (OutPersist before any OutSendMessage in the same batch),
	// not real disk I/O — see context doc's persist-before-respond note.
	persisted map[int]persistedState
	applied   map[int][]appliedEntry
}

// FaultConfig holds the random per-message fault rates the simulator rolls
// on every send. Zero value means no faults.
type FaultConfig struct {
	DropProbability      float64
	DuplicateProbability float64
}

type persistedState struct {
	Term     uint64
	VotedFor int
	LogLen   int
}

type appliedEntry struct {
	Index   uint64
	Command []byte
}

// NewSimulator wires up a fresh event loop over nodes, keyed by node ID.
// Every node starts alive; the queue starts empty — the caller is
// responsible for scheduling whatever initial events (typically each
// node's first election timer) the scenario needs.
func NewSimulator(nodes map[int]*raft.NodeState, rng *rand.Rand) *Simulator {
	ids := make([]int, 0, len(nodes))
	alive := make(map[int]bool, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
		alive[id] = true
	}
	sort.Ints(ids)

	return &Simulator{
		nodes:     nodes,
		nodeIDs:   ids,
		rng:       rng,
		alive:     alive,
		persisted: make(map[int]persistedState, len(nodes)),
		applied:   make(map[int][]appliedEntry, len(nodes)),
	}
}

// schedule assigns ev the next Seq (the deterministic tiebreak for events
// sharing an At) and pushes it onto the queue.
func (s *Simulator) schedule(ev raft.Event) {
	ev.Seq = s.nextSeq
	s.nextSeq++
	heap.Push(&s.queue, ev)
}

// Schedule adds ev to the queue, same as schedule but exported for callers
// outside the package — e.g. a driver seeding each node's initial election
// timer, or a CLI submitting a client request — that need to put an event
// on the queue without reaching into the Simulator's internals.
func (s *Simulator) Schedule(ev raft.Event) {
	s.schedule(ev)
}

// Now returns the simulator's current simulated time.
func (s *Simulator) Now() raft.Time {
	return s.now
}

// IsAlive reports whether id is currently up.
func (s *Simulator) IsAlive(id int) bool {
	return s.alive[id]
}

// Leader returns the node currently acting as Leader, or nil if none is —
// e.g. mid-election, or a minority side stalled during a partition.
func (s *Simulator) Leader() *raft.NodeState {
	for _, id := range s.nodeIDs {
		if s.nodes[id].Role == raft.Leader {
			return s.nodes[id]
		}
	}
	return nil
}

// Kill marks id as down. It receives no special "you're dead" event — it
// simply stops being delivered to: step() already discards any event whose
// target isn't alive, so nothing needs scrubbing from the queue here.
func (s *Simulator) Kill(id int) {
	s.alive[id] = false
}

// Restart brings id back up: marks it alive again and runs its
// raft.NodeState.Restart outbound (a fresh election timer) through the same
// applyOutbound path every other Step result goes through.
func (s *Simulator) Restart(id int) {
	s.alive[id] = true
	node := s.nodes[id]
	s.applyOutbound(id, node, node.Restart(s.rng))
}

// Partition splits the cluster into isolated groups: nodes in different
// groups can no longer exchange messages until Heal is called. Every node
// should appear in exactly one group — the doc's worked example
// ({1,2,3} vs {4,5}) is the shape this models.
func (s *Simulator) Partition(groups [][]int) {
	s.groupOf = make(map[int]int, len(s.nodeIDs))
	for g, group := range groups {
		for _, id := range group {
			s.groupOf[id] = g
		}
	}
}

// Heal reconnects the cluster: every node can reach every other node again.
func (s *Simulator) Heal() {
	s.groupOf = nil
}

// connected reports whether a and b can currently exchange messages — true
// unless a partition is active and put them in different groups.
func (s *Simulator) connected(a, b int) bool {
	if s.groupOf == nil {
		return true
	}
	return s.groupOf[a] == s.groupOf[b]
}

// SetFaultConfig sets the random drop/duplicate rates every subsequent send
// rolls against. Existing tests that never call this keep the zero value —
// no faults.
func (s *Simulator) SetFaultConfig(cfg FaultConfig) {
	s.faults = cfg
}

// Run pops and processes every scheduled event with At <= until, in
// (At, Seq) order. Returns once the queue is empty or the next event is
// past the horizon — the caller can keep calling Run with a later horizon
// to keep advancing.
func (s *Simulator) Run(until raft.Time) {
	for s.queue.Len() > 0 && s.queue[0].At <= until {
		s.Step()
	}
}

// RunEach pops and processes every scheduled event with At <= until,
// calling observe after each one — the shared primitive behind
// RunObserving and any other per-event watcher (e.g. a TraceRecorder) that
// needs to see every intermediate state, not just Run's end-of-batch
// result. Returns the number of events processed.
func (s *Simulator) RunEach(until raft.Time, observe func()) int {
	steps := 0
	for s.queue.Len() > 0 && s.queue[0].At <= until {
		s.Step()
		observe()
		steps++
	}
	return steps
}

// RunObserving is Run plus a SafetyMonitor.Observe() call after every
// individual event, not just once at the end of the batch — needed because
// Election Safety and Leader Append-Only are properties of the whole
// timeline, and a violation that briefly appears and self-corrects within
// a batch would be invisible to a monitor only consulted after Run
// returns. Returns the number of events processed.
func (s *Simulator) RunObserving(monitor *SafetyMonitor, until raft.Time) int {
	return s.RunEach(until, monitor.Observe)
}

// Step pops and delivers the single next scheduled event to its target
// node's Step() and applies every Outbound the node returns, advancing s.now
// to that event's At. Reports false (nothing done) if the queue is empty.
//
// Exported so a caller — e.g. a property-based fault-injection harness that
// needs to inspect node state between individual events, not just after a
// whole Run — can drive the loop one event at a time.
func (s *Simulator) Step() bool {
	if s.queue.Len() == 0 {
		return false
	}

	ev := heap.Pop(&s.queue).(raft.Event)
	s.now = ev.At

	if !s.alive[ev.NodeID] {
		// A dead node is simply never delivered to — no "you're dead"
		// event, per context doc §4: a real crashed process doesn't get
		// a heads-up either.
		return true
	}

	node := s.nodes[ev.NodeID]
	s.applyOutbound(ev.NodeID, node, node.Step(ev, s.rng))
	return true
}

// applyOutbound turns everything a node's Step() asked for into simulator
// action: persisting, scheduling message delivery, scheduling the next
// timer fire, and recording applied entries.
//
// It also enforces the persist-before-respond rule at the boundary rather
// than trusting handler code blindly: an OutSendMessage is never allowed to
// precede the OutPersist it depends on within the same batch.
func (s *Simulator) applyOutbound(nodeID int, node *raft.NodeState, out []raft.Outbound) {
	sawSend := false
	for _, ob := range out {
		switch ob.Kind {
		case raft.OutPersist:
			if sawSend {
				panic("sim: OutPersist arrived after OutSendMessage in the same batch")
			}
			s.persisted[nodeID] = persistedState{
				Term:     ob.PersistedTerm,
				VotedFor: ob.PersistedVotedFor,
				LogLen:   ob.PersistedLogLen,
			}
		case raft.OutSendMessage:
			sawSend = true
			s.scheduleDelivery(nodeID, ob.To, ob.Message)
		case raft.OutResetTimer:
			s.schedule(raft.Event{
				At:             s.now + ob.Duration,
				NodeID:         nodeID,
				Kind:           raft.EventTimerFire,
				TimerKindField: ob.TimerKindField,
				TimerGen:       node.TimerGeneration(ob.TimerKindField),
			})
		case raft.OutApply:
			s.applied[nodeID] = append(s.applied[nodeID], appliedEntry{
				Index:   ob.ApplyIndex,
				Command: ob.ApplyCommand,
			})
		}
	}
}
