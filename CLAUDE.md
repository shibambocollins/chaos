# CLAUDE.md — Chaos

## What this project is

A from-scratch Raft implementation running inside a deterministic, replayable simulated network, with fault injection and correctness testing built in from the start. See `docs/CHAOS_PROJECT_CONTEXT.md` for the full architecture, decisions log, Raft correctness reference, and type definitions — **that file is the authoritative spec.** Read it before implementing anything in `internal/raft` or `internal/sim`. If anything here conflicts with it, the context doc wins.

## How I want to work with you

I'm building this to understand distributed systems well enough to explain it in an interview — not to have a working binary appear. That changes how I want you to operate here, specifically:

- Before implementing non-trivial logic — especially anything under `internal/raft` (the handlers in Section 7 of the context doc) — explain your planned approach first and wait for me to confirm, rather than writing the full function unprompted.
- The context doc names two specific correctness gotchas: the election-log-restriction check in `handleRequestVote`, and the current-term-only rule for advancing `CommitIndex` in `handleAppendEntriesReply`. When you reach either one, **stop and ask me to state the rule back to you before you write that branch.** If I can't, explain it, then have me restate it — don't just write the code past that point.
- Prefer small, reviewable diffs. If a single change to core Raft/simulator logic would be more than roughly 40 lines, break it into steps and let me review each one.
- When you finish a chunk, tell me specifically what I should be able to explain about it afterward — not just "done."
- This applies to `internal/raft` and the core of `internal/sim`. Boilerplate, test scaffolding, the frontend (once we're at Phase 5), and config files don't need this ceremony — use judgment.

## Non-negotiable engineering rules

These are correctness requirements from the architecture doc, not style preferences — violating them reintroduces the exact nondeterminism this whole project exists to eliminate.

- No `time.Now()`, no package-level `math/rand`, no goroutines, no real blocking channel sends anywhere inside `internal/raft` or the deterministic core of `internal/sim`. Everything nondeterministic is passed in as an argument.
- Never range over a map (e.g. `map[int]*Peer`) when iterating peers to send RPCs — Go randomizes map iteration order, which silently breaks deterministic replay. Use a sorted slice of peer IDs.
- Every timer kind needs a generation counter, bumped on every legitimate reset. A fired `EventTimerFire` with a stale generation must be ignored.
- A leader may only directly advance `CommitIndex` off a majority-replicated count for entries from its **own current term** — never an older term, even if a majority now happens to have a copy.
- A voter must reject `RequestVote` if the candidate's log is less up-to-date (`LastLogTerm` first, then `LastLogIndex`).
- `OutPersist` must appear before any corresponding `OutSendMessage` in a handler's returned `[]Outbound` slice.
- Kill/Partition/Heal/Restart are simulator-level operations on scheduling and routing — never events delivered to a node's `Step()`. A node never "knows" it's being tested.

## Commit and PR conventions

- **Never add a `Co-Authored-By: Claude` line, a `🤖 Generated with Claude Code` line, or any other Claude/Anthropic attribution to a commit message, PR description, or any git metadata.** This is also enforced via `.claude/settings.json` (`attribution.commit`/`attribution.pr` set to empty strings) — this instruction is the second layer, not the only one.
- This repository shows me as sole author and contributor. No exceptions, regardless of how a change was drafted.
- Write commit messages in imperative mood (`Add election timeout handling`, not `Added` or `Adding`). Explain *why* in the body when the reason isn't obvious from the diff alone.
- Prefer small, atomic commits over one large commit per phase — the commit history is itself part of how I can demonstrate incremental understanding later, so it should read as a real build log, not a single dump.

## Build & test

```
go build ./...
go test ./... -race    # -race is not optional on this project — see the whole reason it exists
go vet ./...
```

## Current phase

Update this section as we move through the roadmap so a new session knows where things stand without me re-explaining.

- [x] Phase 1 — Raft fundamentals
- [ ] Phase 2 — Deterministic simulator architecture + Raft implementation (`internal/raft`, `internal/sim`)
- [ ] Phase 3 — React/TypeScript visualization
- [ ] Phase 4 — Fault injection + correctness tests on the deterministic core
- [ ] Optional: experiment reporting, second protocol (2PC), Docker + real networking, multiplayer — see the context doc's roadmap for priority order if/when we get here
