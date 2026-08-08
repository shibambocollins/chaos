package raft

import "math/rand"

func (n *NodeState) handleMessage(from int, msg *RaftMessage, rng *rand.Rand) []Outbound {
	// Universal term check (§7, step 1) — applies before any kind-specific
	// logic, for every message kind, including a stale leader's own
	// heartbeat. Seeing a higher term always means stepping down. This is
	// itself a persistent-state change, so it must be persisted even if
	// the kind-specific handling below doesn't grant/accept anything.
	steppedDown := false
	if msg.Term > n.CurrentTerm {
		n.CurrentTerm = msg.Term
		n.Role = Follower
		n.VotedFor = -1
		steppedDown = true
	}

	switch msg.Kind {
	case MsgRequestVote:
		return n.handleRequestVote(from, msg, rng, steppedDown)
	}

	// Remaining message kinds (RequestVoteReply, AppendEntries,
	// AppendEntriesReply) are later sub-modules of Module 4.
	return nil
}

// handleRequestVote implements the election restriction (Gotcha #1): a
// voter must reject a candidate whose log is less up-to-date than its
// own — compare LastLogTerm first, then LastLogIndex as a tiebreak.
// Skipping this can elect a leader missing entries the rest of the
// cluster already agreed on, a direct safety violation.
//
// mustPersist is true when handleMessage already changed CurrentTerm for
// this message (a step-down) — that change needs OutPersist regardless of
// whether the vote itself ends up granted.
func (n *NodeState) handleRequestVote(from int, msg *RaftMessage, rng *rand.Rand, mustPersist bool) []Outbound {
	lastIndex, lastTerm := n.lastLogIndexAndTerm()

	logIsUpToDate := msg.LastLogTerm > lastTerm ||
		(msg.LastLogTerm == lastTerm && msg.LastLogIndex >= lastIndex)

	alreadyVotedForOther := n.VotedFor != -1 && n.VotedFor != msg.CandidateID

	grant := msg.Term == n.CurrentTerm && logIsUpToDate && !alreadyVotedForOther

	if grant {
		n.VotedFor = msg.CandidateID
		mustPersist = true
	}

	var out []Outbound

	// OutPersist first, always, whenever CurrentTerm or VotedFor changed —
	// before the OutSendMessage below that depends on it.
	if mustPersist {
		out = append(out, Outbound{
			Kind:              OutPersist,
			PersistedTerm:     n.CurrentTerm,
			PersistedVotedFor: n.VotedFor,
			PersistedLogLen:   len(n.Log),
		})
	}

	if grant {
		n.timerGen[TimerElection]++
		out = append(out, Outbound{
			Kind:           OutResetTimer,
			TimerKindField: TimerElection,
			Duration:       sampleElectionTimeout(rng),
		})
	}

	out = append(out, Outbound{
		Kind: OutSendMessage,
		To:   from,
		Message: &RaftMessage{
			Kind:        MsgRequestVoteReply,
			Term:        n.CurrentTerm,
			VoteGranted: grant,
		},
	})
	return out
}
