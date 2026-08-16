// Command twopc runs a small deterministic 2PC scenario and narrates it,
// existing mainly to demonstrate one thing: a participant that has voted
// to commit cannot safely resolve itself if the coordinator disappears, so
// it blocks for as long as the coordinator is unavailable — however long
// that is. Compare internal/sim's kill/restart tests, where a Raft cluster
// recovers a working leader within tens of ticks and never needs the
// original one to return at all.
package main

import (
	"flag"
	"fmt"
	"os"

	"chaos/internal/twopc"
)

func main() {
	scenario := flag.String("scenario", "crash", "scenario to run: commit, abort, or crash")
	flag.Parse()

	switch *scenario {
	case "commit":
		runCommit()
	case "abort":
		runAbort()
	case "crash":
		runCrash()
	default:
		fmt.Fprintf(os.Stderr, "unknown scenario %q (want commit, abort, or crash)\n", *scenario)
		os.Exit(1)
	}
}

func buildCluster() *twopc.Simulator {
	nodes := map[int]*twopc.NodeState{
		1: twopc.NewCoordinator(1, []int{2, 3}),
		2: twopc.NewParticipant(2, 1, false),
		3: twopc.NewParticipant(3, 1, false),
	}
	return twopc.NewSimulator(nodes)
}

func printStates(s *twopc.Simulator, label string) {
	fmt.Printf("--- %s (tick=%d) ---\n", label, s.Now())
	c := s.Node(1)
	fmt.Printf("  coordinator(1): state=%v decision=%v alive=%v\n", c.CoordinatorState, c.Decision, s.IsAlive(1))
	for _, id := range []int{2, 3} {
		p := s.Node(id)
		fmt.Printf("  participant(%d): state=%v alive=%v\n", id, p.ParticipantState, s.IsAlive(id))
	}
	fmt.Println()
}

func runCommit() {
	fmt.Println("=== 2PC happy path: every participant votes Commit ===")
	fmt.Println()

	s := buildCluster()
	s.Schedule(twopc.Event{At: 0, NodeID: 1, Kind: twopc.EventClientRequest, Command: []byte("txn-commit")})
	s.Run(100)

	printStates(s, "Final state")
}

func runAbort() {
	fmt.Println("=== 2PC abort path: participant 3 votes Abort ===")
	fmt.Println("A single Abort vote decides the whole transaction immediately —")
	fmt.Println("the coordinator doesn't wait for the rest.")
	fmt.Println()

	nodes := map[int]*twopc.NodeState{
		1: twopc.NewCoordinator(1, []int{2, 3}),
		2: twopc.NewParticipant(2, 1, false),
		3: twopc.NewParticipant(3, 1, true),
	}
	s := twopc.NewSimulator(nodes)
	s.Schedule(twopc.Event{At: 0, NodeID: 1, Kind: twopc.EventClientRequest, Command: []byte("txn-abort")})
	s.Run(100)

	printStates(s, "Final state")
}

func runCrash() {
	fmt.Println("=== 2PC coordinator-crash demonstration ===")
	fmt.Println("Both participants vote Commit and enter the 'Prepared' (uncertain) state.")
	fmt.Println("The coordinator then crashes before deciding. Unlike Raft, no other node")
	fmt.Println("can resolve this transaction — only this specific coordinator, coming back,")
	fmt.Println("can unblock it.")
	fmt.Println()

	s := buildCluster()
	s.Schedule(twopc.Event{At: 0, NodeID: 1, Kind: twopc.EventClientRequest, Command: []byte("txn-42")})

	for i := 0; i < 4; i++ {
		s.Step()
	}
	printStates(s, "Both participants Prepared; coordinator still awaiting the last vote")

	fmt.Println(">>> Killing the coordinator now. <<<")
	fmt.Println()
	s.Kill(1)

	s.Run(100_000)
	printStates(s, "Ran out to tick 100,000 with the coordinator dead (queue went idle at tick=20 — nothing left to happen)")
	fmt.Println("Both participants are still blocked. A Raft cluster in the same situation")
	fmt.Println("would have elected a new leader within tens of ticks; here, nothing can")
	fmt.Println("happen without this exact coordinator coming back.")
	fmt.Println()

	fmt.Println(">>> Restarting the coordinator. <<<")
	fmt.Println()
	s.Restart(1)
	s.Run(s.Now() + 10)

	printStates(s, "After coordinator restart")
	fmt.Println("The coordinator lost its in-memory vote tally on restart, so it safely")
	fmt.Println("defaulted to Abort — the only choice that couldn't contradict a promise")
	fmt.Println("already made — and re-broadcast that decision, which is what finally")
	fmt.Println("unblocked both participants.")
}
