package twopc

// handleClientRequest starts a new transaction. Coordinator only — a
// Participant never initiates one.
func (n *NodeState) handleClientRequest(cmd []byte) []Outbound {
	if n.Role != Coordinator || n.CoordinatorState != CoordinatorIdle {
		return nil // not a coordinator, or one already mid-transaction — ignore
	}

	n.CoordinatorState = WaitingForVotes
	n.Command = cmd
	n.VotesReceived = make(map[int]Vote, len(n.Peers))

	n.timerGen[TimerPrepare]++

	out := []Outbound{n.persistOutbound(), {
		Kind: OutResetTimer, TimerKindField: TimerPrepare, Duration: prepareTimeout,
	}}
	for _, peer := range n.Peers {
		out = append(out, Outbound{
			Kind: OutSendMessage, To: peer,
			Message: &Message{Kind: MsgPrepare, Command: cmd},
		})
	}
	return out
}

func (n *NodeState) handleMessage(from int, msg *Message) []Outbound {
	switch msg.Kind {
	case MsgPrepare:
		return n.handlePrepare(from, msg)
	case MsgVoteReply:
		return n.handleVoteReply(from, msg)
	case MsgDecision:
		return n.handleDecision(msg)
	}
	return nil
}

// handlePrepare is a Participant's Phase 1: decide a vote, persist it —
// the vote itself, not just the reply, must be durable, so a restart can
// recover the promise this node is about to make — then reply. Voting
// Commit moves this node to Prepared, blocked until a Decision arrives.
// Voting Abort resolves immediately: an Abort vote guarantees the global
// outcome will be Abort, so there is nothing to wait for.
func (n *NodeState) handlePrepare(from int, msg *Message) []Outbound {
	if n.Role != Participant || n.ParticipantState != ParticipantIdle {
		return nil // not a participant, or already mid-transaction — ignore (no txn IDs in this simplified model)
	}

	n.Command = msg.Command

	vote := VoteCommit
	if n.AlwaysAbort {
		vote = VoteAbort
	}

	if vote == VoteCommit {
		n.ParticipantState = ParticipantPrepared
	} else {
		n.ParticipantState = ParticipantAborted
	}

	return []Outbound{
		n.persistOutbound(),
		{Kind: OutSendMessage, To: from, Message: &Message{Kind: MsgVoteReply, Vote: vote}},
	}
}

// handleVoteReply is the coordinator's Phase 1 tally. Any single Abort
// vote decides the whole transaction immediately — no need to wait for the
// rest. Only once every participant has voted Commit does the coordinator
// decide Commit.
func (n *NodeState) handleVoteReply(from int, msg *Message) []Outbound {
	if n.Role != Coordinator || n.CoordinatorState != WaitingForVotes {
		return nil // stale — already decided, or not currently collecting votes
	}

	n.VotesReceived[from] = msg.Vote

	if msg.Vote == VoteAbort {
		return n.decide(DecisionAbort)
	}
	if len(n.VotesReceived) < len(n.Peers) {
		return nil // still waiting on the rest
	}
	return n.decide(DecisionCommit)
}

// decide is the coordinator's Phase 2: persist the decision, then
// broadcast it. Persist-before-send applies here exactly as it does in
// Raft — a crash between deciding and durably recording that decision
// would leave a future Restart unable to tell what was actually decided.
func (n *NodeState) decide(d Decision) []Outbound {
	n.CoordinatorState = Decided
	n.Decision = d
	n.timerGen[TimerPrepare]++ // cancel the prepare timeout — no longer waiting

	out := []Outbound{n.persistOutbound()}
	for _, peer := range n.Peers {
		out = append(out, Outbound{
			Kind: OutSendMessage, To: peer,
			Message: &Message{Kind: MsgDecision, Decision: d},
		})
	}
	return out
}

// handleDecision is a Participant's Phase 2: apply or roll back per the
// coordinator's decision, and persist the outcome. A Participant that
// already resolved itself (it voted Abort and settled immediately) treats
// a subsequent Decision as a no-op — it's redundant, not new information.
func (n *NodeState) handleDecision(msg *Message) []Outbound {
	if n.Role != Participant {
		return nil
	}
	if n.ParticipantState == ParticipantCommitted || n.ParticipantState == ParticipantAborted {
		return nil
	}

	committed := msg.Decision == DecisionCommit
	if committed {
		n.ParticipantState = ParticipantCommitted
	} else {
		n.ParticipantState = ParticipantAborted
	}

	return []Outbound{
		n.persistOutbound(),
		{Kind: OutApply, ApplyCommitted: committed, ApplyCommand: n.Command},
	}
}

func (n *NodeState) handleTimeout(kind TimerKind) []Outbound {
	switch kind {
	case TimerPrepare:
		return n.handlePrepareTimeout()
	}
	return nil
}

// handlePrepareTimeout is the one safe timeout in this entire package: if
// the coordinator hasn't heard from everyone yet, it can still abort
// unilaterally, because by construction nobody has been told Commit. This
// option does not exist for a Participant once it has voted Commit — see
// the package doc for why.
func (n *NodeState) handlePrepareTimeout() []Outbound {
	if n.Role != Coordinator || n.CoordinatorState != WaitingForVotes {
		return nil // already decided, or stale
	}
	return n.decide(DecisionAbort)
}
