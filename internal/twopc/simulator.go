package twopc

import (
	"container/heap"
	"sort"
)

type eventQueue []Event

func (q eventQueue) Len() int { return len(q) }

func (q eventQueue) Less(i, j int) bool {
	if q[i].At != q[j].At {
		return q[i].At < q[j].At
	}
	return q[i].Seq < q[j].Seq
}

func (q eventQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }

func (q *eventQueue) Push(x any) { *q = append(*q, x.(Event)) }

func (q *eventQueue) Pop() any {
	old := *q
	n := len(old)
	ev := old[n-1]
	*q = old[:n-1]
	return ev
}

const messageLatency Time = 2

// Simulator drives a set of 2PC NodeState instances through a single
// deterministic event loop — same discipline as internal/sim.Simulator,
// deliberately smaller: no fault injection beyond Kill/Restart, since
// demonstrating the coordinator-blocking problem is all this package needs
// from its simulator.
type Simulator struct {
	nodes   map[int]*NodeState
	nodeIDs []int

	queue   eventQueue
	nextSeq uint64
	now     Time

	alive map[int]bool
}

// NewSimulator wires up a fresh event loop over nodes, keyed by node ID.
// Every node starts alive; the queue starts empty.
func NewSimulator(nodes map[int]*NodeState) *Simulator {
	ids := make([]int, 0, len(nodes))
	alive := make(map[int]bool, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
		alive[id] = true
	}
	sort.Ints(ids)

	return &Simulator{nodes: nodes, nodeIDs: ids, alive: alive}
}

// Now returns the simulator's current simulated time.
func (s *Simulator) Now() Time { return s.now }

// IsAlive reports whether id is currently up.
func (s *Simulator) IsAlive(id int) bool { return s.alive[id] }

// Node returns id's current state, for inspecting a run's outcome.
func (s *Simulator) Node(id int) *NodeState { return s.nodes[id] }

// Schedule adds ev to the queue, assigning it the next Seq.
func (s *Simulator) Schedule(ev Event) {
	ev.Seq = s.nextSeq
	s.nextSeq++
	heap.Push(&s.queue, ev)
}

// Kill marks id as down — no "you're dead" event, it simply stops being
// delivered to.
func (s *Simulator) Kill(id int) {
	s.alive[id] = false
}

// Restart brings id back up and runs its NodeState.Restart outbound
// through the same applyOutbound path every other Step result goes
// through.
func (s *Simulator) Restart(id int) {
	s.alive[id] = true
	node := s.nodes[id]
	s.applyOutbound(id, node, node.Restart())
}

// Run pops and processes every scheduled event with At <= until.
func (s *Simulator) Run(until Time) {
	for s.queue.Len() > 0 && s.queue[0].At <= until {
		s.Step()
	}
}

// Step pops and delivers the single next scheduled event. Reports false if
// the queue is empty.
func (s *Simulator) Step() bool {
	if s.queue.Len() == 0 {
		return false
	}

	ev := heap.Pop(&s.queue).(Event)
	s.now = ev.At

	if !s.alive[ev.NodeID] {
		return true
	}

	node := s.nodes[ev.NodeID]
	s.applyOutbound(ev.NodeID, node, node.Step(ev))
	return true
}

func (s *Simulator) applyOutbound(nodeID int, node *NodeState, out []Outbound) {
	sawSend := false
	for _, ob := range out {
		switch ob.Kind {
		case OutPersist:
			if sawSend {
				panic("twopc: OutPersist arrived after OutSendMessage in the same batch")
			}

		case OutSendMessage:
			sawSend = true
			s.Schedule(Event{
				At:      s.now + messageLatency,
				NodeID:  ob.To,
				Kind:    EventMessageArrival,
				From:    nodeID,
				Message: ob.Message,
			})
		case OutResetTimer:
			s.Schedule(Event{
				At:             s.now + ob.Duration,
				NodeID:         nodeID,
				Kind:           EventTimerFire,
				TimerKindField: ob.TimerKindField,
				TimerGen:       node.TimerGeneration(ob.TimerKindField),
			})
		case OutApply:

		}
	}
}
