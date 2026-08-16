
# Chaos

<img width="1535" height="773" alt="image" src="https://github.com/user-attachments/assets/bcfe93ef-1596-4615-bd80-0241e09b83f3" />

Chaos is a Raft implementation written from scratch in Go, running on top of a deterministic network simulator instead of real sockets and real time. The simulator can drop messages, duplicate them, delay them, kill nodes, and partition the network, and because everything runs off a single seeded random number generator, any run can be replayed exactly.

I built this to actually understand Raft, not just read the paper and nod along. Consensus bugs are almost always timing bugs, and timing bugs on a real network are nearly impossible to reproduce reliably. Running the whole cluster inside a simulator you control means you can hit the same bad interleaving of messages over and over until you actually understand why it broke, instead of hoping it happens again.

## System architecture

<img width="2244" height="693" alt="Chaos Architecture" src="https://github.com/user-attachments/assets/b4a56a0d-7f5d-4c0c-992d-4f5440facd65" />

The simulator sits in the middle. It drives every Raft node through `Step(event, rng)` and gets back a list of actions to carry out, since the node never talks to the network directly. `cmd/chaos` runs seeded fault-injection scenarios against the simulator and checks Raft's safety properties after each one. `cmd/chaos-server` exposes one live cluster over HTTP through `internal/server`, and the web UI talks to that server over HTTP and server-sent events. `internal/twopc` sits off to the side as a comparison protocol, not part of the main flow.

## Why Go

Raft is fundamentally a bunch of independent actors passing messages to each other, which is what Go's concurrency model is built for. It's also what etcd's production Raft library is written in, so there's real prior art to check my implementation against once it works. Beyond that, the standard library's `container/heap` and `math/rand` were enough to build the whole event-loop simulator without pulling in a single external dependency, and the race detector (`go test -race`) is what actually lets me claim the deterministic core has no hidden concurrency bugs, not just that it looks correct.

## Why it's deterministic

Every Raft node is written as a pure function: `Step(event, rng)` returns a list of outbound actions. It doesn't send anything itself, it just describes what it wants done (send this message, persist this state, reset that timer), and the simulator is the only thing that actually carries it out. Since the node never touches real time, real concurrency, or Go's global rand, the same seed always produces the same sequence of events. That's what makes replaying a failing run possible instead of just hoping to catch it again.

## Fault types and a real scenario

The simulator injects five kinds of faults, all through the same code path that schedules message delivery:

- **Drop** - a message never arrives.
- **Duplicate** - a message arrives more than once.
- **Latency** - a message arrives late.
- **Kill / restart** - a node stops being scheduled entirely, then comes back with only its persisted state.
- **Partition / heal** - the network splits into groups that can't reach each other, then reconnects.

Reordering isn't a separate fault. If message A is sent before B but B samples a shorter delay, B just lands earlier in the event queue and gets delivered first.

A concrete run that exercises most of this: five nodes, node 1 is leader, entries 1 through 40 are committed. A new command comes in and gets replicated to nodes 2 and 3, which is already a majority, so it commits. Before it reaches nodes 4 and 5, a partition splits the cluster into {1,2,3} and {4,5}. The majority side keeps node 1 as leader and keeps committing normally. The minority side tries to elect a new leader and can't, since two votes out of five is never a majority, so it correctly stalls instead of doing anything wrong. When the partition heals, node 1 is still leader and uses the normal `AppendEntries` consistency check to bring nodes 4 and 5 back in sync. Nothing is lost, because nothing on the minority side was ever committed in the first place.

This scenario is encoded as a replayable test, and the same kind of thing runs continuously as part of the fault-injection sweep across many random seeds.

## What's here

- `internal/raft` - the Raft state machine itself. Leader election, log replication, commit index. No goroutines, no `time.Now()`, no real randomness anywhere in here, everything it needs gets passed in as an argument.
- `internal/sim` - the event loop that drives the nodes. It owns a virtual clock and a priority queue of events, and decides when (or if) a message actually gets delivered. Kill/restart/partition/heal live here too, as operations on scheduling and routing, not something a node is ever told about.
- `internal/server` - a small HTTP layer for running one live cluster and streaming its state out over server-sent events.
- `internal/twopc` - a basic two-phase commit implementation, mostly to have something to compare against Raft. 2PC's coordinator just blocks forever if it dies mid-commit; Raft elects a new leader and keeps going.
- `cmd/chaos` - a CLI that runs a batch of randomized fault-injection scenarios and checks Raft's safety properties after each one. Can also export a single scenario as a JSON trace.
- `cmd/chaos-server` - runs a live cluster over HTTP for the frontend to control.
- `web/` - a Next.js UI that either replays an exported trace step by step, or drives a live cluster and shows elections and replication happening as they occur.

## Running it

Needs Go 1.25+, and Node if you want the web UI.

```
go test ./... -race
```

The race detector isn't optional here. The whole point of this project is proving there's no hidden nondeterminism, so it has to stay clean.

Run a batch of fault-injection scenarios:

```
go run ./cmd/chaos -seeds 50 -rounds 200
```

Export one scenario as a trace file:

```
go run ./cmd/chaos -trace partition-heal -trace-out trace.json
```

Run a live cluster over HTTP:

```
go run ./cmd/chaos-server
```

Then in a second terminal:

```
cd web
npm install
npm run dev
```

Open the local URL it prints to watch a live cluster, or load a trace to step through a specific scenario.

Nothing here is deployed anywhere. It's meant to run locally.

<img width="2244" height="693" alt="Chaos Architecture" src="https://github.com/user-attachments/assets/b4a56a0d-7f5d-4c0c-992d-4f5440facd65" />
