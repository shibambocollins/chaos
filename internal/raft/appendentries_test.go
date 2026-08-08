package raft

import "testing"

func replyOf(out []Outbound) *RaftMessage {
	for _, ob := range out {
		if ob.Kind == OutSendMessage {
			return ob.Message
		}
	}
	return nil
}

func TestHandleAppendEntries_RejectsStaleTerm(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.CurrentTerm = 5

	out := n.handleMessage(2, &RaftMessage{
		Kind: MsgAppendEntries, Term: 3, LeaderID: 2,
	}, newRng())

	reply := replyOf(out)
	if reply == nil || reply.Success || reply.Term != 5 {
		t.Fatalf("expected a rejected reply carrying term 5, got %v", reply)
	}
	for _, ob := range out {
		if ob.Kind == OutResetTimer {
			t.Fatalf("did not expect a timer reset for a stale leader")
		}
	}
}

func TestHandleAppendEntries_CandidateStepsDownOnEqualTermLeader(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Candidate
	n.CurrentTerm = 5

	out := n.handleMessage(2, &RaftMessage{
		Kind: MsgAppendEntries, Term: 5, LeaderID: 2,
	}, newRng())

	if n.Role != Follower {
		t.Fatalf("expected Candidate to concede to a same-term leader, got %v", n.Role)
	}
	reply := replyOf(out)
	if reply == nil || !reply.Success {
		t.Fatalf("expected a successful reply, got %v", reply)
	}
}

func TestHandleAppendEntries_RejectsOnLogMismatch(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.CurrentTerm = 3
	n.Log = []LogEntry{{Index: 1, Term: 1}}

	out := n.handleMessage(2, &RaftMessage{
		Kind: MsgAppendEntries, Term: 3, LeaderID: 2,
		PrevLogIndex: 1, PrevLogTerm: 2, // we have index 1 but at term 1, not 2
	}, newRng())

	reply := replyOf(out)
	if reply == nil || reply.Success {
		t.Fatalf("expected a rejected reply on log mismatch, got %v", reply)
	}
	if len(n.Log) != 1 {
		t.Fatalf("expected log untouched on rejection, got %v", n.Log)
	}
	sawReset := false
	for _, ob := range out {
		if ob.Kind == OutResetTimer {
			sawReset = true
		}
	}
	if !sawReset {
		t.Fatalf("expected the election timer to still reset — this is a legitimate leader")
	}
}

func TestHandleAppendEntries_AppendsToEmptyLog(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.CurrentTerm = 1

	entries := []LogEntry{{Index: 1, Term: 1, Command: []byte("a")}, {Index: 2, Term: 1, Command: []byte("b")}}
	out := n.handleMessage(2, &RaftMessage{
		Kind: MsgAppendEntries, Term: 1, LeaderID: 2,
		PrevLogIndex: 0, PrevLogTerm: 0, Entries: entries,
	}, newRng())

	if len(n.Log) != 2 || n.Log[1].Command[0] != 'b' {
		t.Fatalf("expected both entries appended, got %v", n.Log)
	}
	reply := replyOf(out)
	if reply == nil || !reply.Success {
		t.Fatalf("expected a successful reply, got %v", reply)
	}
	if out[0].Kind != OutPersist {
		t.Fatalf("expected OutPersist first for a log change, got %v", out)
	}
}

func TestHandleAppendEntries_TruncatesConflictingSuffix(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.CurrentTerm = 3
	// Follower has a stale entry at index 2 (term 1) that conflicts with
	// what the (new-term) leader is now sending for that index.
	n.Log = []LogEntry{{Index: 1, Term: 1}, {Index: 2, Term: 1}, {Index: 3, Term: 1}}

	newEntry := LogEntry{Index: 2, Term: 3, Command: []byte("new")}
	out := n.handleMessage(2, &RaftMessage{
		Kind: MsgAppendEntries, Term: 3, LeaderID: 2,
		PrevLogIndex: 1, PrevLogTerm: 1, Entries: []LogEntry{newEntry},
	}, newRng())

	if len(n.Log) != 2 {
		t.Fatalf("expected the conflicting old index-3 entry discarded, got %v", n.Log)
	}
	if n.Log[1].Term != 3 || string(n.Log[1].Command) != "new" {
		t.Fatalf("expected index 2 replaced with the new entry, got %v", n.Log[1])
	}
	reply := replyOf(out)
	if reply == nil || !reply.Success {
		t.Fatalf("expected a successful reply, got %v", reply)
	}
}

func TestHandleAppendEntries_SkipsAlreadyMatchingEntries(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.CurrentTerm = 3
	n.Log = []LogEntry{{Index: 1, Term: 1}, {Index: 2, Term: 1}}

	// Leader resends an entry the follower already has, identical term —
	// e.g. a duplicated heartbeat/replication message.
	out := n.handleMessage(2, &RaftMessage{
		Kind: MsgAppendEntries, Term: 3, LeaderID: 2,
		PrevLogIndex: 0, PrevLogTerm: 0, Entries: []LogEntry{{Index: 1, Term: 1}},
	}, newRng())

	if len(n.Log) != 2 || n.Log[0].Term != 1 {
		t.Fatalf("expected log unchanged, got %v", n.Log)
	}
	for _, ob := range out {
		if ob.Kind == OutPersist {
			t.Fatalf("did not expect OutPersist when nothing actually changed, got %v", out)
		}
	}
}

func TestHandleAppendEntries_AdvancesCommitIndexAndApplies(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.CurrentTerm = 1

	entries := []LogEntry{
		{Index: 1, Term: 1, Command: []byte("a")},
		{Index: 2, Term: 1, Command: []byte("b")},
		{Index: 3, Term: 1, Command: []byte("c")},
	}
	out := n.handleMessage(2, &RaftMessage{
		Kind: MsgAppendEntries, Term: 1, LeaderID: 2,
		PrevLogIndex: 0, PrevLogTerm: 0, Entries: entries, LeaderCommit: 2,
	}, newRng())

	if n.CommitIndex != 2 {
		t.Fatalf("expected CommitIndex=2 (min of leaderCommit=2, lastNew=3), got %d", n.CommitIndex)
	}
	if n.LastApplied != 2 {
		t.Fatalf("expected LastApplied to catch up to 2, got %d", n.LastApplied)
	}

	var applied []uint64
	for _, ob := range out {
		if ob.Kind == OutApply {
			applied = append(applied, ob.ApplyIndex)
		}
	}
	if len(applied) != 2 || applied[0] != 1 || applied[1] != 2 {
		t.Fatalf("expected OutApply for indices 1 and 2, got %v", applied)
	}
}

func TestHandleAppendEntries_HigherTermPersistsEvenOnLogMismatch(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.CurrentTerm = 3
	n.Log = []LogEntry{{Index: 1, Term: 1}}

	out := n.handleMessage(2, &RaftMessage{
		Kind: MsgAppendEntries, Term: 5, LeaderID: 2, // higher term
		PrevLogIndex: 1, PrevLogTerm: 9, // mismatched
	}, newRng())

	if n.CurrentTerm != 5 {
		t.Fatalf("expected step-down to term 5, got %d", n.CurrentTerm)
	}
	if out[0].Kind != OutPersist || out[0].PersistedTerm != 5 {
		t.Fatalf("expected OutPersist first for the term step-down, got %v", out)
	}
	reply := replyOf(out)
	if reply == nil || reply.Success {
		t.Fatalf("expected a rejected reply on log mismatch, got %v", reply)
	}
}
