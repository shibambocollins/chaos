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

// ---------- Messages ----------

type MessageKind int

const (
	MsgPrepare   MessageKind = iota // Coordinator -> Participant: Phase 1 begin
	MsgVoteReply                    // Participant -> Coordinator: this participant's vote
	MsgDecision                     // Coordinator -> Participant: Phase 2, the final outcome
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

	Command []byte // MsgPrepare: the transaction's payload

	Vote Vote // MsgVoteReply

	Decision Decision // MsgDecision
}

// ---------- Node roles and per-role state machines ----------

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
	ParticipantIdle     ParticipantState = iota // no transaction in flight
	ParticipantPrepared                         // voted Commit; blocked until a Decision arrives
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

// ---------- NodeState ----------

// NodeState is a single 2PC node's complete state — either a Coordinator or
// a Participant, per Role. Fields are grouped by the persistent/volatile
// split, same discipline as internal/raft.NodeState: everything under
// "Persistent" must be durably saved (OutPersist) before any Outbound that
// depends on it is sent, and must come back unchanged across a Restart.
type NodeState struct {
	ID    int
	Peers []int // sorted, excludes ID — Coordinator's Peers are the participants; a Participant's Peers is just [coordinatorID]

	Role Role

	// AlwaysAbort is a Participant-only test/demo knob, not part of the
	// protocol itself: forces this participant to vote Abort on every
	// transaction, for exercising the abort path deliberately.
	AlwaysAbort bool

	// Persistent state.
	ParticipantState ParticipantState // Participant only
	CoordinatorState CoordinatorState // Coordinator only
	Decision         Decision         // Coordinator only; meaningful once CoordinatorState == Decided
	Command          []byte           // the transaction's payload, once known to this node

	// Volatile state — Coordinator only. Never consulted after a Restart:
	// a coordinator that comes back mid-vote-collection has lost this and
	// must fall back to the safe default (abort), not try to resume
	// collecting votes from where it left off.
	VotesReceived map[int]Vote

	// timerGen holds the current generation for each TimerKind — same
	// stale-timer-cancellation discipline as internal/raft.
	timerGen [1]uint64
}

// ---------- Events (input to a node's Step) ----------

type EventKind int

const (
	EventMessageArrival EventKind = iota
	EventTimerFire
	EventClientRequest // Coordinator only: begin a new transaction
)

type TimerKind int

const (
	// TimerPrepare is the coordinator-only "haven't heard from everyone in
	// time" timeout. There is no participant-side equivalent — that
	// absence is the entire point of this package.
	TimerPrepare TimerKind = iota
)

type Event struct {
	At     Time
	Seq    uint64 // deterministic tiebreak for equal At, assigned by the simulator at scheduling time
	NodeID int

	Kind EventKind

	// EventMessageArrival
	From    int
	Message *Message

	// EventTimerFire
	TimerKindField TimerKind
	TimerGen       uint64

	// EventClientRequest
	Command []byte
}

// ---------- Outbound (what a node's Step wants the simulator to do) ----------

type OutboundKind int

const (
	OutSendMessage OutboundKind = iota
	OutResetTimer
	OutPersist
	OutApply // signals this node's final local outcome for the transaction
)

type Outbound struct {
	Kind OutboundKind

	// OutSendMessage
	To      int
	Message *Message

	// OutResetTimer
	TimerKindField TimerKind
	Duration       Time

	// OutPersist — mirrors the node's persistent fields exactly, after
	// whatever change this batch made.
	PersistedParticipantState ParticipantState
	PersistedCoordinatorState CoordinatorState
	PersistedDecision         Decision
	PersistedCommand          []byte

	// OutApply
	ApplyCommitted bool
	ApplyCommand   []byte
}
