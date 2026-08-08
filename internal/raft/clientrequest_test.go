package raft

import (
	"bytes"
	"testing"
)

func logEntryEqual(a, b LogEntry) bool {
	return a.Term == b.Term && a.Index == b.Index && bytes.Equal(a.Command, b.Command)
}

func TestHandleClientRequest_LeaderAppendsAndReplicatesImmediately(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Leader
	n.CurrentTerm = 3
	n.NextIndex = map[int]uint64{2: 1, 3: 1}
	n.MatchIndex = map[int]uint64{2: 0, 3: 0}

	out := n.Step(Event{Kind: EventClientRequest, Command: []byte("cmd")}, nil)

	if len(n.Log) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(n.Log))
	}
	want := LogEntry{Term: 3, Index: 1, Command: []byte("cmd")}
	if !logEntryEqual(n.Log[0], want) {
		t.Fatalf("expected log entry %+v, got %+v", want, n.Log[0])
	}

	persistIdx, sentTo := -1, map[int]bool{}
	for i, ob := range out {
		if ob.Kind == OutPersist && persistIdx == -1 {
			persistIdx = i
		}
		if ob.Kind == OutSendMessage {
			if persistIdx == -1 || persistIdx > i {
				t.Fatalf("expected OutPersist before OutSendMessage to %d", ob.To)
			}
			if ob.Message.Kind != MsgAppendEntries {
				t.Fatalf("expected MsgAppendEntries, got %v", ob.Message.Kind)
			}
			if ob.Message.PrevLogIndex != 0 || ob.Message.PrevLogTerm != 0 {
				t.Fatalf("expected PrevLogIndex/Term 0/0, got %d/%d", ob.Message.PrevLogIndex, ob.Message.PrevLogTerm)
			}
			if len(ob.Message.Entries) != 1 || !logEntryEqual(ob.Message.Entries[0], want) {
				t.Fatalf("expected entries [%+v], got %+v", want, ob.Message.Entries)
			}
			sentTo[ob.To] = true
		}
	}
	if persistIdx == -1 {
		t.Fatalf("expected an OutPersist entry, got none")
	}
	for _, peer := range []int{2, 3} {
		if !sentTo[peer] {
			t.Fatalf("expected immediate AppendEntries sent to peer %d", peer)
		}
	}
}

func TestHandleClientRequest_NonLeaderIgnoresIt(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Follower

	out := n.Step(Event{Kind: EventClientRequest, Command: []byte("cmd")}, nil)

	if out != nil {
		t.Fatalf("expected nil Outbound when a non-leader receives a client request, got %v", out)
	}
	if len(n.Log) != 0 {
		t.Fatalf("expected log untouched, got %d entries", len(n.Log))
	}
}

func TestHandleHeartbeatTimeout_CarriesEachFollowersMissingEntries(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Leader
	n.CurrentTerm = 5
	n.Log = []LogEntry{
		{Term: 5, Index: 1, Command: []byte("a")},
		{Term: 5, Index: 2, Command: []byte("b")},
	}
	// Peer 2 has replicated nothing yet; peer 3 is fully caught up.
	n.NextIndex = map[int]uint64{2: 1, 3: 3}
	n.MatchIndex = map[int]uint64{2: 0, 3: 2}

	out := n.handleHeartbeatTimeout()

	byPeer := map[int]*RaftMessage{}
	for _, ob := range out {
		if ob.Kind == OutSendMessage {
			byPeer[ob.To] = ob.Message
		}
	}

	msg2 := byPeer[2]
	if msg2 == nil {
		t.Fatalf("expected a message sent to peer 2")
	}
	if msg2.PrevLogIndex != 0 || msg2.PrevLogTerm != 0 {
		t.Fatalf("peer 2: expected PrevLogIndex/Term 0/0, got %d/%d", msg2.PrevLogIndex, msg2.PrevLogTerm)
	}
	if len(msg2.Entries) != 2 || !logEntryEqual(msg2.Entries[0], n.Log[0]) || !logEntryEqual(msg2.Entries[1], n.Log[1]) {
		t.Fatalf("peer 2: expected both log entries, got %+v", msg2.Entries)
	}

	msg3 := byPeer[3]
	if msg3 == nil {
		t.Fatalf("expected a message sent to peer 3")
	}
	if msg3.PrevLogIndex != 2 || msg3.PrevLogTerm != 5 {
		t.Fatalf("peer 3: expected PrevLogIndex/Term 2/5, got %d/%d", msg3.PrevLogIndex, msg3.PrevLogTerm)
	}
	if len(msg3.Entries) != 0 {
		t.Fatalf("peer 3: expected no entries (already caught up), got %+v", msg3.Entries)
	}
}
