# Chaos

A from-scratch implementation of the **Raft consensus algorithm**, running inside a **deterministic, replayable simulated network**, with fault injection and correctness testing built in from day one — plus a live browser visualization you can watch (and break) in real time.

This isn't a Raft tutorial with a UI bolted on. The point of the project is the correctness engineering: proving that a cluster of nodes stays consistent even when the network is actively working against it.

---

## What problem is this actually solving?

Imagine you have several database servers that are supposed to agree on the same sequence of operations — say, five copies of a bank ledger. If they ever disagree about the order events happened in, you get double-spends and lost transactions. Keeping distributed copies of data in agreement, even when machines crash and network links fail, is one of the hardest problems in computer science. **Raft** is an algorithm that solves it: it lets a cluster of machines elect a leader, replicate a log of operations, and keep working correctly even if some machines die or the network partitions into two groups that can't talk to each other.

The catch with implementing something like Raft is that its hardest bugs only show up under specific, rare timing conditions — a message arriving a few milliseconds late, two crashes happening back to back, a partition healing at just the wrong moment. Testing against a real network means waiting for these situations to happen by chance, and a bug that only reproduces once every 500 runs is nearly impossible to debug.

This project sidesteps that by never using a real network or real clock at all. Instead, the entire cluster runs inside a **deterministic simulator**: a single-threaded event loop that owns a virtual clock and decides exactly when every message is delivered, dropped, duplicated, or delayed. Every random decision (election timeouts, message loss, latency) is derived from one seeded random number generator. The result: the *exact same seed* always produces the *exact same sequence of events*, every time. That means:

- A bug found once can be replayed exactly, instead of chased for hours.
- Thousands of randomized failure scenarios can run in seconds, instead of real time.
- Correctness can be checked continuously against Raft's formal safety properties, not just eyeballed from a demo.

This is the same technique used by systems like FoundationDB and TigerBeetle to validate distributed databases before they ship.

---

## How it works, technically

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     internal/sim (Simulator)                │
│  Owns: virtual clock, event min-heap, one seeded *rand.Rand │
│  Decides: delivery order, drop/duplicate/latency,           │
│           Kill / Restart / Partition / Heal                 │
└───────────────────────────┬───────────────────────────────--┘
                             │ Event in → []Outbound out
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    internal/raft (NodeState)                │
│  Pure function core: Step(Event, rng) []Outbound             │
│  No time.Now(), no goroutines, no global rand, no I/O        │
│  Follower / Candidate / Leader, terms, log replication        │
└─────────────────────────────────────────────────────────────┘
```

**The core design decision:** a Raft node is a pure function, not a live goroutine. `Step(event, rng) []Outbound` takes one event (a message arriving, a timer firing) and returns a description of what it wants done — send this message, persist this state, reset that timer. It never does any of that itself. The simulator is the only thing that actually moves time forward, delivers messages, or applies faults. This separation is what makes exact replay possible: nothing in the node's logic can introduce timing variance, because it has no access to real time, real randomness, or real concurrency in the first place.

**Fault injection is not special-cased.** Drop, duplicate, and latency are just parameters to the one function that decides whether/when a "send this message" request turns into a scheduled delivery event. Reordering isn't implemented as its own feature — it falls out for free: if message A is sent before message B but B samples a shorter delay, B is simply delivered first. Kill, Restart, Partition, and Heal are simulator-level operations on scheduling and routing — a node is never told it's being tested; a killed node just silently stops being scheduled, the same way a real crashed process gets no warning either.

### What's implemented

- **`internal/raft`** — the Raft state machine: leader election with randomized timeouts and the election-log-restriction safety check, log replication with the log-matching property, and the current-term-only rule for advancing commit index (the classic Raft correctness pitfall, per the paper's Figure 8 scenario).
- **`internal/sim`** — the deterministic event-loop simulator: a min-heap event queue, fault injection (drop/duplicate/latency), Kill/Restart/Partition/Heal, and a `SafetyMonitor` that continuously checks all five of Raft's safety properties (Election Safety, Leader Append-Only, Log Matching, Leader Completeness, State Machine Safety) throughout a run — not just at the end.
- **`internal/server`** — an HTTP layer that hosts one live cluster, exposing snapshots and a server-sent-events stream so a frontend can watch (and control) a real running cluster.
- **`internal/twopc`** — a simplified Two-Phase Commit implementation, kept as a point of comparison: 2PC's coordinator blocks forever on failure, where Raft elects a new leader and keeps going.
- **`cmd/chaos`** — a CLI fault-injection runner: drives many seeded, randomized scenarios (kill/restart/partition/heal/client-requests) through the simulator and reports pass/fail against the five safety properties, or exports a single curated scenario as a JSON trace.
- **`cmd/chaos-server`** — hosts a live cluster over HTTP for the frontend to drive interactively.
- **`web/`** — a Next.js/React/TypeScript visualization with two modes: replaying an exported trace step-by-step, or driving a live cluster in real time (kill nodes, partition the network, heal it, submit client commands, and watch leader election and replication happen).

### Correctness rules enforced throughout

- No `time.Now()`, no package-level `math/rand`, no goroutines, no real blocking channel sends anywhere in the deterministic core — everything nondeterministic is passed in explicitly.
- Never range over a Go map when iterating peers to send RPCs (map iteration order is randomized in Go, which would silently break replay) — always a sorted slice of peer IDs.
- Every timer has a generation counter, bumped on every legitimate reset, so a stale timer firing after a reset is correctly ignored.
- A leader only advances its commit index off a majority-replicated count for entries from its own current term — never an older term, even if a majority now happens to hold a copy of it.
- A voter rejects `RequestVote` if the candidate's log is less up-to-date than its own.
- State is persisted (`OutPersist`) before any corresponding message is sent (`OutSendMessage`), on every path.

---

## Running it

**Requirements:** Go 1.25+, Node.js (for the web UI).

```bash
# Run the full test suite, race detector included (non-optional on this project —
# the entire point is proving there's no hidden nondeterminism)
go test ./... -race

# Run a batch of randomized fault-injection scenarios
go run ./cmd/chaos -seeds 50 -rounds 200

# Export one curated scenario (e.g. the partition/heal worked example) as a trace
go run ./cmd/chaos -trace partition-heal -trace-out trace.json

# Host a live cluster over HTTP
go run ./cmd/chaos-server
```

```bash
# In a second terminal, run the frontend against the live server
cd web
npm install
npm run dev
```

Then open the printed local URL to watch the cluster, or load an exported trace to step through a specific scenario.

This project is developed and run locally — it isn't deployed or hosted anywhere.

---

## Project layout

```
internal/raft     Raft state machine — pure, deterministic, no I/O
internal/sim      Event-loop simulator, fault injection, safety monitor
internal/server   HTTP layer for driving a live cluster
internal/twopc    Two-Phase Commit, for comparison against Raft
cmd/chaos         CLI fault-injection runner / trace exporter
cmd/chaos-server  HTTP server hosting a live cluster
cmd/twopc         2PC demo entrypoint
web/              Next.js/React visualization (trace playback + live mode)
docs/             Full architecture doc, decisions log, Raft correctness reference
```

See [`docs/CHAOS_PROJECT_CONTEXT.md`](docs/CHAOS_PROJECT_CONTEXT.md) for the complete design rationale, the full Raft correctness reference, and the type definitions the implementation is built from.

## Roadmap status

- [x] Phase 1 — Raft fundamentals
- [x] Phase 2 — Deterministic simulator + Raft implementation
- [ ] Phase 3 — React/TypeScript visualization
- [x] Phase 4 — Fault injection + correctness tests (random drop/duplicate, partition/heal, replay-determinism, property-based sweeps across 50 seeds × 200 rounds)
