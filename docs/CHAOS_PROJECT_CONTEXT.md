# Chaos — Project Context & Architecture

**What this document is:** the consolidated record of every decision made so far, why it was made, and the technical foundation the implementation sits on. This is meant to stand alone — if you come back to this in three weeks having forgotten the conversation that produced it, this document should be enough to keep building without re-deriving any of it.

**What Chaos is, in one sentence:** a from-scratch Raft implementation running inside a deterministic, replayable simulated network, with fault injection and correctness testing built in from the start — not a visualization with Raft bolted on.

**The standard this project is held to, in your own words:**

> "I implemented a distributed consensus system, created a controllable virtual network around it, deliberately injected failures, and built automated tests to verify that the system maintained its safety and consistency guarantees."

**The priority order, also in your own words, and the one rule everything else in this document serves:**

> Correctness > understanding > testing > realistic simulation > visualization > aesthetics.

**A note on how this document is written:** it gives you architecture, types, and contracts — the shape of the thing. It deliberately does **not** hand you a finished implementation of the Raft election/replication logic. You said you want to understand this well enough to explain it in an interview, not have it generated for you — the sections below respect that on purpose, and say so explicitly where it matters.

**This document's companion:** `CLAUDE.md`, at the repo root, is a separate file Claude Code reads automatically at the start of every session. This document is the spec — architecture, types, correctness rules, the "what" and "why." `CLAUDE.md` is the "how to work with me on this" file — collaboration style, commit conventions, and a condensed restatement of the non-negotiable rules for quick reference during a session. If the two ever conflict, this document wins; `CLAUDE.md` should be updated to match, not the other way around.

---

## Project Status

*(Update this each session so a cold read tells you exactly where things stand.)*

- [x] Phase 1 — Raft fundamentals understood
- [x] Development environment set up — Go, VS Code, Claude Code, GitHub repo, `CLAUDE.md` (see Section 9)
- [ ] Phase 2 — Deterministic simulator architecture + Raft implementation (`internal/raft`, `internal/sim`)
- [ ] Phase 3 — React/TypeScript visualization
- [ ] Phase 4 — Fault injection + correctness tests on the deterministic core
- [ ] Optional phases — see Section 2

---

## 1. Decisions Log

Recorded so you never have to wonder "why did I choose that" six weeks from now.

| Decision | Choice | Why |
|---|---|---|
| Language for the core | **Go** | Raft is fundamentally concurrent message-passing between independent actors — goroutines/channels model that directly. Java is possible but would need real concurrency design, not a default Spring Boot CRUD pattern, to be appropriate. Go is also what production Raft implementations (etcd) are written in — real prior art to compare against. |
| How the node executes | **Pure function, single-threaded event loop (Option A)** — not real goroutines synchronized to a virtual clock (Option B) | Option B means you're responsible for pausing/resuming real goroutines in lockstep — get that even slightly wrong and you've reintroduced the exact nondeterminism this whole architecture exists to eliminate, except now it's hiding in your test harness instead of your Raft code. Option A's cost to switch to Docker later is small: only what sits *underneath* `Step()` changes (real sockets instead of the event queue), the Raft logic itself was pure from day one. |
| Correctness testing | **Deterministic simulation testing** (same family of technique as FoundationDB's simulation testing and TigerBeetle's VOPR) | Real fault injection over real time is nearly unreproducible — a bug that appears once in 500 runs is nearly impossible to chase. A deterministic core lets you replay the *exact same* failing scenario from a seed and run thousands of randomized fault scenarios in seconds instead of real time. |
| Fault injection mechanism | Not special-cased. Delay/drop/duplicate/reorder are all just parameters to **one function** deciding when/whether a message-arrival event gets scheduled | Keeps the Raft node fully ignorant of the fact that it's being tested — it just reacts to whatever the queue delivers, which is exactly how it should behave against a real network too. |
| Second protocol (optional) | 2-Phase Commit, kept simple, over Paxos | Simpler to implement correctly, and the coordinator-blocking-forever-on-failure vs Raft-elects-a-new-leader contrast is easier to demo and explain than a Raft/Paxos comparison would be. |
| Docker (optional) | Yes, later phase, budgeted as its own learning milestone | Real virtual networking (real IP addressing, real packet loss) is more convincing than in-process simulation, but you don't know Docker yet — don't let it get silently absorbed into another phase's estimate. |
| Multiplayer/collaborative (optional) | Keep, but lowest priority, first to cut if time runs short | Passes the "free/low-memory" filter cleanly, but hides real *design* cost your filter doesn't measure: concurrent actions on the same cluster (two people partitioning the same node in the same instant) need real conflict-handling logic, not just WebSocket plumbing you already know. |
| Experiment recording ("Phase 9" in earlier drafts) | Keep — but understand it's not separate work | Once the deterministic simulator exists, every run already produces a structured event log by necessity (that's what makes replay possible). Turning that log into a pass/fail report is a small addition on top of something you're building anyway, not a new subsystem. |
| Development workflow | **Claude Code, run from inside VS Code**, not code pasted from chat | Keeps you as the one reviewing and approving every diff — matches the "understand it, don't just have it exist" standard at the top of this document. The workflow itself is a decision, not just a tooling preference: it's what makes the interactive check-ins at the two correctness gotchas (Section 3) actually happen instead of being skipped. |
| Git commit attribution | **Disabled** — `attribution.commit`/`attribution.pr` set to `""` in `.claude/settings.json`, reinforced as an instruction in `CLAUDE.md` | Worth being precise about what this does and doesn't fix: commits are already authored under your own git identity regardless of this setting — Claude Code commits as you, not as a separate account — so this isn't correcting a real authorship problem. It removes ambiguity from the commit log for anyone reading it later (an interviewer, a marker), which matters more here than in a typical repo, given the whole point of this project is demonstrating what *you* understood and built. |
| Project layout | Standard Go layout — `internal/raft`, `internal/sim`, `cmd/chaos`, `docs/`, `web/` (the last one added at Phase 5) | `internal/` is a compiler-enforced boundary in Go, not just a convention — packages under it cannot be imported from outside this module. That's the correct way to mark "implementation detail, not a public API" for something that isn't a library. |

---

## 2. Roadmap

**Core (not optional — this is the actual portfolio project):**

1. Learn Raft fundamentals (see Section 3 — you're past this point already)
2. **Design the deterministic simulator architecture** (Section 4/5) — before any Raft code, because retrofitting determinism onto code written against real goroutines/real time doesn't work cleanly
3. Implement Raft on top of that architecture: states, terms, voting, log replication, commit index
4. Fault injection + correctness tests on the deterministic core (experiment recording effectively falls out of this step)
5. React/TypeScript visualization

**Optional — gated by "no heavy config, no heavy memory use, free," roughly in build order:**

6. Experiment/report generation (cheapest addition, near-automatic once step 4 exists)
7. Second protocol — simplified 2PC, only if you want the comparative "why Raft" story
8. Docker + real virtual networking (budget real learning time — this is the one item that looks free on the resource filter but isn't free on the time filter)
9. Multiplayer/collaborative mode (last; first cut if time runs short — see Decisions Log for why)

---

## 3. Raft Correctness Reference

The knowledge base — what you need internalized before writing `handleMessage`/`handleTimeout`, not just "understand the shape of it."

### States, terms, the two RPCs

Every node is **Follower**, **Candidate**, or **Leader**. Time is divided into **terms** — a monotonically increasing integer, incremented on every election. At most one leader can exist per term (Election Safety — see the five properties below).

Raft needs exactly two RPCs:

```
RequestVote(term, candidateId, lastLogIndex, lastLogTerm) → (term, voteGranted)
AppendEntries(term, leaderId, prevLogIndex, prevLogTerm, entries[], leaderCommit) → (term, success)
```

`AppendEntries` with an empty `entries[]` doubles as the heartbeat.

### Election

Followers wait for a **randomized** timeout. This randomization is load-bearing, not cosmetic — without it, every node times out simultaneously, every node becomes a candidate simultaneously, votes split evenly, nobody wins, and you get infinite failed elections. On timeout: increment term, vote for self, send `RequestVote` to everyone. Majority of votes → leader, immediately send heartbeats to prevent anyone else timing out.

**Rule that applies in every state, always:** if a node ever sees a higher term in *any* message — not just a `RequestVote` response, also an `AppendEntries` heartbeat from a stale leader — it steps down to follower immediately, no matter what state it was in.

### Gotcha #1 — the election restriction

A voter must **reject** a vote if the candidate's log is *less up-to-date* than its own — compare `lastLogTerm` first, then `lastLogIndex` as a tiebreak. Skip this and you can elect a leader that's missing entries the rest of the cluster already agreed on — a direct safety violation, not a cosmetic bug. This is the single most commonly forgotten rule in a first-pass implementation.

**Flagged in `CLAUDE.md`:** this is one of the two points where Claude Code should stop and have you state the rule back before writing the code.

### Log replication and the log matching property

Leader appends the new command to its own log, sends `AppendEntries` to every follower in parallel. A follower only accepts if the entry at `prevLogIndex` matches `prevLogTerm` — the **log matching property**: if two logs agree on the entry at some index, they agree on everything before it too. On mismatch, the follower rejects; the leader backs off `nextIndex` for that follower and retries until it finds agreement, then overwrites the follower's log from there.

*(Optional efficiency note, not a correctness requirement: a follower can return `ConflictIndex`/`ConflictTerm` in its reject response so the leader can jump `nextIndex` back directly instead of backing off one entry at a time — this is in the type design in Section 6 as optional fields, implement it only once the naive version works.)*

### Gotcha #2 — committing across terms (the one that causes hard-to-find bugs)

A leader may only advance `commitIndex` based on "replicated on a majority" **for entries from its own current term.** It cannot look at an older-term entry, see a majority now happens to have a copy, and declare it committed on that basis alone. Older entries only become committed *indirectly*, as a side effect of a later, current-term entry being committed. The Raft paper's Figure 8 scenario is the canonical proof of why this matters: a specific sequence of leader crashes and partial replications where an entry can end up replicated on a majority and still get overwritten later if you commit it directly instead of waiting.

**Action item:** write a deliberate test reproducing this scenario rather than hoping random fault injection stumbles onto it — it's rare enough that pure randomization might not hit it in a reasonable number of runs.

**Flagged in `CLAUDE.md`:** the second of the two points where Claude Code should stop and have you state the rule back before writing the code.

### The five safety properties — write these directly into your test suite

1. **Election Safety** — at most one leader per term
2. **Leader Append-Only** — a leader never deletes or overwrites entries in its own log
3. **Log Matching** — as above
4. **Leader Completeness** — once an entry is committed, every future leader's log contains it
5. **State Machine Safety** — if any server applies an entry at a given log index, no other server ever applies a *different* entry at that same index

### Worked example — partition and heal

5 nodes, Node 1 leader in term 3, entries 1–40 committed. New command → entry 41 replicated to Node 2 and Node 3 (majority of 5 → committed), but delivery to Node 4/5 is still in flight when a partition splits `{1,2,3}` from `{4,5}`.

- Majority side keeps Node 1 as leader, keeps committing normally.
- Minority side (`{4,5}`) cannot elect anyone — a candidate there gets at most 2 votes, never a majority of 5 — so it correctly stalls. That stall is Raft working correctly, not a bug.
- On heal, Node 1 (still leader) uses `AppendEntries`'s consistency check to find where Node 4/5's logs diverge and overwrites their tail. No data is lost, because nothing on the minority side was ever committed — exactly what the majority-commit rule protects.

This scenario, encoded as a replayable deterministic test, directly exercises Leader Completeness and State Machine Safety.

### Where from-scratch implementations usually break — checklist

- [ ] No randomized election timeout → livelock
- [ ] Not stepping down on a higher term seen *anywhere*, including as a follower receiving a stale leader's heartbeat
- [ ] Applying an entry to the state machine before it's actually committed, not just appended
- [ ] Directly committing an old-term entry off a majority count (Gotcha #2)
- [ ] Not correctly splitting persisted state (`currentTerm`, `votedFor`, `log`) from volatile state (`commitIndex`, `lastApplied`, leader-only `nextIndex[]`/`matchIndex[]`) across a simulated restart
- [ ] Not handling duplicate or reordered RPC *responses* idempotently at the network layer — directly relevant here, since duplication and reordering are on your own fault-injection list; a stale response from a term the node has since moved past needs to be recognized and discarded, or you get state corruption that has nothing to do with Raft and everything to do with the simulated network layer

---

## 4. Deterministic Simulator — Design Principles

**The one rule the whole architecture serves:** the Raft node's logic must never call anything nondeterministic itself. No `time.Now()`, no package-level `math/rand`, no goroutines, no real blocking channel sends inside the core. Everything the node needs (simulated time, a random number for election jitter) is handed to it. This is what makes exact replay possible: same seed in → same event sequence out, every time, because there's no OS thread scheduler and no wall clock anywhere in the loop to introduce variance.

This mirrors how etcd's production `raft` library is actually structured: the core state machine has no knowledge of networking or timers at all — it consumes inputs and returns "here's what you need to do." Worth reading once your own version works, as a sanity check, not a template to copy.

**Simulated time:** not wall-clock — an integer tick counter (`type Time uint64`) that only advances because the event loop says so.

**The event queue:** a min-heap ordered by `(At, Seq)`. `Seq` is a monotonically increasing counter assigned at scheduling time — Go's heap doesn't guarantee stable ordering among equal `At` values on its own, so without an explicit tiebreak you get silent nondeterminism between "identical" runs.

**Fault injection lives in exactly one place:** the function that turns a node's "send this message" request into a scheduled arrival event. Partition check, random drop, random duplicate, latency sampling — all in one function. **Reorder is never implemented as its own case** — it falls out for free: if message A is sent before message B but B samples a shorter latency, B's event lands earlier in the heap and gets delivered first. The Raft node never has a "reorder mode"; it just reacts to whatever arrives, in whatever order the queue produces.

**Fault-injection controls (Kill/Restart/Partition/Heal) are simulator-level, not node-level.** They are never events delivered to a node's `Step()`. A killed node doesn't receive an event telling it "you're dead" — it simply stops being scheduled at all, which is a more faithful model of a real crash than a notification would be (a real crashed process doesn't get a heads-up either). Partition/Heal similarly just mutate a simulator-owned routing table; the node code never needs to know a partition exists — it just stops receiving certain messages and starts again later.

**RNG discipline:** everything nondeterministic — election jitter, latency sampling, drop/duplicate coin-flips — comes from **one explicit `*rand.Rand` instance owned by the simulator**, threaded through every call as an argument. Never Go's package-level `rand.Float64()`/`rand.Intn()` — its state isn't something you fully control from your code, so "the same seed" set globally doesn't guarantee the same output sequence if anything else touches it.

**Go-specific trap:** never range over a `map[int]*Peer` when a leader iterates its peers to send RPCs. Go deliberately randomizes map iteration order between runs — that randomness leaks into `Seq` assignment order and quietly breaks replay even though everything else is deterministic. Keep peer IDs in a sorted slice, iterate that.

**Timer cancellation via generation counters is not optional.** Every timer kind a node owns needs a generation number bumped on every legitimate reset; a fired `EventTimerFire` whose generation doesn't match the node's current one is stale and must be ignored, or a timer from before a reset can fire anyway and trigger a spurious election.

---

## 5. Node Execution Model

```go
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
```

The node **describes** what it wants done (`Outbound` values); only the simulator actually does it. That separation is the whole trick — it's what lets the same `NodeState`/`Step` pair run unmodified under the in-process simulator now and under real goroutines/sockets in the Docker phase later.

---

## 6. Core Types

Two reasonable designs exist for how `RaftMessage` represents different RPC kinds in Go, since Go has no native tagged union:

- **Option A — one struct, a `Kind` tag, all possible fields present** (used below). Simple, easy to serialize, easy to log/replay, some wasted memory per message.
- **Option B — an interface, each message type implementing it, type-switched at the handler.** More idiomatic Go, less wasted memory, more boilerplate.

**Recommendation:** Option A for now — it keeps the `Event`/`Outbound` plumbing simple while you're still building the core logic. Migrating to Option B later (e.g., if you want cleaner `gob`/JSON serialization for the Docker phase) is a mechanical refactor, not a redesign.

```go
package chaos

import "math/rand"

// ---------- Simulated time ----------

// Time is simulated logical time — never wall-clock. Advanced only by the event loop.
type Time uint64

// ---------- Raft message payloads ----------

type MessageKind int

const (
    MsgRequestVote MessageKind = iota
    MsgRequestVoteReply
    MsgAppendEntries
    MsgAppendEntriesReply
)

type LogEntry struct {
    Term    uint64
    Index   uint64
    Command []byte // opaque application payload — Chaos doesn't need to interpret it
}

type RaftMessage struct {
    Kind MessageKind
    Term uint64 // present on every message — lets a handler check "is this stale relative to my term?" uniformly, before even looking at the specific payload

    // RequestVote
    CandidateID  int
    LastLogIndex uint64
    LastLogTerm  uint64

    // RequestVoteReply
    VoteGranted bool

    // AppendEntries
    LeaderID     int
    PrevLogIndex uint64
    PrevLogTerm  uint64
    Entries      []LogEntry
    LeaderCommit uint64

    // AppendEntriesReply
    Success bool
    // Fast backtracking — OPTIONAL, an efficiency optimization, not a
    // correctness requirement. Implement only after the naive
    // one-entry-at-a-time nextIndex backoff already works and is tested.
    ConflictIndex uint64
    ConflictTerm  uint64
}

// ---------- Events (input to a node's Step) ----------

type EventKind int

const (
    EventMessageArrival EventKind = iota
    EventTimerFire
)

type TimerKind int

const (
    TimerElection  TimerKind = iota // followers & candidates: "no leader heard from in time"
    TimerHeartbeat                  // leaders only: "time to prove I'm still alive"
)

type Event struct {
    At     Time
    Seq    uint64 // deterministic tiebreak for equal At — see RNG/ordering discipline
    NodeID int    // which node's Step() receives this event

    Kind EventKind

    // EventMessageArrival
    From    int
    Message *RaftMessage

    // EventTimerFire
    TimerKindField TimerKind
    TimerGen       uint64 // must match the node's current generation for this timer kind, or it's stale
}

// ---------- Outbound (what a node's Step wants the simulator to do) ----------

type OutboundKind int

const (
    OutSendMessage OutboundKind = iota
    OutResetTimer
    OutPersist
    OutApply
)

type Outbound struct {
    Kind OutboundKind

    // OutSendMessage
    To      int
    Message *RaftMessage

    // OutResetTimer — simulator adds Duration to "now" and assigns the
    // resulting event the node's NEW generation for that timer kind.
    TimerKindField TimerKind
    Duration       Time

    // OutPersist — signals that CurrentTerm/VotedFor/Log changed and MUST
    // be durably saved before any OutSendMessage in this same batch is
    // actually delivered. See "the persist-before-respond rule" below.
    // Modeled as an ordering guarantee in the simulator, not real disk I/O.
    PersistedTerm     uint64
    PersistedVotedFor int
    PersistedLogLen   int

    // OutApply — entry has passed CommitIndex; hand it to the state machine.
    ApplyIndex   uint64
    ApplyCommand []byte
}
```

### The persist-before-respond rule (a correctness point the types encode, worth understanding explicitly)

In real Raft, a node must persist any change to `currentTerm`, `votedFor`, or its log **before** sending the RPC response that depends on that change — otherwise a crash between "I decided to grant this vote" and "I actually saved that decision" lets the node vote again for a different candidate in the same term after restart, which directly violates Election Safety. The simulated world doesn't need real disk I/O for this, but the *ordering guarantee* still has to be modeled: the simulator should treat `OutPersist` as happening instantly relative to the simulated clock, but a node's `Step()` implementation should still emit `OutPersist` before the corresponding `OutSendMessage` in its returned slice, and the simulator should enforce that ordering rather than silently letting a send race ahead of a persist. This is exactly the kind of subtle rule that's easy to skip and hard to notice you've skipped — it won't show up until you specifically test "crash right after voting, before the response was sent."

---

## 7. Handler Contracts — `handleMessage` / `handleTimeout`

**Deliberately not implemented here.** This is the actual Raft algorithm — the part you said you want to build yourself, not have generated. What follows is the *contract* each handler must satisfy: the questions it has to answer correctly, in the right order, so that when you sit down to write the Go, you're implementing against a clear specification rather than a blank page.

### `handleMessage(from int, msg *RaftMessage, rng *rand.Rand) []Outbound`

For **every** message kind, in this order:
1. Is `msg.Term` higher than `n.CurrentTerm`? → update `CurrentTerm`, step down to Follower, clear `VotedFor`. This check happens *before* the kind-specific logic below, for every message kind, including `AppendEntries` — a stale leader's heartbeat still needs to trigger step-down if it somehow has a lower term than you.
2. Then branch on `msg.Kind`:
   - **`MsgRequestVote`:** apply the election restriction (Gotcha #1) — compare `LastLogTerm` then `LastLogIndex`. Grant only if the log check passes *and* you haven't already voted for someone else this term. Reset the election timer (`OutResetTimer`) only if you grant the vote.
   - **`MsgRequestVoteReply`:** ignore if you're no longer a Candidate in the term this reply is for (stale). Otherwise count it; on majority, become Leader, immediately send heartbeats to every peer.
   - **`MsgAppendEntries`:** reset the election timer (you heard from a leader). Reject if `msg.Term < n.CurrentTerm`. Check `PrevLogIndex`/`PrevLogTerm` against your own log (log matching property) — reject with conflict info if it doesn't match. On match, append new entries (truncating any conflicting suffix first), then advance your own `CommitIndex` to `min(msg.LeaderCommit, index of last new entry)`.
   - **`MsgAppendEntriesReply`:** ignore if stale (you're no longer leader for that term). On success, update `nextIndex`/`matchIndex` for that follower; recompute whether a new `N` exists such that a majority of `matchIndex[i] >= N` **and** `log[N].Term == CurrentTerm` (Gotcha #2 — the current-term-only check is not optional here) — if so, advance `CommitIndex` to `N`. On failure, back off `nextIndex` (using `ConflictIndex`/`ConflictTerm` if you've implemented the optional fast-backtrack) and retry.

### `handleTimeout(kind TimerKind, rng *rand.Rand) []Outbound`

- **`TimerElection`:** only meaningful if you're Follower or Candidate (a Leader ignores this — it shouldn't even have this timer running; make sure your timer-management logic actually stops it on becoming leader, or you've built a bug where leaders can spuriously start elections against themselves). Increment `CurrentTerm`, vote for self, transition to Candidate, send `RequestVote` to every peer, reset the election timer with a **freshly sampled random duration** (via the passed `rng`, never a fixed value) for the next timeout.
- **`TimerHeartbeat`:** only meaningful if you're Leader. Send `AppendEntries` (possibly empty) to every peer, reset the heartbeat timer.

### A question worth sitting with before you write this

When you sat down to implement `MsgAppendEntriesReply`'s success path, what specifically stops a leader from advancing `CommitIndex` past an old-term entry just because a majority now happens to have replicated it? Write the answer as a code comment directly above wherever you compute the new commit index — future-you debugging a partition-recovery test at 2am will thank present-you.

---

## 8. Development Environment & Workflow

### Repo layout

```
chaos/
├── .claude/
│   └── settings.json       ← attribution disabled (see below); committed, shared
├── .vscode/
│   └── settings.json       ← Go formatting/test defaults
├── docs/
│   └── CHAOS_PROJECT_CONTEXT.md   ← this file
├── internal/
│   ├── raft/                ← NodeState, handleMessage, handleTimeout, types
│   └── sim/                 ← event queue, scheduleDelivery, the simulator loop
├── cmd/
│   └── chaos/                ← main entrypoint, added when there's something to run
├── web/                       ← React/TypeScript visualization — added at Phase 5
├── .gitignore
├── CLAUDE.md                  ← agent-facing instructions (see the note at the top of this doc)
├── README.md
└── go.mod
```

`internal/` is load-bearing, not stylistic: the Go compiler refuses imports of anything under `internal/` from outside this module. That's the mechanism, not just the convention, for marking this as "implementation, not a public API."

### Toolchain

- **Go 1.22+** — the core language (see Decisions Log for why).
- **VS Code** with the Go extension (`golang.go`) — format-on-save and `goimports` on save are configured in `.vscode/settings.json`.
- **Claude Code**, native install, run from inside VS Code (terminal or the VS Code extension) — this is the actual development workflow, not a chat-paste workflow. See the "How I want to work with you" section of `CLAUDE.md` for the specific collaboration rules this implies, particularly around the two Raft gotchas in Section 3.
- **Node.js** — not needed yet; only relevant from Phase 5 onward (the `web/` visualization).
- **Docker** — not needed yet; Phase 8 (optional tier).

### Git and commit attribution

Claude Code's default behavior adds a `Co-Authored-By: Claude` trailer and a `🤖 Generated with Claude Code` line to commits. This is disabled here via `.claude/settings.json`:

```json
{
  "attribution": {
    "commit": "",
    "pr": ""
  }
}
```

reinforced as an explicit instruction in `CLAUDE.md`. Worth remembering precisely what this does and doesn't change: commits are made under your own configured git identity regardless of this setting — Claude Code doesn't commit as a separate account — so this isn't a fix for a real authorship gap. It removes any ambiguity from the commit log itself, which matters more for a project explicitly meant to demonstrate what you personally understood and built.

Commit convention: imperative mood, small atomic commits over one large commit per phase — the commit history is itself part of demonstrating incremental understanding later, not just a mechanism for saving work.

### Build & test

```
go build ./...
go test ./... -race
go vet ./...
```

`-race` specifically is not optional for this project — the entire premise is surfacing concurrency/determinism bugs, so a test run that doesn't check for data races is checking less than half of what actually matters here.

---

## 9. Open Next Steps

In rough order:

1. Implement `handleMessage`/`handleTimeout` against the contracts in Section 7 — this is the actual Raft algorithm, and where the two gotcha check-ins from `CLAUDE.md` apply.
2. Implement the simulator's event loop and `scheduleDelivery` (fault injection point) per Section 4.
3. Write the Figure-8-style partition/commit test from Section 3 as your first real correctness test, before broader random fault injection — it's the one gotcha most likely to hide from you otherwise.
4. Broader property-based fault injection: run many randomized fault scenarios per seed, assert the five safety properties after each.
5. React/TypeScript visualization layer, once the above is provably correct — visualizing a system you haven't verified yet is building on sand.

Everything past that (2PC, Docker, multiplayer, experiment reporting) is optional, in the priority order from Section 2.
