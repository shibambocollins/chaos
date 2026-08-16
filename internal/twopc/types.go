// Package twopc is a from-scratch, deliberately simple Two-Phase Commit
// implementation, standalone from internal/raft and internal/sim on
// purpose: it exists to demonstrate one specific contrast with Raft — a
// participant that has voted to commit cannot safely resolve itself if the
// coordinator disappears, so it blocks for as long as the coordinator is
// unavailable, however long that is. Raft has no equivalent single point
// of indefinite unavailability; a crashed leader gets replaced.
package twopc

// Time is simulated logical time — never wall-clock. This package's own
// type, not shared with internal/raft: twopc is intentionally standalone.
type Time uint64

type MessageKind int

const (
	MsgPrepare MessageKind = iota
	MsgVoteReply
	MsgDecision
)

type Vote int

const (
	VoteCommit Vote = iota
	VoteAbort
)

func (v Vote) String() string {
	switch v {
	case VoteCommit:
		return "VoteCommit"
	case VoteAbort:
		return "VoteAbort"
	default:
		return "Unknown"
	}
}

// Decision is the coordinator's outcome for the current transaction.
// DecisionPending means no decision has been made yet.
type Decision int

const (
	DecisionPending Decision = iota
	DecisionCommit
	DecisionAbort
)

func (d Decision) String() string {
	switch d {
	case DecisionPending:
		return "DecisionPending"
	case DecisionCommit:
		return "DecisionCommit"
	case DecisionAbort:
		return "DecisionAbort"
	default:
		return "Unknown"
	}
}

type Message struct {
	Kind MessageKind

	Command []byte

	Vote Vote

	Decision Decision
}

type Role int

const (
	Participant Role = iota
	Coordinator
)

// ParticipantState is a participant's own persistent transaction state.
// Once it reaches Prepared, that value MUST survive a Restart unchanged —
// a participant that forgot it had voted Commit could default to Abort (or
// Commit) on its own after restarting, which is exactly the safety
// violation 2PC exists to avoid. See the package doc.
type ParticipantState int

const (
	ParticipantIdle ParticipantState = iota
	ParticipantPrepared
	ParticipantCommitted
	ParticipantAborted
)

func (p ParticipantState) String() string {
	switch p {
	case ParticipantIdle:
		return "Idle"
	case ParticipantPrepared:
		return "Prepared"
	case ParticipantCommitted:
		return "Committed"
	case ParticipantAborted:
		return "Aborted"
	default:
		return "Unknown"
	}
}

// CoordinatorState is the coordinator's own persistent transaction state.
// WaitingForVotes with no persisted Decision is the one state a Restart is
// allowed to resolve unilaterally (default to Abort — safe, since nothing
// has been told Commit yet). Decided must be replayed exactly as
// persisted, never re-decided.
type CoordinatorState int

const (
	CoordinatorIdle CoordinatorState = iota
	WaitingForVotes
	Decided
)

func (c CoordinatorState) String() string {
	switch c {
	case CoordinatorIdle:
		return "CoordinatorIdle"
	case WaitingForVotes:
		return "WaitingForVotes"
	case Decided:
		return "Decided"
	default:
		return "Unknown"
	}
}

// NodeState is a single 2PC node's complete state — either a Coordinator or
// a Participant, per Role. Fields are grouped by the persistent/volatile
// split, same discipline as internal/raft.NodeState: everything under
// "Persistent" must be durably saved (OutPersist) before any Outbound that
// depends on it is sent, and must come back unchanged across a Restart.
type NodeState struct {
	ID    int
	Peers []int

	Role Role

	AlwaysAbort bool

	ParticipantState ParticipantState
	CoordinatorState CoordinatorState
	Decision         Decision
	Command          []byte

	VotesReceived map[int]Vote

	timerGen [1]uint64
}

type EventKind int

const (
	EventMessageArrival EventKind = iota
	EventTimerFire
	EventClientRequest
)

type TimerKind int

const (
	TimerPrepare TimerKind = iota
)

type Event struct {
	At     Time
	Seq    uint64
	NodeID int

	Kind EventKind

	From    int
	Message *Message

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
	Message *Message

	TimerKindField TimerKind
	Duration       Time

	PersistedParticipantState ParticipantState
	PersistedCoordinatorState CoordinatorState
	PersistedDecision         Decision
	PersistedCommand          []byte

	ApplyCommitted bool
	ApplyCommand   []byte
}
