package twopc

import "testing"

// TestRestart_PreparedParticipantSurvivesUnchanged is the correctness point
// this whole package exists to demonstrate. A participant that voted
// Commit must come back exactly as Prepared, still blocked — restart must
// never let it default to Committed or Aborted on its own.
func TestRestart_PreparedParticipantSurvivesUnchanged(t *testing.T) {
	n := NewParticipant(2, 1, false)
	n.Step(Event{Kind: EventMessageArrival, From: 1, Message: &Message{Kind: MsgPrepare, Command: []byte("txn")}})

	if n.ParticipantState != ParticipantPrepared {
		t.Fatalf("setup: expected Prepared before restart, got %v", n.ParticipantState)
	}

	out := n.Restart()

	if n.ParticipantState != ParticipantPrepared {
		t.Fatalf("expected ParticipantState to survive Restart as Prepared, got %v", n.ParticipantState)
	}
	if string(n.Command) != "txn" {
		t.Fatalf("expected Command to survive Restart, got %q", n.Command)
	}
	if out != nil {
		t.Fatalf("expected a restarted Participant to send nothing on its own — it can only wait, got %v", out)
	}
}

// TestRestart_CoordinatorReplaysPersistedDecision covers the case that
// actually unblocks a waiting participant: a coordinator that crashed
// after persisting its decision (but possibly before every Phase 2 message
// went out) must resend that exact decision on restart, not re-decide.
func TestRestart_CoordinatorReplaysPersistedDecision(t *testing.T) {
	n := NewCoordinator(1, []int{2, 3})
	n.Step(Event{Kind: EventClientRequest, Command: []byte("txn")})
	n.Step(Event{Kind: EventMessageArrival, From: 2, Message: &Message{Kind: MsgVoteReply, Vote: VoteCommit}})
	n.Step(Event{Kind: EventMessageArrival, From: 3, Message: &Message{Kind: MsgVoteReply, Vote: VoteCommit}})

	if n.CoordinatorState != Decided || n.Decision != DecisionCommit {
		t.Fatalf("setup: expected Decided/DecisionCommit before restart, got state=%v decision=%v", n.CoordinatorState, n.Decision)
	}

	out := n.Restart()

	if n.CoordinatorState != Decided || n.Decision != DecisionCommit {
		t.Fatalf("expected the persisted decision to survive Restart unchanged, got state=%v decision=%v", n.CoordinatorState, n.Decision)
	}

	sentTo := map[int]bool{}
	for _, ob := range out {
		if ob.Kind == OutSendMessage {
			if ob.Message.Kind != MsgDecision || ob.Message.Decision != DecisionCommit {
				t.Fatalf("expected the replayed message to be the original Commit decision, got %+v", ob.Message)
			}
			sentTo[ob.To] = true
		}
	}
	for _, peer := range []int{2, 3} {
		if !sentTo[peer] {
			t.Fatalf("expected the replayed decision resent to peer %d", peer)
		}
	}
}

// TestRestart_CoordinatorMidVoteCollectionDefaultsToAbort covers the other
// branch: a coordinator that crashed before deciding anything has lost its
// in-memory vote tally, and the only safe move is to abort — nothing has
// ever been told Commit, so aborting now can't contradict a promise
// already made.
func TestRestart_CoordinatorMidVoteCollectionDefaultsToAbort(t *testing.T) {
	n := NewCoordinator(1, []int{2, 3})
	n.Step(Event{Kind: EventClientRequest, Command: []byte("txn")})
	n.Step(Event{Kind: EventMessageArrival, From: 2, Message: &Message{Kind: MsgVoteReply, Vote: VoteCommit}})

	if n.CoordinatorState != WaitingForVotes {
		t.Fatalf("setup: expected still WaitingForVotes before restart (only 1 of 2 votes in), got %v", n.CoordinatorState)
	}

	out := n.Restart()

	if n.CoordinatorState != Decided || n.Decision != DecisionAbort {
		t.Fatalf("expected Restart to default to Decided/DecisionAbort, got state=%v decision=%v", n.CoordinatorState, n.Decision)
	}
	found := false
	for _, ob := range out {
		if ob.Kind == OutSendMessage && ob.Message.Decision == DecisionAbort {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the default Abort decision to be broadcast on restart")
	}
}

func TestRestart_IdleCoordinatorSendsNothing(t *testing.T) {
	n := NewCoordinator(1, []int{2, 3})

	out := n.Restart()

	if out != nil {
		t.Fatalf("expected nil Outbound for a restart with no transaction in flight, got %v", out)
	}
}
