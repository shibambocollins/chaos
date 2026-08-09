package twopc

import "testing"

func newClusterWithAbort(alwaysAbort ...int) *Simulator {
	abort := map[int]bool{}
	for _, id := range alwaysAbort {
		abort[id] = true
	}
	nodes := map[int]*NodeState{
		1: NewCoordinator(1, []int{2, 3}),
		2: NewParticipant(2, 1, abort[2]),
		3: NewParticipant(3, 1, abort[3]),
	}
	return NewSimulator(nodes)
}

func TestSimulator_HappyPathAllCommit(t *testing.T) {
	s := newClusterWithAbort()
	s.Schedule(Event{At: 0, NodeID: 1, Kind: EventClientRequest, Command: []byte("txn")})

	s.Run(100)

	for _, id := range []int{2, 3} {
		if got := s.Node(id).ParticipantState; got != ParticipantCommitted {
			t.Fatalf("node %d: expected ParticipantCommitted, got %v", id, got)
		}
	}
	if got := s.Node(1).Decision; got != DecisionCommit {
		t.Fatalf("coordinator: expected DecisionCommit, got %v", got)
	}
}

func TestSimulator_AbortPathWhenAnyParticipantVotesAbort(t *testing.T) {
	s := newClusterWithAbort(3) // node 3 always votes Abort
	s.Schedule(Event{At: 0, NodeID: 1, Kind: EventClientRequest, Command: []byte("txn")})

	s.Run(100)

	for _, id := range []int{2, 3} {
		if got := s.Node(id).ParticipantState; got != ParticipantAborted {
			t.Fatalf("node %d: expected ParticipantAborted, got %v", id, got)
		}
	}
	if got := s.Node(1).Decision; got != DecisionAbort {
		t.Fatalf("coordinator: expected DecisionAbort, got %v", got)
	}
}

func TestSimulator_PrepareTimeoutAbortsWhenAParticipantNeverResponds(t *testing.T) {
	s := newClusterWithAbort()
	s.Kill(3) // node 3 never receives its Prepare, never votes

	s.Schedule(Event{At: 0, NodeID: 1, Kind: EventClientRequest, Command: []byte("txn")})
	s.Run(100) // well past prepareTimeout

	if got := s.Node(1).Decision; got != DecisionAbort {
		t.Fatalf("coordinator: expected DecisionAbort after prepareTimeout with a missing vote, got %v", got)
	}
	if got := s.Node(2).ParticipantState; got != ParticipantAborted {
		t.Fatalf("node 2: expected ParticipantAborted, got %v", got)
	}
}

// TestSimulator_CoordinatorCrashBlocksBothPreparedParticipantsUntilRestart is
// the demonstration this whole package exists for. Both participants vote
// Commit and enter Prepared before the coordinator crashes with one vote
// reply still unread. While the coordinator is down, both participants
// stay blocked — not for some bounded timeout, but indefinitely, however
// long "down" turns out to mean. Restarting the coordinator is the only
// thing that resolves it, and it resolves to Abort here specifically
// because the coordinator's own crash landed before it had decided
// anything (see TestRestart_CoordinatorMidVoteCollectionDefaultsToAbort).
func TestSimulator_CoordinatorCrashBlocksBothPreparedParticipantsUntilRestart(t *testing.T) {
	s := newClusterWithAbort()
	s.Schedule(Event{At: 0, NodeID: 1, Kind: EventClientRequest, Command: []byte("txn")})

	// Step through by hand to land exactly between the two vote replies
	// arriving at the coordinator, rather than relying on Run(until) and a
	// tick guess: 1) EventClientRequest -> Prepare sent to 2 and 3;
	// 2) Prepare arrives at 2 -> votes Commit, enters Prepared;
	// 3) Prepare arrives at 3 -> votes Commit, enters Prepared;
	// 4) node 2's VoteReply arrives at the coordinator (still waiting on 3).
	for i := 0; i < 4; i++ {
		if !s.Step() {
			t.Fatalf("setup: queue emptied after only %d of 4 expected events", i)
		}
	}
	if got := s.Node(2).ParticipantState; got != ParticipantPrepared {
		t.Fatalf("setup: expected node 2 Prepared, got %v", got)
	}
	if got := s.Node(3).ParticipantState; got != ParticipantPrepared {
		t.Fatalf("setup: expected node 3 Prepared, got %v", got)
	}
	if got := s.Node(1).CoordinatorState; got != WaitingForVotes {
		t.Fatalf("setup: expected coordinator still WaitingForVotes, got %v", got)
	}

	// Crash now: node 3's still-in-flight VoteReply, and the eventual
	// TimerPrepare fire, will both be silently dropped on delivery — the
	// coordinator is not there to receive them.
	s.Kill(1)

	// Run far past every timeout that would ever have fired had the
	// coordinator been alive. Nothing resolves — that's the point.
	s.Run(100_000)

	for _, id := range []int{2, 3} {
		if got := s.Node(id).ParticipantState; got != ParticipantPrepared {
			t.Fatalf("node %d: expected still Prepared (blocked) after 100,000 ticks with the coordinator dead, got %v", id, got)
		}
	}

	// Restart: the coordinator lost its in-memory vote tally, so it
	// defaults to Abort — the one safe choice, since nothing was ever told
	// Commit. That broadcast is what finally unblocks both participants.
	s.Restart(1)
	s.Run(s.Now() + messageLatency + 5)

	for _, id := range []int{2, 3} {
		if got := s.Node(id).ParticipantState; got != ParticipantAborted {
			t.Fatalf("node %d: expected Aborted after coordinator restart resolved the transaction, got %v", id, got)
		}
	}
}
