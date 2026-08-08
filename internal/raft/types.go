package raft

// Time is simulated logical time — never wall-clock. Advanced only by the
// simulator's event loop.
type Time uint64

// ---------- Raft message payloads ----------

type MessageKind int

const (
	MsgRequestVote MessageKind = iota
	MsgRequestVoteReply
	MsgAppendEntries
	MsgAppendEntriesReply
)

type LogEntry struct {
	Term    uint64
	Index   uint64
	Command []byte // opaque application payload — Chaos doesn't need to interpret it
}

type RaftMessage struct {
	Kind MessageKind
	Term uint64 // present on every message — lets a handler check "is this stale relative to my term?" uniformly, before even looking at the specific payload

	// RequestVote
	CandidateID  int
	LastLogIndex uint64
	LastLogTerm  uint64

	// RequestVoteReply
	VoteGranted bool

	// AppendEntries
	LeaderID     int
	PrevLogIndex uint64
	PrevLogTerm  uint64
	Entries      []LogEntry
	LeaderCommit uint64

	// AppendEntriesReply
	Success bool
	// Fast backtracking — OPTIONAL, an efficiency optimization, not a
	// correctness requirement. Implement only after the naive
	// one-entry-at-a-time nextIndex backoff already works and is tested.
	ConflictIndex uint64
	ConflictTerm  uint64
}

// ---------- Events (input to a node's Step) ----------

type EventKind int

const (
	EventMessageArrival EventKind = iota
	EventTimerFire
)

type TimerKind int

const (
	TimerElection  TimerKind = iota // followers & candidates: "no leader heard from in time"
	TimerHeartbeat                  // leaders only: "time to prove I'm still alive"
)

type Event struct {
	At     Time
	Seq    uint64 // deterministic tiebreak for equal At — assigned by the simulator at scheduling time
	NodeID int    // which node's Step() receives this event

	Kind EventKind

	// EventMessageArrival
	From    int
	Message *RaftMessage

	// EventTimerFire
	TimerKindField TimerKind
	TimerGen       uint64 // must match the node's current generation for this timer kind, or it's stale
}

// ---------- Outbound (what a node's Step wants the simulator to do) ----------

type OutboundKind int

const (
	OutSendMessage OutboundKind = iota
	OutResetTimer
	OutPersist
	OutApply
)

type Outbound struct {
	Kind OutboundKind

	// OutSendMessage
	To      int
	Message *RaftMessage

	// OutResetTimer — simulator adds Duration to "now" and assigns the
	// resulting event the node's NEW generation for that timer kind.
	TimerKindField TimerKind
	Duration       Time

	// OutPersist — signals that CurrentTerm/VotedFor/Log changed and MUST
	// be durably saved before any OutSendMessage in this same batch is
	// actually delivered. Modeled as an ordering guarantee the simulator
	// enforces, not real disk I/O.
	PersistedTerm     uint64
	PersistedVotedFor int
	PersistedLogLen   int

	// OutApply — entry has passed CommitIndex; hand it to the state machine.
	ApplyIndex   uint64
	ApplyCommand []byte
}
