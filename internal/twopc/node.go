package twopc

import "sort"

const prepareTimeout Time = 20

// NewCoordinator constructs a Coordinator node overseeing participants.
func NewCoordinator(id int, participants []int) *NodeState {
	sorted := make([]int, len(participants))
	copy(sorted, participants)
	sort.Ints(sorted)

	return &NodeState{
		ID:    id,
		Peers: sorted,
		Role:  Coordinator,
	}
}

// NewParticipant constructs a Participant node reporting to coordinatorID.
// alwaysAbort forces this participant to vote Abort on every transaction —
// a test/demo knob for exercising the abort path, not part of the protocol
// itself.
func NewParticipant(id, coordinatorID int, alwaysAbort bool) *NodeState {
	return &NodeState{
		ID:          id,
		Peers:       []int{coordinatorID},
		Role:        Participant,
		AlwaysAbort: alwaysAbort,
	}
}

// TimerGeneration returns the current generation for kind, mirroring
// raft.NodeState's accessor of the same name and for the same reason: a
// simulator needs this to correctly stamp a scheduled EventTimerFire.
func (n *NodeState) TimerGeneration(kind TimerKind) uint64 {
	return n.timerGen[kind]
}

// Step is the entire interface between 2PC logic and the outside world —
// same shape as raft.NodeState.Step, minus the *rand.Rand argument, since
// nothing here is randomized.
func (n *NodeState) Step(ev Event) []Outbound {
	switch ev.Kind {
	case EventClientRequest:
		return n.handleClientRequest(ev.Command)
	case EventMessageArrival:
		return n.handleMessage(ev.From, ev.Message)
	case EventTimerFire:
		if ev.TimerGen != n.timerGen[ev.TimerKindField] {
			return nil
		}
		return n.handleTimeout(ev.TimerKindField)
	}
	return nil
}

// Restart models this node coming back up after a simulated crash.
//
// A Participant's ParticipantState is untouched by design: Prepared must
// come back Prepared, still blocked, not reset to Idle — see the package
// doc for why guessing either outcome would be unsafe.
//
// A Coordinator that had already reached Decided must replay that exact
// decision — this is what eventually unblocks a Participant still waiting,
// however long the coordinator was gone. A Coordinator caught mid-vote-
// collection (WaitingForVotes, no persisted Decision) has lost its
// in-memory vote tally and falls back to the one safe default: decide
// Abort now, since nothing has ever been told Commit.
func (n *NodeState) Restart() []Outbound {
	n.VotesReceived = nil

	if n.Role != Coordinator {
		return nil
	}

	switch n.CoordinatorState {
	case WaitingForVotes:
		return n.decide(DecisionAbort)
	case Decided:
		var out []Outbound
		for _, peer := range n.Peers {
			out = append(out, Outbound{
				Kind: OutSendMessage, To: peer,
				Message: &Message{Kind: MsgDecision, Decision: n.Decision},
			})
		}
		return out
	default:
		return nil
	}
}

func (n *NodeState) persistOutbound() Outbound {
	return Outbound{
		Kind:                      OutPersist,
		PersistedParticipantState: n.ParticipantState,
		PersistedCoordinatorState: n.CoordinatorState,
		PersistedDecision:         n.Decision,
		PersistedCommand:          n.Command,
	}
}
