package sim

import (
	"container/heap"

	"chaos/internal/raft"
)

// eventQueue is a min-heap of scheduled events ordered by (At, Seq). Seq is
// the deterministic tiebreak for events with equal At — container/heap does
// not guarantee stable ordering among equal-priority elements on its own,
// so without this two "identical" runs could deliver same-tick events in
// different orders and silently break replay.
type eventQueue []raft.Event

func (q eventQueue) Len() int { return len(q) }

func (q eventQueue) Less(i, j int) bool {
	if q[i].At != q[j].At {
		return q[i].At < q[j].At
	}
	return q[i].Seq < q[j].Seq
}

func (q eventQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }

func (q *eventQueue) Push(x any) {
	*q = append(*q, x.(raft.Event))
}

func (q *eventQueue) Pop() any {
	old := *q
	n := len(old)
	ev := old[n-1]
	*q = old[:n-1]
	return ev
}

var _ heap.Interface = (*eventQueue)(nil)
