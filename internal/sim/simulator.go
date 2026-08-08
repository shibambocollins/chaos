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

	// persisted/applied record what each node's OutPersist/OutApply
	// outbounds reported. Modeled as an ordering guarantee the simulator
	// enforces (OutPersist before any OutSendMessage in the same batch),
	// not real disk I/O — see context doc's persist-before-respond note.
	persisted map[int]persistedState
	applied   map[int][]appliedEntry
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

// Run pops and processes every scheduled event with At <= until, in
// (At, Seq) order. Returns once the queue is empty or the next event is
// past the horizon — the caller can keep calling Run with a later horizon
// to keep advancing.
func (s *Simulator) Run(until raft.Time) {
	for s.queue.Len() > 0 && s.queue[0].At <= until {
		s.step()
	}
}

// step delivers the single next event to its target node's Step() and
// applies every Outbound the node returns.
func (s *Simulator) step() {
	ev := heap.Pop(&s.queue).(raft.Event)
	s.now = ev.At

	if !s.alive[ev.NodeID] {
		// A dead node is simply never delivered to — no "you're dead"
		// event, per context doc §4: a real crashed process doesn't get
		// a heads-up either.
		return
	}

	node := s.nodes[ev.NodeID]
	s.applyOutbound(ev.NodeID, node, node.Step(ev, s.rng))
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
