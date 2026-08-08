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
