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
	case MsgRequestVoteReply:
		return n.handleRequestVoteReply(from, msg, steppedDown)
	}

	// Remaining message kinds (AppendEntries, AppendEntriesReply) are
	// later sub-modules of Module 4.
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

// handleRequestVoteReply implements §7's MsgRequestVoteReply contract:
// ignore stale replies (no longer a Candidate, or the reply is for a term
// this node isn't campaigning for), otherwise count granted votes and
// become Leader on reaching a majority.
//
// mustPersist is true when handleMessage just stepped this node down
// because of this exact reply's term — that's always a stale reply too
// (stepping down set Role to Follower), so the only remaining obligation
// is persisting the new term before discarding it.
func (n *NodeState) handleRequestVoteReply(from int, msg *RaftMessage, mustPersist bool) []Outbound {
	if n.Role != Candidate || msg.Term != n.CurrentTerm {
		if mustPersist {
			return []Outbound{{
				Kind:              OutPersist,
				PersistedTerm:     n.CurrentTerm,
				PersistedVotedFor: n.VotedFor,
				PersistedLogLen:   len(n.Log),
			}}
		}
		return nil
	}

	if !msg.VoteGranted {
		return nil
	}

	n.VotesReceived[from] = true
	if len(n.VotesReceived) < n.majority() {
		return nil
	}

	return n.becomeLeader()
}

// majority returns the number of votes needed to win an election, or
// (later, Module 4c) the number of matching replicas needed to commit an
// entry — half the cluster including self, plus one.
func (n *NodeState) majority() int {
	return (len(n.Peers)+1)/2 + 1
}

// becomeLeader transitions a Candidate that just won an election. It
// initializes per-follower replication state, stops the election timer —
// a leader never runs one, and bumping the generation without scheduling
// a replacement invalidates any already-pending EventTimerFire so Step()
// drops it as stale — and immediately sends heartbeats per §7 by reusing
// the same construction the periodic heartbeat timer uses.
func (n *NodeState) becomeLeader() []Outbound {
	n.Role = Leader

	lastIndex, _ := n.lastLogIndexAndTerm()
	n.NextIndex = make(map[int]uint64, len(n.Peers))
	n.MatchIndex = make(map[int]uint64, len(n.Peers))
	for _, peer := range n.Peers {
		n.NextIndex[peer] = lastIndex + 1
		n.MatchIndex[peer] = 0
	}
	n.VotesReceived = nil

	n.timerGen[TimerElection]++

	return n.handleHeartbeatTimeout()
}
