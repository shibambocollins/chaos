// Command chaos-server hosts one live Raft cluster over HTTP: actions
// (kill/restart/partition/heal/client-request) are POST requests, and the
// live cluster state is a GET /stream of server-sent-events snapshots.
// Multiple clients can connect and act concurrently — see
// internal/server's package doc for how that's arbitrated safely.
package main

import (
	"flag"
	"log"
	"math/rand"
	"net/http"
	"time"

	"chaos/internal/raft"
	"chaos/internal/server"
	"chaos/internal/sim"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	nodeCount := flag.Int("nodes", 5, "cluster size")
	seed := flag.Int64("seed", 1, "rng seed for the underlying Simulator (election jitter, fault sampling)")
	tickIncrement := flag.Int("tick-increment", 5, "simulated ticks advanced per tick")
	tickMs := flag.Int("tick-ms", 5, "real milliseconds per tick")
	drop := flag.Float64("drop", 0, "per-message drop probability")
	duplicate := flag.Float64("duplicate", 0, "per-message duplicate probability")
	flag.Parse()

	ids := make([]int, *nodeCount)
	for i := range ids {
		ids[i] = i + 1
	}
	nodes := make(map[int]*raft.NodeState, len(ids))
	for _, id := range ids {
		var peers []int
		for _, other := range ids {
			if other != id {
				peers = append(peers, other)
			}
		}
		nodes[id] = raft.NewNodeState(id, peers)
	}

	s := sim.NewSimulator(nodes, rand.New(rand.NewSource(*seed)))
	s.SetFaultConfig(sim.FaultConfig{DropProbability: *drop, DuplicateProbability: *duplicate})
	for _, id := range ids {
		s.Schedule(raft.Event{At: 0, NodeID: id, Kind: raft.EventTimerFire, TimerKindField: raft.TimerElection, TimerGen: 0})
	}

	hub := server.NewHub(s, raft.Time(*tickIncrement), time.Duration(*tickMs)*time.Millisecond)
	go hub.Run()
	defer hub.Stop()

	log.Printf("chaos-server listening on %s (%d nodes, %d ticks / %dms)", *addr, *nodeCount, *tickIncrement, *tickMs)
	log.Printf("POST /kill/{id}  /restart/{id}  /partition {\"groups\":[[...]]}  /heal  /client-request {\"command\":\"...\"}")
	log.Printf("GET  /stream — live server-sent-events feed of cluster state")
	log.Fatal(http.ListenAndServe(*addr, server.NewMux(hub)))
}
