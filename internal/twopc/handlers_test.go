package twopc

import "testing"

func TestHandleClientRequest_CoordinatorSendsPrepareToEveryParticipant(t *testing.T) {
	n := NewCoordinator(1, []int{2, 3})

	out := n.Step(Event{Kind: EventClientRequest, Command: []byte("txn")})

	if n.CoordinatorState != WaitingForVotes {
		t.Fatalf("expected CoordinatorState WaitingForVotes, got %v", n.CoordinatorState)
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
			if ob.Message.Kind != MsgPrepare || string(ob.Message.Command) != "txn" {
				t.Fatalf("expected MsgPrepare carrying the command, got %+v", ob.Message)
			}
			sentTo[ob.To] = true
		}
	}
	for _, peer := range []int{2, 3} {
		if !sentTo[peer] {
			t.Fatalf("expected Prepare sent to peer %d", peer)
		}
	}
}

func TestHandleClientRequest_CoordinatorIgnoresSecondRequestMidTransaction(t *testing.T) {
	n := NewCoordinator(1, []int{2, 3})
	n.Step(Event{Kind: EventClientRequest, Command: []byte("first")})

	out := n.Step(Event{Kind: EventClientRequest, Command: []byte("second")})

	if out != nil {
		t.Fatalf("expected nil Outbound for a second request while mid-transaction, got %v", out)
	}
	if string(n.Command) != "first" {
		t.Fatalf("expected the in-flight transaction untouched, got command %q", n.Command)
	}
}

func TestHandlePrepare_ParticipantVotesCommitAndEntersPrepared(t *testing.T) {
	n := NewParticipant(2, 1, false)

	out := n.Step(Event{Kind: EventMessageArrival, From: 1, Message: &Message{Kind: MsgPrepare, Command: []byte("txn")}})

	if n.ParticipantState != ParticipantPrepared {
		t.Fatalf("expected ParticipantState Prepared, got %v", n.ParticipantState)
	}

	found := false
	for _, ob := range out {
		if ob.Kind == OutSendMessage {
			found = true
			if ob.Message.Kind != MsgVoteReply || ob.Message.Vote != VoteCommit {
				t.Fatalf("expected a VoteCommit reply, got %+v", ob.Message)
			}
		}
	}
	if !found {
		t.Fatalf("expected a vote reply to be sent")
	}
}

func TestHandlePrepare_AlwaysAbortParticipantVotesAbortAndResolvesImmediately(t *testing.T) {
	n := NewParticipant(2, 1, true)

	out := n.Step(Event{Kind: EventMessageArrival, From: 1, Message: &Message{Kind: MsgPrepare, Command: []byte("txn")}})

	if n.ParticipantState != ParticipantAborted {
		t.Fatalf("expected ParticipantState Aborted immediately (no need to wait for the coordinator's decision), got %v", n.ParticipantState)
	}

	for _, ob := range out {
		if ob.Kind == OutSendMessage && ob.Message.Vote != VoteAbort {
			t.Fatalf("expected a VoteAbort reply, got %+v", ob.Message)
		}
	}
}

func TestHandleVoteReply_AllCommitVotesDecideCommit(t *testing.T) {
	n := NewCoordinator(1, []int{2, 3})
	n.Step(Event{Kind: EventClientRequest, Command: []byte("txn")})

	out := n.Step(Event{Kind: EventMessageArrival, From: 2, Message: &Message{Kind: MsgVoteReply, Vote: VoteCommit}})
	if n.CoordinatorState == Decided {
		t.Fatalf("expected still waiting after only 1 of 2 votes, got Decided")
	}
	if out != nil {
		t.Fatalf("expected nil Outbound while still waiting on votes, got %v", out)
	}

	out = n.Step(Event{Kind: EventMessageArrival, From: 3, Message: &Message{Kind: MsgVoteReply, Vote: VoteCommit}})
	if n.CoordinatorState != Decided || n.Decision != DecisionCommit {
		t.Fatalf("expected Decided/DecisionCommit after all votes in, got state=%v decision=%v", n.CoordinatorState, n.Decision)
	}

	sentTo := map[int]bool{}
	for _, ob := range out {
		if ob.Kind == OutSendMessage {
			if ob.Message.Kind != MsgDecision || ob.Message.Decision != DecisionCommit {
				t.Fatalf("expected a Commit decision sent, got %+v", ob.Message)
			}
			sentTo[ob.To] = true
		}
	}
	for _, peer := range []int{2, 3} {
		if !sentTo[peer] {
			t.Fatalf("expected the decision sent to peer %d", peer)
		}
	}
}

func TestHandleVoteReply_AnyAbortVoteDecidesAbortImmediately(t *testing.T) {
	n := NewCoordinator(1, []int{2, 3})
	n.Step(Event{Kind: EventClientRequest, Command: []byte("txn")})

	out := n.Step(Event{Kind: EventMessageArrival, From: 2, Message: &Message{Kind: MsgVoteReply, Vote: VoteAbort}})

	if n.CoordinatorState != Decided || n.Decision != DecisionAbort {
		t.Fatalf("expected immediate Decided/DecisionAbort on a single Abort vote, got state=%v decision=%v", n.CoordinatorState, n.Decision)
	}
	found := false
	for _, ob := range out {
		if ob.Kind == OutSendMessage && ob.Message.Decision == DecisionAbort {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an Abort decision broadcast without waiting for peer 3")
	}
}

func TestHandleDecision_ParticipantAppliesCommit(t *testing.T) {
	n := NewParticipant(2, 1, false)
	n.Step(Event{Kind: EventMessageArrival, From: 1, Message: &Message{Kind: MsgPrepare, Command: []byte("txn")}})

	out := n.Step(Event{Kind: EventMessageArrival, From: 1, Message: &Message{Kind: MsgDecision, Decision: DecisionCommit}})

	if n.ParticipantState != ParticipantCommitted {
		t.Fatalf("expected ParticipantState Committed, got %v", n.ParticipantState)
	}
	found := false
	for _, ob := range out {
		if ob.Kind == OutApply {
			found = true
			if !ob.ApplyCommitted || string(ob.ApplyCommand) != "txn" {
				t.Fatalf("expected OutApply{Committed:true, Command:txn}, got %+v", ob)
			}
		}
	}
	if !found {
		t.Fatalf("expected an OutApply outbound")
	}
}

func TestHandleDecision_ParticipantRollsBackOnAbort(t *testing.T) {
	n := NewParticipant(2, 1, false)
	n.Step(Event{Kind: EventMessageArrival, From: 1, Message: &Message{Kind: MsgPrepare, Command: []byte("txn")}})

	n.Step(Event{Kind: EventMessageArrival, From: 1, Message: &Message{Kind: MsgDecision, Decision: DecisionAbort}})

	if n.ParticipantState != ParticipantAborted {
		t.Fatalf("expected ParticipantState Aborted, got %v", n.ParticipantState)
	}
}

func TestHandlePrepareTimeout_CoordinatorAbortsIfVotesMissing(t *testing.T) {
	n := NewCoordinator(1, []int{2, 3})
	n.Step(Event{Kind: EventClientRequest, Command: []byte("txn")})
	n.Step(Event{Kind: EventMessageArrival, From: 2, Message: &Message{Kind: MsgVoteReply, Vote: VoteCommit}})

	out := n.Step(Event{Kind: EventTimerFire, TimerKindField: TimerPrepare, TimerGen: n.TimerGeneration(TimerPrepare)})

	if n.CoordinatorState != Decided || n.Decision != DecisionAbort {
		t.Fatalf("expected Decided/DecisionAbort on prepare timeout with a missing vote, got state=%v decision=%v", n.CoordinatorState, n.Decision)
	}
	if out == nil {
		t.Fatalf("expected outbound abort broadcast on timeout")
	}
}

func TestHandlePrepareTimeout_StaleGenerationIgnored(t *testing.T) {
	n := NewCoordinator(1, []int{2, 3})
	n.Step(Event{Kind: EventClientRequest, Command: []byte("txn")})

	stale := Event{Kind: EventTimerFire, TimerKindField: TimerPrepare, TimerGen: 999}
	out := n.Step(stale)

	if out != nil {
		t.Fatalf("expected nil Outbound for a stale timer generation, got %v", out)
	}
	if n.CoordinatorState != WaitingForVotes {
		t.Fatalf("expected CoordinatorState untouched by a stale timer, got %v", n.CoordinatorState)
	}
}
