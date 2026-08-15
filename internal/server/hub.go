// Package server is the live boundary: it hosts one long-running
// internal/sim.Simulator that multiple concurrent HTTP clients act on and
// watch. internal/raft and internal/sim stay exactly as pure and
// single-threaded as everywhere else in this project — every goroutine,
// every use of real wall-clock time, and the one bit of real randomness
// this package needs all live here instead, at the boundary the
// architecture doc describes as "only what sits underneath Step() changes."
package server

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"chaos/internal/raft"
	"chaos/internal/sim"
)

type actionKind int

const (
	actionKill actionKind = iota
	actionRestart
	actionPartition
	actionHeal
	actionClientRequest
	actionFavor
)

// action is a client-submitted command, funneled through Hub's single
// commands channel so the Simulator is only ever touched by the one writer
// goroutine that owns it, no matter how many HTTP requests arrive
// concurrently. Arbitration between conflicting actions (e.g. two players
// racing to act on the same node) is exactly this: whichever action lands
// in the channel first is applied first. Nothing fancier.
type action struct {
	kind    actionKind
	nodeID  int
	groups  [][]int
	command []byte
}

// favorState tracks one in-flight "improve this node's odds" request:
// which of its rivals were taken offline to run it, and when to give up
// and bring them back regardless of whether it worked. Giving up matters —
// the point is a temporary nudge, not a way to strand nodes offline if the
// favored node loses anyway or was already dead.
type favorState struct {
	target   int
	downed   []int
	deadline time.Time
}

// Hub owns the one live Simulator this server process hosts. It's the only
// thing in this codebase that lets more than one goroutine interact with a
// Simulator's world, but the Simulator itself is still only ever touched
// by the single goroutine running Hub.Run. Every action funnels through
// commands and is applied strictly one at a time; every subscriber reads a
// cached, mutex-protected snapshot rather than the live Simulator, so
// nothing outside Run ever races with it.
type Hub struct {
	sim     *sim.Simulator
	monitor *sim.SafetyMonitor
	ids     []int // every node ID in the cluster, sorted, fixed at construction

	commands chan action
	stop     chan struct{}

	mu           sync.Mutex // guards clients and lastSnapshot only — never the Simulator
	clients      map[chan []byte]bool
	lastSnapshot []byte

	eventsProcessed int
	tickIncrement   raft.Time
	tickInterval    time.Duration
	favorDeadline   time.Duration

	// horizon is the simulated-time deadline RunObserving is allowed to
	// process up to. It advances by tickIncrement on every real ticker
	// fire, unconditionally — NOT recomputed from sim.Now(). sim.Now()
	// only advances when the Simulator actually processes an event, so
	// during any quiet gap (e.g. every node mid-election-timeout with
	// nothing due for another 20 ticks), sim.Now()+tickIncrement would
	// stall at the same value forever, never reaching far enough ahead to
	// find the next real event. horizon tracks elapsed real time instead,
	// so it keeps climbing regardless of what the Simulator did last tick.
	horizon raft.Time

	pendingFavor *favorState
}

// NewHub wires a Hub around s. ids is every node ID configured in the
// cluster (not just the currently alive ones) — the Hub needs the full
// roster to know what a quorum actually is; it has no other way to ask s
// for that. tickIncrement simulated ticks are advanced every tickInterval
// of real wall-clock time, which is what makes election timeouts and
// heartbeats "just happen" for connected clients without anyone manually
// stepping the Simulator. favorDeadline caps how long a Favor action is
// allowed to leave nodes down before giving up and restoring them anyway.
func NewHub(s *sim.Simulator, ids []int, tickIncrement raft.Time, tickInterval, favorDeadline time.Duration) *Hub {
	sorted := append([]int(nil), ids...)
	sort.Ints(sorted)
	return &Hub{
		sim:           s,
		monitor:       sim.NewSafetyMonitor(s),
		ids:           sorted,
		commands:      make(chan action, 64),
		stop:          make(chan struct{}),
		clients:       make(map[chan []byte]bool),
		tickIncrement: tickIncrement,
		tickInterval:  tickInterval,
		favorDeadline: favorDeadline,
	}
}

// Run is the single writer goroutine: for as long as it runs, it owns the
// Simulator exclusively, applying commands as they arrive and advancing
// simulated time on a real ticker, publishing a fresh snapshot after every
// change. Meant to be started once, in its own goroutine, by main; Stop
// ends it.
func (h *Hub) Run() {
	ticker := time.NewTicker(h.tickInterval)
	defer ticker.Stop()

	h.publish() // give the very first subscriber something before anything happens

	for {
		select {
		case <-h.stop:
			return
		case act := <-h.commands:
			h.apply(act)
			h.resolveFavor()
			h.publish()
		case <-ticker.C:
			h.horizon += h.tickIncrement
			h.eventsProcessed += h.sim.RunObserving(h.monitor, h.horizon)
			h.resolveFavor()
			h.publish()
		}
	}
}

// Stop ends Run's loop.
func (h *Hub) Stop() {
	close(h.stop)
}

// apply is only ever called from Run's goroutine — the one place in this
// package allowed to touch h.sim/h.monitor directly.
func (h *Hub) apply(act action) {
	switch act.kind {
	case actionKill:
		h.sim.Kill(act.nodeID)
	case actionRestart:
		h.sim.Restart(act.nodeID)
	case actionPartition:
		h.sim.Partition(act.groups)
	case actionHeal:
		h.sim.Heal()
	case actionClientRequest:
		if leader := h.sim.Leader(); leader != nil {
			h.sim.Schedule(raft.Event{
				At: h.sim.Now(), NodeID: leader.ID,
				Kind: raft.EventClientRequest, Command: act.command,
			})
			// Flush the just-scheduled same-tick event immediately rather
			// than waiting for the next real-time ticker fire — a
			// submitted command should feel responsive.
			h.eventsProcessed += h.sim.RunObserving(h.monitor, h.sim.Now())
		}
	case actionFavor:
		h.startFavor(act.nodeID)
	}
	h.monitor.Observe()
}

// startFavor takes just enough of target's live rivals offline that target
// only has to out-race the rest, rather than the whole cluster, for the
// next election — the honest substitute for "assign a leader" a protocol
// like Raft doesn't actually allow one node to do to another. Quorum in
// this implementation is majority-of-configured-cluster regardless of who
// is alive (that's what makes Kill/Restart correctness hold at all), so
// this never downs so many rivals that nobody, including target, could
// still reach quorum.
func (h *Hub) startFavor(target int) {
	if h.pendingFavor != nil {
		h.restoreFavor()
	}
	if !h.sim.IsAlive(target) {
		return // a dead node can't win an election by having rivals downed
	}

	quorum := len(h.ids)/2 + 1
	keepRivals := quorum - 1 // rivals left alive alongside target

	var rivals []int
	for _, id := range h.ids {
		if id != target && h.sim.IsAlive(id) {
			rivals = append(rivals, id)
		}
	}
	if len(rivals) <= keepRivals {
		return // target already faces at most the intended number of live rivals
	}

	downed := append([]int(nil), rivals[keepRivals:]...)
	for _, id := range downed {
		h.sim.Kill(id)
	}
	h.pendingFavor = &favorState{target: target, downed: downed, deadline: time.Now().Add(h.favorDeadline)}
}

// resolveFavor restores whatever startFavor downed, once target has won
// (the favor achieved its goal) or favorDeadline has passed (give up
// rather than strand nodes offline indefinitely). Called after every
// applied action and every tick, from Run's goroutine.
func (h *Hub) resolveFavor() {
	if h.pendingFavor == nil {
		return
	}
	leader := h.sim.Leader()
	won := leader != nil && leader.ID == h.pendingFavor.target
	if won || time.Now().After(h.pendingFavor.deadline) {
		h.restoreFavor()
	}
}

func (h *Hub) restoreFavor() {
	for _, id := range h.pendingFavor.downed {
		h.sim.Restart(id)
	}
	h.pendingFavor = nil
}

// submit enqueues act for the writer goroutine to apply. Safe to call
// concurrently from any number of HTTP handler goroutines — the channel
// send is what actually serializes access to the Simulator.
func (h *Hub) submit(act action) {
	h.commands <- act
}

// Subscribe registers a new listener and returns a channel of
// JSON-encoded sim.Report snapshots pushed to it whenever the cluster
// state changes. The caller must call Unsubscribe when done (e.g. the
// client disconnected).
func (h *Hub) Subscribe() chan []byte {
	ch := make(chan []byte, 8)

	h.mu.Lock()
	h.clients[ch] = true
	initial := h.lastSnapshot
	h.mu.Unlock()

	if initial != nil {
		ch <- initial
	}
	return ch
}

// Unsubscribe removes ch and closes it.
func (h *Hub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
}

// Snapshot returns the most recently published state as JSON, or nil if
// nothing has been published yet (Run hasn't started). Safe to call from
// any goroutine.
func (h *Hub) Snapshot() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastSnapshot
}

// publish builds a fresh snapshot and pushes it to every current
// subscriber. Only ever called from Run's goroutine, that's what makes
// touching h.sim/h.monitor here safe.
func (h *Hub) publish() {
	report := sim.BuildReport(0, h.sim, h.monitor, h.eventsProcessed)
	data, err := json.Marshal(report)
	if err != nil {
		return // Report is always JSON-serializable; defensive only
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastSnapshot = data
	for ch := range h.clients {
		select {
		case ch <- data:
		default:
			// Slow subscriber — drop this update rather than block the
			// single writer goroutine on a stuck client.
		}
	}
}
