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
	case MsgAppendEntries:
		return n.handleAppendEntries(from, msg, rng, steppedDown)
	}

	// MsgAppendEntriesReply is the last sub-module of Module 4.
	return nil
}

// persistOutbound builds the OutPersist reflecting the node's current
// CurrentTerm/VotedFor/Log. Always emitted first, before any
// OutSendMessage, whenever one of those three changed.
func (n *NodeState) persistOutbound() Outbound {
	return Outbound{
		Kind:              OutPersist,
		PersistedTerm:     n.CurrentTerm,
		PersistedVotedFor: n.VotedFor,
		PersistedLogLen:   len(n.Log),
	}
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
		out = append(out, n.persistOutbound())
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
			return []Outbound{n.persistOutbound()}
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

// handleAppendEntries implements §7's MsgAppendEntries contract: the log
// matching property, truncate-and-append, and advancing CommitIndex.
//
// mustPersist is true when handleMessage already changed CurrentTerm for
// this message (a step-down) — folded into the single OutPersist emitted
// here if the log also changes, rather than persisting twice.
func (n *NodeState) handleAppendEntries(from int, msg *RaftMessage, rng *rand.Rand, mustPersist bool) []Outbound {
	if msg.Term < n.CurrentTerm {
		// Stale leader — reject without touching timers or the log.
		return []Outbound{{
			Kind: OutSendMessage,
			To:   from,
			Message: &RaftMessage{
				Kind: MsgAppendEntriesReply, Term: n.CurrentTerm, Success: false,
			},
		}}
	}

	// A legitimate leader for this term. Recognize it — this is also how
	// a Candidate concedes an election it didn't win, without needing a
	// strictly higher term to do so.
	n.Role = Follower

	n.timerGen[TimerElection]++
	resetTimer := Outbound{
		Kind:           OutResetTimer,
		TimerKindField: TimerElection,
		Duration:       sampleElectionTimeout(rng),
	}

	logMatches := msg.PrevLogIndex == 0 ||
		(msg.PrevLogIndex <= uint64(len(n.Log)) && n.Log[msg.PrevLogIndex-1].Term == msg.PrevLogTerm)

	if !logMatches {
		var out []Outbound
		if mustPersist {
			out = append(out, n.persistOutbound())
		}
		out = append(out, resetTimer, Outbound{
			Kind: OutSendMessage,
			To:   from,
			Message: &RaftMessage{
				Kind: MsgAppendEntriesReply, Term: n.CurrentTerm, Success: false,
			},
		})
		return out
	}

	logChanged := false
	for i, entry := range msg.Entries {
		idx := msg.PrevLogIndex + uint64(i) + 1
		if idx <= uint64(len(n.Log)) {
			if n.Log[idx-1].Term == entry.Term {
				continue // already present and matches — nothing to do
			}
			n.Log = n.Log[:idx-1] // conflicting suffix — truncate (a follower-only operation; leaders never do this)
			logChanged = true
		}
		n.Log = append(n.Log, entry)
		logChanged = true
	}

	if msg.LeaderCommit > n.CommitIndex {
		lastNewIndex := msg.PrevLogIndex + uint64(len(msg.Entries))
		n.CommitIndex = min(msg.LeaderCommit, lastNewIndex)
	}

	var out []Outbound
	if mustPersist || logChanged {
		out = append(out, n.persistOutbound())
	}
	out = append(out, resetTimer)
	out = append(out, n.applyCommittedEntries()...)
	out = append(out, Outbound{
		Kind: OutSendMessage,
		To:   from,
		Message: &RaftMessage{
			Kind: MsgAppendEntriesReply, Term: n.CurrentTerm, Success: true,
		},
	})
	return out
}

// applyCommittedEntries returns OutApply for every log entry between
// LastApplied and CommitIndex, advancing LastApplied as it goes. Shared
// by both the follower path above and the leader path in Module 4d —
// anywhere CommitIndex can advance.
func (n *NodeState) applyCommittedEntries() []Outbound {
	var out []Outbound
	for n.LastApplied < n.CommitIndex {
		n.LastApplied++
		entry := n.Log[n.LastApplied-1] // indices are 1-based and contiguous
		out = append(out, Outbound{
			Kind:         OutApply,
			ApplyIndex:   entry.Index,
			ApplyCommand: entry.Command,
		})
	}
	return out
}
