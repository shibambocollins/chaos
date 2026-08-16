package raft

// Time is simulated logical time — never wall-clock. Advanced only by the
// simulator's event loop.
type Time uint64

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
	Command []byte
}

type RaftMessage struct {
	Kind MessageKind
	Term uint64

	CandidateID  int
	LastLogIndex uint64
	LastLogTerm  uint64

	VoteGranted bool

	LeaderID     int
	PrevLogIndex uint64
	PrevLogTerm  uint64
	Entries      []LogEntry
	LeaderCommit uint64

	Success bool

	MatchIndex uint64

	ConflictIndex uint64
	ConflictTerm  uint64
}

type EventKind int

const (
	EventMessageArrival EventKind = iota
	EventTimerFire
	EventClientRequest
)

type TimerKind int

const (
	TimerElection TimerKind = iota
	TimerHeartbeat
)

type Event struct {
	At     Time
	Seq    uint64
	NodeID int

	Kind EventKind

	From    int
	Message *RaftMessage

	TimerKindField TimerKind
	TimerGen       uint64

	Command []byte
}

type OutboundKind int

const (
	OutSendMessage OutboundKind = iota
	OutResetTimer
	OutPersist
	OutApply
)

type Outbound struct {
	Kind OutboundKind

	To      int
	Message *RaftMessage

	TimerKindField TimerKind
	Duration       Time

	PersistedTerm     uint64
	PersistedVotedFor int
	PersistedLogLen   int

	ApplyIndex   uint64
	ApplyCommand []byte
}
