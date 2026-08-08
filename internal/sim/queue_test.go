package sim

import (
	"container/heap"
	"testing"

	"chaos/internal/raft"
)

func TestEventQueue_OrdersByAtThenSeq(t *testing.T) {
	q := &eventQueue{}
	heap.Init(q)

	heap.Push(q, raft.Event{At: 5, Seq: 1, NodeID: 100})
	heap.Push(q, raft.Event{At: 2, Seq: 2, NodeID: 200})
	heap.Push(q, raft.Event{At: 2, Seq: 0, NodeID: 300}) // same At as above, lower Seq — must come first
	heap.Push(q, raft.Event{At: 5, Seq: 0, NodeID: 400})

	want := []int{300, 200, 400, 100}
	for _, wantNode := range want {
		got := heap.Pop(q).(raft.Event)
		if got.NodeID != wantNode {
			t.Fatalf("expected NodeID %d, got %d (At=%d Seq=%d)", wantNode, got.NodeID, got.At, got.Seq)
		}
	}
	if q.Len() != 0 {
		t.Fatalf("expected queue drained, got %d remaining", q.Len())
	}
}
