
# Chaos

<img width="1535" height="773" alt="image" src="https://github.com/user-attachments/assets/bcfe93ef-1596-4615-bd80-0241e09b83f3" />


Chaos is a Raft implementation written from scratch in Go, running on top of a deterministic network simulator instead of real sockets and real time. The simulator can drop messages, duplicate them, delay them, kill nodes, and partition the network, and because everything runs off a single seeded random number generator, any run can be replayed exactly.

I built this to actually understand Raft, not just read the paper and nod along. Consensus bugs are almost always timing bugs, and timing bugs on a real network are nearly impossible to reproduce reliably. Running the whole cluster inside a simulator you control means you can hit the same bad interleaving of messages over and over until you actually understand why it broke, instead of hoping it happens again.

## What's here

- `internal/raft` - the Raft state machine itself. Leader election, log replication, commit index. No goroutines, no `time.Now()`, no real randomness anywhere in here, everything it needs gets passed in as an argument.
- `internal/sim` - the event loop that drives the nodes. It owns a virtual clock and a priority queue of events, and decides when (or if) a message actually gets delivered. Kill/restart/partition/heal live here too, as operations on scheduling and routing, not something a node is ever told about.
- `internal/server` - a small HTTP layer for running one live cluster and streaming its state out over server-sent events.
- `internal/twopc` - a basic two-phase commit implementation, mostly to have something to compare against Raft. 2PC's coordinator just blocks forever if it dies mid-commit; Raft elects a new leader and keeps going.
- `cmd/chaos` - a CLI that runs a batch of randomized fault-injection scenarios and checks Raft's safety properties after each one. Can also export a single scenario as a JSON trace.
- `cmd/chaos-server` - runs a live cluster over HTTP for the frontend to control.
- `web/` - a Next.js UI that either replays an exported trace step by step, or drives a live cluster and shows elections and replication happening as they occur.

## Why it's deterministic

Every Raft node is written as a pure function: `Step(event, rng)` returns a list of outbound actions. It doesn't send anything itself, it just describes what it wants done (send this message, persist this state, reset that timer), and the simulator is the only thing that actually carries it out. Since the node never touches real time, real concurrency, or Go's global rand, the same seed always produces the same sequence of events. That's what makes replaying a failing run possible instead of just hoping to catch it again.

Fault injection isn't a special mode bolted on top. Drop, duplicate, and latency are just parameters in the one function that turns a "send this message" call into a scheduled delivery event. Reordering isn't even implemented as its own thing, it just happens naturally when a later message samples a shorter delay and ends up delivered first.

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
