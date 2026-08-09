// Package server is the live multiplayer boundary: it hosts one long-running
// internal/sim.Simulator that multiple concurrent HTTP clients act on and
// watch. internal/raft and internal/sim stay exactly as pure and
// single-threaded as everywhere else in this project — every goroutine and
// every use of real wall-clock time lives here instead, at the boundary the
// architecture doc describes as "only what sits underneath Step() changes."
package server

import (
	"encoding/json"
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
)

// action is a client-submitted command, funneled through Hub's single
// commands channel so the Simulator is only ever touched by the one writer
// goroutine that owns it — no matter how many HTTP requests arrive
// concurrently. Arbitration between conflicting actions (e.g. two players
// racing to act on the same node) is exactly this: whichever action lands
// in the channel first is applied first. Nothing fancier.
type action struct {
	kind    actionKind
	nodeID  int
	groups  [][]int
	command []byte
}

// Hub owns the one live Simulator this server process hosts. It's the only
// thing in this codebase that lets more than one goroutine interact with a
// Simulator's world — but the Simulator itself is still only ever touched
// by the single goroutine running Hub.Run. Every action funnels through
// commands and is applied strictly one at a time; every subscriber reads a
// cached, mutex-protected snapshot rather than the live Simulator, so
// nothing outside Run ever races with it.
type Hub struct {
	sim     *sim.Simulator
	monitor *sim.SafetyMonitor

	commands chan action
	stop     chan struct{}

	mu           sync.Mutex // guards clients and lastSnapshot only — never the Simulator
	clients      map[chan []byte]bool
	lastSnapshot []byte

	eventsProcessed int
	tickIncrement   raft.Time
	tickInterval    time.Duration

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
}

// NewHub wires a Hub around s. tickIncrement simulated ticks are advanced
// every tickInterval of real wall-clock time — this is what makes election
// timeouts and heartbeats "just happen" for connected clients without
// anyone manually stepping the Simulator.
func NewHub(s *sim.Simulator, tickIncrement raft.Time, tickInterval time.Duration) *Hub {
	return &Hub{
		sim:           s,
		monitor:       sim.NewSafetyMonitor(s),
		commands:      make(chan action, 64),
		stop:          make(chan struct{}),
		clients:       make(map[chan []byte]bool),
		tickIncrement: tickIncrement,
		tickInterval:  tickInterval,
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
			h.publish()
		case <-ticker.C:
			h.horizon += h.tickIncrement
			h.eventsProcessed += h.sim.RunObserving(h.monitor, h.horizon)
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
	}
	h.monitor.Observe()
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

// publish builds a fresh snapshot and pushes it to every current
// subscriber. Only ever called from Run's goroutine — that's what makes
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
