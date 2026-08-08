package raft

import "testing"

func TestHandleAppendEntriesReply_IgnoresStaleReply(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Follower // not leader
	n.CurrentTerm = 3

	out := n.handleMessage(2, &RaftMessage{
		Kind: MsgAppendEntriesReply, Term: 3, Success: true, MatchIndex: 5,
	}, newRng())

	if out != nil {
		t.Fatalf("expected nil for a reply while not leader, got %v", out)
	}
}

func TestHandleAppendEntriesReply_BacksOffNextIndexOnFailure(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Leader
	n.CurrentTerm = 3
	n.NextIndex = map[int]uint64{2: 5, 3: 5}
	n.MatchIndex = map[int]uint64{2: 0, 3: 0}

	n.handleMessage(2, &RaftMessage{Kind: MsgAppendEntriesReply, Term: 3, Success: false}, newRng())

	if n.NextIndex[2] != 4 {
		t.Fatalf("expected NextIndex[2] backed off to 4, got %d", n.NextIndex[2])
	}
	if n.NextIndex[3] != 5 {
		t.Fatalf("expected NextIndex[3] untouched, got %d", n.NextIndex[3])
	}
}

func TestHandleAppendEntriesReply_NextIndexNeverGoesBelowOne(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Leader
	n.CurrentTerm = 3
	n.NextIndex = map[int]uint64{2: 1, 3: 1}
	n.MatchIndex = map[int]uint64{2: 0, 3: 0}

	n.handleMessage(2, &RaftMessage{Kind: MsgAppendEntriesReply, Term: 3, Success: false}, newRng())

	if n.NextIndex[2] != 1 {
		t.Fatalf("expected NextIndex[2] to stay at the floor of 1, got %d", n.NextIndex[2])
	}
}

func TestHandleAppendEntriesReply_AdvancesCommitIndexOnMajorityCurrentTerm(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Leader
	n.CurrentTerm = 2
	n.Log = []LogEntry{{Index: 1, Term: 2, Command: []byte("x")}}
	n.NextIndex = map[int]uint64{2: 2, 3: 2}
	n.MatchIndex = map[int]uint64{2: 0, 3: 0}

	out := n.handleMessage(2, &RaftMessage{Kind: MsgAppendEntriesReply, Term: 2, Success: true, MatchIndex: 1}, newRng())

	if n.CommitIndex != 1 {
		t.Fatalf("expected CommitIndex=1 with self+peer2 forming a majority of 3, got %d", n.CommitIndex)
	}
	var applied []uint64
	for _, ob := range out {
		if ob.Kind == OutApply {
			applied = append(applied, ob.ApplyIndex)
		}
	}
	if len(applied) != 1 || applied[0] != 1 {
		t.Fatalf("expected OutApply for index 1, got %v", applied)
	}
}

// TestHandleAppendEntriesReply_DoesNotCommitOlderTermEntryDirectly is the
// deliberate Figure-8-style regression test the context doc calls for
// (§3, Gotcha #2): a majority can hold a copy of an older-term entry
// without it being safe to commit directly off that count.
func TestHandleAppendEntriesReply_DoesNotCommitOlderTermEntryDirectly(t *testing.T) {
	n := NewNodeState(1, []int{2, 3, 4, 5})
	n.Role = Leader
	n.CurrentTerm = 3
	n.Log = []LogEntry{{Index: 1, Term: 1, Command: []byte("old")}} // entry from an OLDER term
	n.NextIndex = map[int]uint64{2: 2, 3: 2, 4: 2, 5: 2}
	n.MatchIndex = map[int]uint64{2: 0, 3: 0, 4: 0, 5: 0}

	// Two peers (a majority alongside self, 3 of 5) now report matching
	// index 1 — but that entry is from term 1, not the leader's current
	// term 3, so it must NOT be committed directly on this basis.
	n.handleMessage(2, &RaftMessage{Kind: MsgAppendEntriesReply, Term: 3, Success: true, MatchIndex: 1}, newRng())
	out := n.handleMessage(3, &RaftMessage{Kind: MsgAppendEntriesReply, Term: 3, Success: true, MatchIndex: 1}, newRng())

	if n.CommitIndex != 0 {
		t.Fatalf("Gotcha #2 violated: committed an older-term entry directly off a majority count, CommitIndex=%d", n.CommitIndex)
	}
	if out != nil {
		t.Fatalf("expected no OutApply since nothing was committed, got %v", out)
	}
}

// TestHandleAppendEntriesReply_OlderEntryCommitsIndirectlyViaNewerEntry
// completes the Figure-8 story: once a majority also replicates a
// CURRENT-term entry, CommitIndex can advance past it — which commits
// the earlier older-term entry too, indirectly, as a side effect.
func TestHandleAppendEntriesReply_OlderEntryCommitsIndirectlyViaNewerEntry(t *testing.T) {
	n := NewNodeState(1, []int{2, 3, 4, 5})
	n.Role = Leader
	n.CurrentTerm = 3
	n.Log = []LogEntry{
		{Index: 1, Term: 1, Command: []byte("old")},
		{Index: 2, Term: 3, Command: []byte("new")}, // leader's own current-term entry
	}
	n.NextIndex = map[int]uint64{2: 3, 3: 3, 4: 3, 5: 3}
	n.MatchIndex = map[int]uint64{2: 0, 3: 0, 4: 0, 5: 0}

	n.handleMessage(2, &RaftMessage{Kind: MsgAppendEntriesReply, Term: 3, Success: true, MatchIndex: 2}, newRng())
	out := n.handleMessage(3, &RaftMessage{Kind: MsgAppendEntriesReply, Term: 3, Success: true, MatchIndex: 2}, newRng())

	if n.CommitIndex != 2 {
		t.Fatalf("expected CommitIndex=2 once the current-term entry reaches a majority, got %d", n.CommitIndex)
	}
	var applied []uint64
	for _, ob := range out {
		if ob.Kind == OutApply {
			applied = append(applied, ob.ApplyIndex)
		}
	}
	if len(applied) != 2 || applied[0] != 1 || applied[1] != 2 {
		t.Fatalf("expected both index 1 and 2 applied together, got %v", applied)
	}
}

func TestHandleAppendEntriesReply_MatchIndexNeverMovesBackward(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Leader
	n.CurrentTerm = 1
	n.Log = []LogEntry{{Index: 1, Term: 1}, {Index: 2, Term: 1}, {Index: 3, Term: 1}}
	n.NextIndex = map[int]uint64{2: 4, 3: 4}
	n.MatchIndex = map[int]uint64{2: 3, 3: 0} // peer 2 already confirmed up to index 3

	// A stale/reordered reply about an earlier request arrives after the
	// fact, claiming only index 1 — must not drag MatchIndex backward.
	n.handleMessage(2, &RaftMessage{Kind: MsgAppendEntriesReply, Term: 1, Success: true, MatchIndex: 1}, newRng())

	if n.MatchIndex[2] != 3 {
		t.Fatalf("expected MatchIndex[2] to stay at 3, got %d", n.MatchIndex[2])
	}
}

func TestHandleAppendEntriesReply_HigherTermStepsDownAndPersists(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Leader
	n.CurrentTerm = 3

	out := n.handleMessage(2, &RaftMessage{Kind: MsgAppendEntriesReply, Term: 5, Success: false}, newRng())

	if n.CurrentTerm != 5 || n.Role != Follower {
		t.Fatalf("expected step-down to term 5/Follower, got term=%d role=%v", n.CurrentTerm, n.Role)
	}
	if len(out) != 1 || out[0].Kind != OutPersist || out[0].PersistedTerm != 5 {
		t.Fatalf("expected exactly one OutPersist for term 5, got %v", out)
	}
}
