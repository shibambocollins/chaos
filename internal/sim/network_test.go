package sim

import (
	"math/rand"
	"testing"

	"chaos/internal/raft"
)

func TestScheduleDelivery_SchedulesMessageArrivalWithinLatencyBounds(t *testing.T) {
	nodes := map[int]*raft.NodeState{
		1: raft.NewNodeState(1, []int{2}),
		2: raft.NewNodeState(2, []int{1}),
	}
	s := NewSimulator(nodes, rand.New(rand.NewSource(1)))
	s.now = 10

	msg := &raft.RaftMessage{Kind: raft.MsgAppendEntries, Term: 3}
	s.scheduleDelivery(1, 2, msg)

	if s.queue.Len() != 1 {
		t.Fatalf("expected 1 scheduled event, got %d", s.queue.Len())
	}
	ev := s.queue[0]
	if ev.Kind != raft.EventMessageArrival {
		t.Fatalf("expected EventMessageArrival, got %v", ev.Kind)
	}
	if ev.NodeID != 2 || ev.From != 1 || ev.Message != msg {
		t.Fatalf("expected arrival at 2 from 1 carrying msg, got NodeID=%d From=%d Message=%v", ev.NodeID, ev.From, ev.Message)
	}
	if ev.At < s.now+latencyMin || ev.At > s.now+latencyMax {
		t.Fatalf("expected At within [%d,%d], got %d", s.now+latencyMin, s.now+latencyMax, ev.At)
	}
}

func TestScheduleDelivery_DropProbabilityOneDropsEverything(t *testing.T) {
	nodes := map[int]*raft.NodeState{
		1: raft.NewNodeState(1, []int{2}),
		2: raft.NewNodeState(2, []int{1}),
	}
	s := NewSimulator(nodes, rand.New(rand.NewSource(1)))
	s.SetFaultConfig(FaultConfig{DropProbability: 1})

	s.scheduleDelivery(1, 2, &raft.RaftMessage{})

	if s.queue.Len() != 0 {
		t.Fatalf("expected the message to be dropped, got %d scheduled", s.queue.Len())
	}
}

func TestScheduleDelivery_DuplicateProbabilityOneSchedulesTwice(t *testing.T) {
	nodes := map[int]*raft.NodeState{
		1: raft.NewNodeState(1, []int{2}),
		2: raft.NewNodeState(2, []int{1}),
	}
	s := NewSimulator(nodes, rand.New(rand.NewSource(1)))
	s.SetFaultConfig(FaultConfig{DuplicateProbability: 1})

	s.scheduleDelivery(1, 2, &raft.RaftMessage{})

	if s.queue.Len() != 2 {
		t.Fatalf("expected the message to be duplicated into 2 scheduled events, got %d", s.queue.Len())
	}
}

func TestScheduleDelivery_PartitionedPairDropsRegardlessOfFaultConfig(t *testing.T) {
	nodes := map[int]*raft.NodeState{
		1: raft.NewNodeState(1, []int{2}),
		2: raft.NewNodeState(2, []int{1}),
	}
	s := NewSimulator(nodes, rand.New(rand.NewSource(1)))
	s.Partition([][]int{{1}, {2}})

	s.scheduleDelivery(1, 2, &raft.RaftMessage{})

	if s.queue.Len() != 0 {
		t.Fatalf("expected a partitioned send to be dropped, got %d scheduled", s.queue.Len())
	}
}

func TestScheduleDelivery_AssignsIncreasingSeq(t *testing.T) {
	nodes := map[int]*raft.NodeState{
		1: raft.NewNodeState(1, []int{2}),
		2: raft.NewNodeState(2, []int{1}),
	}
	s := NewSimulator(nodes, rand.New(rand.NewSource(1)))

	s.scheduleDelivery(1, 2, &raft.RaftMessage{})
	s.scheduleDelivery(2, 1, &raft.RaftMessage{})

	if s.queue[0].Seq == s.queue[1].Seq {
		t.Fatalf("expected distinct Seq values, both were %d", s.queue[0].Seq)
	}
}
