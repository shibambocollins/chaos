package raft

import "math/rand"

const (
	// electionTimeoutMin/Max bound the randomized follower/candidate
	// election timeout, in simulated ticks. Randomization is load-bearing
	// (see context doc §3) — without it every node times out
	// simultaneously, every node becomes a candidate simultaneously, and
	// elections split forever.
	electionTimeoutMin Time = 15
	electionTimeoutMax Time = 30

	// heartbeatInterval is fixed and well below electionTimeoutMin so a
	// live leader's heartbeats always beat followers' election timers.
	heartbeatInterval Time = 5
)

// sampleElectionTimeout draws a fresh random election timeout within
// [electionTimeoutMin, electionTimeoutMax]. Every legitimate reset of the
// election timer — whether from the timer itself firing or from granting
// a vote — must resample rather than reuse a fixed value, or nodes drift
// back toward synchronized timeouts over time.
func sampleElectionTimeout(rng *rand.Rand) Time {
	return electionTimeoutMin + Time(rng.Int63n(int64(electionTimeoutMax-electionTimeoutMin+1)))
}

func (n *NodeState) handleTimeout(kind TimerKind, rng *rand.Rand) []Outbound {
	switch kind {
	case TimerElection:
		return n.handleElectionTimeout(rng)
	case TimerHeartbeat:
		return n.handleHeartbeatTimeout()
	}
	return nil
}

// handleElectionTimeout implements §7's TimerElection contract: only
// meaningful for a Follower or Candidate. Starts a new election.
func (n *NodeState) handleElectionTimeout(rng *rand.Rand) []Outbound {
	if n.Role == Leader {
		return nil // leaders don't run this timer; ignore defensively
	}

	n.CurrentTerm++
	n.VotedFor = n.ID
	n.Role = Candidate
	n.VotesReceived = map[int]bool{n.ID: true}

	n.timerGen[TimerElection]++
	timeout := sampleElectionTimeout(rng)

	// OutPersist first — CurrentTerm/VotedFor changed and must be durable
	// before the RequestVote sends below are allowed to go out.
	out := []Outbound{
		{
			Kind:              OutPersist,
			PersistedTerm:     n.CurrentTerm,
			PersistedVotedFor: n.VotedFor,
			PersistedLogLen:   len(n.Log),
		},
		{
			Kind:           OutResetTimer,
			TimerKindField: TimerElection,
			Duration:       timeout,
		},
	}

	lastIndex, lastTerm := n.lastLogIndexAndTerm()
	for _, peer := range n.Peers {
		out = append(out, Outbound{
			Kind: OutSendMessage,
			To:   peer,
			Message: &RaftMessage{
				Kind:         MsgRequestVote,
				Term:         n.CurrentTerm,
				CandidateID:  n.ID,
				LastLogIndex: lastIndex,
				LastLogTerm:  lastTerm,
			},
		})
	}
	return out
}

// handleHeartbeatTimeout implements §7's TimerHeartbeat contract: only
// meaningful for a Leader. Sends AppendEntries (empty here — replication
// catch-up on failure is Module 4's concern) to every peer.
func (n *NodeState) handleHeartbeatTimeout() []Outbound {
	if n.Role != Leader {
		return nil // only leaders run this timer; ignore defensively
	}

	n.timerGen[TimerHeartbeat]++

	out := []Outbound{
		{
			Kind:           OutResetTimer,
			TimerKindField: TimerHeartbeat,
			Duration:       heartbeatInterval,
		},
	}

	for _, peer := range n.Peers {
		out = append(out, n.replicateTo(peer))
	}
	return out
}

// replicateTo builds the AppendEntries message peer should receive right
// now: PrevLogIndex/PrevLogTerm anchor the log matching check, and Entries
// carries everything from there to the leader's log tip — i.e. everything
// the leader believes peer is still missing, per NextIndex[peer]. Shared by
// the periodic heartbeat and by a client request that needs to push a new
// entry out immediately rather than waiting for the next heartbeat tick.
func (n *NodeState) replicateTo(peer int) Outbound {
	prevIndex, prevTerm := n.prevLogFor(peer)

	var entries []LogEntry
	if prevIndex < uint64(len(n.Log)) {
		entries = append(entries, n.Log[prevIndex:]...)
	}

	return Outbound{
		Kind: OutSendMessage,
		To:   peer,
		Message: &RaftMessage{
			Kind:         MsgAppendEntries,
			Term:         n.CurrentTerm,
			LeaderID:     n.ID,
			PrevLogIndex: prevIndex,
			PrevLogTerm:  prevTerm,
			Entries:      entries,
			LeaderCommit: n.CommitIndex,
		},
	}
}

// lastLogIndexAndTerm returns the index and term of the last entry in the
// node's log, or (0, 0) if the log is empty.
func (n *NodeState) lastLogIndexAndTerm() (uint64, uint64) {
	if len(n.Log) == 0 {
		return 0, 0
	}
	last := n.Log[len(n.Log)-1]
	return last.Index, last.Term
}

// prevLogFor returns the PrevLogIndex/PrevLogTerm a message to peer should
// carry, based on where the leader believes that follower's log currently
// stands (NextIndex). Falls back to the leader's own log tip if NextIndex
// hasn't been initialized for peer yet (e.g. a node driven directly by a
// unit test rather than through the full election flow in Module 4, which
// sets NextIndex on becoming leader).
func (n *NodeState) prevLogFor(peer int) (uint64, uint64) {
	next, ok := n.NextIndex[peer]
	if !ok {
		return n.lastLogIndexAndTerm()
	}
	prevIndex := next - 1
	if prevIndex == 0 || prevIndex > uint64(len(n.Log)) {
		return prevIndex, 0
	}
	return prevIndex, n.Log[prevIndex-1].Term
}
