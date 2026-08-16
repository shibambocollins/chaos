package raft

import "testing"

func TestHandleRequestVoteReply_BecomesLeaderOnMajority(t *testing.T) {
	n := NewNodeState(1, []int{2, 3, 4, 5})
	rng := newRng()

	n.handleElectionTimeout(rng)
	if n.Role != Candidate {
		t.Fatalf("setup: expected Candidate, got %v", n.Role)
	}
	term := n.CurrentTerm

	grant := func(from int) []Outbound {
		return n.handleMessage(from, &RaftMessage{
			Kind: MsgRequestVoteReply, Term: term, VoteGranted: true,
		}, rng)
	}

	if out := grant(2); out != nil {
		t.Fatalf("expected no majority yet (2/5), got %v", out)
	}
	if n.Role != Candidate {
		t.Fatalf("expected still Candidate before majority, got %v", n.Role)
	}

	out := grant(3)
	if n.Role != Leader {
		t.Fatalf("expected Leader after majority, got %v", n.Role)
	}
	for _, peer := range []int{2, 3, 4, 5} {
		if n.NextIndex[peer] != 1 {
			t.Fatalf("expected NextIndex[%d]=1 on an empty log, got %d", peer, n.NextIndex[peer])
		}
	}

	sentTo := map[int]bool{}
	sawHeartbeatReset := false
	for _, ob := range out {
		if ob.Kind == OutSendMessage {
			if ob.Message.Kind != MsgAppendEntries {
				t.Fatalf("expected an immediate heartbeat AppendEntries, got %v", ob.Message.Kind)
			}
			sentTo[ob.To] = true
		}
		if ob.Kind == OutResetTimer && ob.TimerKindField == TimerHeartbeat {
			sawHeartbeatReset = true
		}
	}
	for _, peer := range []int{2, 3, 4, 5} {
		if !sentTo[peer] {
			t.Fatalf("expected an immediate heartbeat sent to peer %d on becoming leader", peer)
		}
	}
	if !sawHeartbeatReset {
		t.Fatalf("expected the heartbeat timer to start on becoming leader")
	}
}

func TestHandleRequestVoteReply_IgnoresStaleTermReply(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Candidate
	n.CurrentTerm = 3
	n.VotesReceived = map[int]bool{1: true}

	out := n.handleMessage(2, &RaftMessage{
		Kind: MsgRequestVoteReply, Term: 2, VoteGranted: true,
	}, newRng())

	if out != nil {
		t.Fatalf("expected nil for a reply from an older term, got %v", out)
	}
	if n.Role != Candidate {
		t.Fatalf("expected Role to remain Candidate, got %v", n.Role)
	}
}

func TestHandleRequestVoteReply_IgnoresIfNoLongerCandidate(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Follower
	n.CurrentTerm = 3

	out := n.handleMessage(2, &RaftMessage{
		Kind: MsgRequestVoteReply, Term: 3, VoteGranted: true,
	}, newRng())

	if out != nil {
		t.Fatalf("expected nil when no longer a Candidate, got %v", out)
	}
}

func TestHandleRequestVoteReply_DuplicateVoteAfterLeadershipIsIgnored(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	rng := newRng()
	n.handleElectionTimeout(rng)
	term := n.CurrentTerm

	n.handleMessage(2, &RaftMessage{Kind: MsgRequestVoteReply, Term: term, VoteGranted: true}, rng)
	if n.Role != Leader {
		t.Fatalf("setup: expected Leader after majority of 3, got %v", n.Role)
	}

	out := n.handleMessage(2, &RaftMessage{Kind: MsgRequestVoteReply, Term: term, VoteGranted: true}, rng)
	if out != nil {
		t.Fatalf("expected nil for a duplicate vote reply after already leader, got %v", out)
	}
}

func TestHandleRequestVoteReply_HigherTermStepsDownAndPersists(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Candidate
	n.CurrentTerm = 3
	n.VotesReceived = map[int]bool{1: true}

	out := n.handleMessage(2, &RaftMessage{
		Kind: MsgRequestVoteReply, Term: 5, VoteGranted: false,
	}, newRng())

	if n.CurrentTerm != 5 || n.Role != Follower {
		t.Fatalf("expected step-down to term 5/Follower, got term=%d role=%v", n.CurrentTerm, n.Role)
	}
	if len(out) != 1 || out[0].Kind != OutPersist || out[0].PersistedTerm != 5 {
		t.Fatalf("expected exactly one OutPersist for term 5, got %v", out)
	}
}
