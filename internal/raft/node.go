package raft

import (
	"math/rand"
	"sort"
)

type Role int

const (
	Follower Role = iota
	Candidate
	Leader
)

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
	}
	return nil
}
