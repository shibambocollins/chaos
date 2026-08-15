package raft

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
)

type Role int

const (
	Follower Role = iota
	Candidate
	Leader
)

func (r Role) String() string {
	switch r {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	default:
		return "Unknown"
	}
}

// MarshalJSON renders Role as its String() name rather than a bare int —
// callers reading this over the network (internal/server's live snapshots)
// shouldn't have to know the underlying iota values.
func (r Role) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(r.String())), nil
}

// UnmarshalJSON is MarshalJSON's inverse, needed so a snapshot can round
// trip through JSON in tests (and any future client) without losing the
// Role, not just producing readable output one-way.
func (r *Role) UnmarshalJSON(data []byte) error {
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}
	switch s {
	case "Follower":
		*r = Follower
	case "Candidate":
		*r = Candidate
	case "Leader":
		*r = Leader
	default:
		return fmt.Errorf("raft: unknown Role %q", s)
	}
	return nil
}

// NodeState is a single Raft node's complete state. Fields are grouped by
// the persistent/volatile split called out in the correctness checklist:
// CurrentTerm/VotedFor/Log must survive a simulated restart; everything
// else is rebuilt on recovery.
type NodeState struct {
	ID    int
	Peers []int // sorted, excludes ID — never range a map of peers when sending RPCs

	// Persistent state — must be durably saved (OutPersist) before any
	// Outbound that depends on it is allowed to be sent.
	CurrentTerm uint64
	VotedFor    int // -1 means "no vote cast this term"
	Log         []LogEntry

	// Volatile state — all roles.
	Role        Role
	CommitIndex uint64
	LastApplied uint64

	// Volatile state — leader only. Reset whenever the node becomes leader.
	NextIndex  map[int]uint64
	MatchIndex map[int]uint64

	// Volatile state — candidate only. Reset whenever the node starts an election.
	VotesReceived map[int]bool

	// timerGen holds the current generation for each TimerKind. Bumped on
	// every legitimate reset; a fired EventTimerFire carrying an older
	// generation is stale and must be ignored.
	timerGen [2]uint64
}

// TimerGeneration returns the current generation for kind. A simulator
// reading this immediately after Step() returns an OutResetTimer sees the
// value the handler just bumped to — the generation an EventTimerFire must
// carry to not be dropped as stale later.
func (n *NodeState) TimerGeneration(kind TimerKind) uint64 {
	return n.timerGen[kind]
}

func NewNodeState(id int, peers []int) *NodeState {
	sorted := make([]int, len(peers))
	copy(sorted, peers)
	sort.Ints(sorted)

	return &NodeState{
		ID:       id,
		Peers:    sorted,
		VotedFor: -1,
		Role:     Follower,
	}
}

// Restart models this node coming back up after a simulated crash: every
// volatile field resets to a fresh node's defaults, while CurrentTerm/
// VotedFor/Log — the persistent state — survive untouched. That split is
// exactly what the correctness checklist calls out as an easy thing to get
// wrong across a restart.
//
// Both timer generations are bumped so any EventTimerFire still sitting in
// the simulator's queue from before the crash (e.g. a heartbeat timer, if
// this node was Leader) is stale and gets ignored rather than firing
// against the now-reset node. A fresh election timer is armed so the
// restarted node actually rejoins rather than sitting inert forever.
func (n *NodeState) Restart(rng *rand.Rand) []Outbound {
	n.Role = Follower
	n.CommitIndex = 0
	n.LastApplied = 0
	n.NextIndex = nil
	n.MatchIndex = nil
	n.VotesReceived = nil

	n.timerGen[TimerElection]++
	n.timerGen[TimerHeartbeat]++

	return []Outbound{{
		Kind:           OutResetTimer,
		TimerKindField: TimerElection,
		Duration:       sampleElectionTimeout(rng),
	}}
}

// Step is the ENTIRE interface between Raft logic and the outside world.
// No goroutines, no time.Now(), no I/O, no global rand — everything the
// node needs is an argument, everything it wants done is a returned Outbound.
func (n *NodeState) Step(ev Event, rng *rand.Rand) []Outbound {
	switch ev.Kind {
	case EventMessageArrival:
		return n.handleMessage(ev.From, ev.Message, rng)
	case EventTimerFire:
		if ev.TimerGen != n.timerGen[ev.TimerKindField] {
			return nil // stale — a reset happened after this was scheduled
		}
		return n.handleTimeout(ev.TimerKindField, rng)
	case EventClientRequest:
		return n.handleClientRequest(ev.Command)
	}
	return nil
}
