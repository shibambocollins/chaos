package raft

import (
	"math/rand"
	"testing"
)

func newRng() *rand.Rand { return rand.New(rand.NewSource(1)) }

func TestHandleMessage_HigherTermStepsDownAndPersists(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Leader
	n.CurrentTerm = 3
	n.VotedFor = 1

	msg := &RaftMessage{
		Kind:         MsgRequestVote,
		Term:         5,
		CandidateID:  2,
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	out := n.handleMessage(2, msg, newRng())

	if n.CurrentTerm != 5 {
		t.Fatalf("expected step-down to term 5, got %d", n.CurrentTerm)
	}
	if n.Role != Follower {
		t.Fatalf("expected step-down to Follower, got %v", n.Role)
	}

	if len(out) == 0 || out[0].Kind != OutPersist {
		t.Fatalf("expected OutPersist first after a term step-down, got %v", out)
	}
	if out[0].PersistedTerm != 5 {
		t.Fatalf("expected persisted term 5, got %d", out[0].PersistedTerm)
	}
}

func TestHandleRequestVote_GrantsWhenLogIsUpToDateAndUnvoted(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.CurrentTerm = 5

	msg := &RaftMessage{
		Kind:         MsgRequestVote,
		Term:         5,
		CandidateID:  2,
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	out := n.handleMessage(2, msg, newRng())

	if n.VotedFor != 2 {
		t.Fatalf("expected VotedFor=2, got %d", n.VotedFor)
	}

	var gotPersist, gotResetTimer, gotReply bool
	var reply *RaftMessage
	for i, ob := range out {
		switch ob.Kind {
		case OutPersist:
			gotPersist = true
			if i != 0 {
				t.Fatalf("expected OutPersist first, got it at index %d", i)
			}
		case OutResetTimer:
			gotResetTimer = true
			if ob.TimerKindField != TimerElection {
				t.Fatalf("expected election timer reset, got %v", ob.TimerKindField)
			}
			if ob.Duration < electionTimeoutMin || ob.Duration > electionTimeoutMax {
				t.Fatalf("reset duration %d out of bounds", ob.Duration)
			}
		case OutSendMessage:
			gotReply = true
			reply = ob.Message
		}
	}
	if !gotPersist {
		t.Fatalf("expected OutPersist on a granted vote")
	}
	if !gotResetTimer {
		t.Fatalf("expected election timer reset on a granted vote")
	}
	if !gotReply || !reply.VoteGranted || reply.Term != 5 {
		t.Fatalf("expected a granted RequestVoteReply for term 5, got %v", reply)
	}
}

func TestHandleRequestVote_RejectsStaleLog(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.CurrentTerm = 5
	n.Log = []LogEntry{{Index: 1, Term: 4}, {Index: 2, Term: 5}}

	msg := &RaftMessage{
		Kind:         MsgRequestVote,
		Term:         5,
		CandidateID:  2,
		LastLogIndex: 1,
		LastLogTerm:  4,
	}

	out := n.handleMessage(2, msg, newRng())

	if n.VotedFor != -1 {
		t.Fatalf("expected VotedFor to remain unset, got %d", n.VotedFor)
	}

	var reply *RaftMessage
	for _, ob := range out {
		if ob.Kind == OutSendMessage {
			reply = ob.Message
		}
		if ob.Kind == OutResetTimer {
			t.Fatalf("did not expect an election timer reset on a rejected vote")
		}
	}
	if reply == nil || reply.VoteGranted {
		t.Fatalf("expected a rejected RequestVoteReply, got %v", reply)
	}
}

func TestHandleRequestVote_RejectsSecondCandidateSameTerm(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.CurrentTerm = 5
	n.VotedFor = 2

	msg := &RaftMessage{
		Kind:         MsgRequestVote,
		Term:         5,
		CandidateID:  3,
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	out := n.handleMessage(2, msg, newRng())

	if n.VotedFor != 2 {
		t.Fatalf("expected VotedFor to remain 2, got %d", n.VotedFor)
	}

	var reply *RaftMessage
	for _, ob := range out {
		if ob.Kind == OutSendMessage {
			reply = ob.Message
		}
	}
	if reply == nil || reply.VoteGranted {
		t.Fatalf("expected a rejected RequestVoteReply for a second candidate, got %v", reply)
	}
}

func TestHandleRequestVote_IdempotentForSameCandidateSameTerm(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.CurrentTerm = 5
	n.VotedFor = 2

	msg := &RaftMessage{
		Kind:         MsgRequestVote,
		Term:         5,
		CandidateID:  2,
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	out := n.handleMessage(2, msg, newRng())

	var reply *RaftMessage
	for _, ob := range out {
		if ob.Kind == OutSendMessage {
			reply = ob.Message
		}
	}
	if reply == nil || !reply.VoteGranted {
		t.Fatalf("expected a re-granted RequestVoteReply for a duplicate request from the same candidate, got %v", reply)
	}
}
