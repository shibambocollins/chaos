package raft

import (
	"math/rand"
	"testing"
)

func TestHandleElectionTimeout_BecomesCandidateAndBroadcastsRequestVote(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.CurrentTerm = 4
	rng := rand.New(rand.NewSource(1))

	out := n.Step(Event{Kind: EventTimerFire, TimerKindField: TimerElection, TimerGen: 0}, rng)

	if n.CurrentTerm != 5 {
		t.Fatalf("expected CurrentTerm to advance to 5, got %d", n.CurrentTerm)
	}
	if n.Role != Candidate {
		t.Fatalf("expected Role Candidate, got %v", n.Role)
	}
	if n.VotedFor != n.ID {
		t.Fatalf("expected VotedFor to be self (%d), got %d", n.ID, n.VotedFor)
	}
	if !n.VotesReceived[n.ID] {
		t.Fatalf("expected candidate to have voted for itself")
	}

	persistIdx, sendIdx := -1, -1
	requestVotesTo := map[int]bool{}
	for i, ob := range out {
		if ob.Kind == OutPersist && persistIdx == -1 {
			persistIdx = i
		}
		if ob.Kind == OutSendMessage {
			if sendIdx == -1 {
				sendIdx = i
			}
			if ob.Message.Kind != MsgRequestVote {
				t.Fatalf("expected MsgRequestVote, got %v", ob.Message.Kind)
			}
			if ob.Message.Term != 5 {
				t.Fatalf("expected RequestVote term 5, got %d", ob.Message.Term)
			}
			requestVotesTo[ob.To] = true
		}
	}
	if persistIdx == -1 {
		t.Fatalf("expected an OutPersist entry, got none")
	}
	if sendIdx == -1 || persistIdx > sendIdx {
		t.Fatalf("expected OutPersist before OutSendMessage, persistIdx=%d sendIdx=%d", persistIdx, sendIdx)
	}
	for _, peer := range []int{2, 3} {
		if !requestVotesTo[peer] {
			t.Fatalf("expected a RequestVote sent to peer %d", peer)
		}
	}

	found := false
	for _, ob := range out {
		if ob.Kind == OutResetTimer && ob.TimerKindField == TimerElection {
			found = true
			if ob.Duration < electionTimeoutMin || ob.Duration > electionTimeoutMax {
				t.Fatalf("election timeout %d out of bounds [%d,%d]", ob.Duration, electionTimeoutMin, electionTimeoutMax)
			}
		}
	}
	if !found {
		t.Fatalf("expected an OutResetTimer for TimerElection")
	}
}

func TestHandleElectionTimeout_LeaderIgnoresIt(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Leader
	n.CurrentTerm = 4
	rng := rand.New(rand.NewSource(1))

	out := n.handleElectionTimeout(rng)

	if out != nil {
		t.Fatalf("expected nil Outbound when a leader's election timer spuriously fires, got %v", out)
	}
	if n.CurrentTerm != 4 || n.Role != Leader {
		t.Fatalf("expected leader's state to be untouched, got term=%d role=%v", n.CurrentTerm, n.Role)
	}
}

func TestHandleHeartbeatTimeout_LeaderBroadcastsAppendEntries(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Leader
	n.CurrentTerm = 7
	n.CommitIndex = 2

	out := n.handleHeartbeatTimeout()

	sentTo := map[int]bool{}
	for _, ob := range out {
		if ob.Kind == OutResetTimer {
			if ob.TimerKindField != TimerHeartbeat || ob.Duration != heartbeatInterval {
				t.Fatalf("expected heartbeat reset with duration %d, got kind=%v duration=%d", heartbeatInterval, ob.TimerKindField, ob.Duration)
			}
		}
		if ob.Kind == OutSendMessage {
			if ob.Message.Kind != MsgAppendEntries {
				t.Fatalf("expected MsgAppendEntries, got %v", ob.Message.Kind)
			}
			if ob.Message.Term != 7 || ob.Message.LeaderCommit != 2 {
				t.Fatalf("expected term=7 leaderCommit=2, got term=%d leaderCommit=%d", ob.Message.Term, ob.Message.LeaderCommit)
			}
			sentTo[ob.To] = true
		}
	}
	for _, peer := range []int{2, 3} {
		if !sentTo[peer] {
			t.Fatalf("expected a heartbeat sent to peer %d", peer)
		}
	}
}

func TestHandleHeartbeatTimeout_NonLeaderIgnoresIt(t *testing.T) {
	n := NewNodeState(1, []int{2, 3})
	n.Role = Follower

	out := n.handleHeartbeatTimeout()

	if out != nil {
		t.Fatalf("expected nil Outbound when a non-leader's heartbeat timer fires, got %v", out)
	}
}
