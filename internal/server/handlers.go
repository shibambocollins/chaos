package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// NewMux wires every HTTP endpoint to hub and returns a ready-to-serve
// handler. Actions are POST requests, fire-and-forget: 202 Accepted means
// "queued," not "applied" — watch /stream (or poll /state) to see the
// effect. CORS is wide open (Access-Control-Allow-Origin: *) because this
// is a local development tool with one intended client (the Next.js dev
// server on a different port), not a public service.
func NewMux(hub *Hub) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /state", handleState(hub))
	mux.HandleFunc("GET /stream", handleStream(hub))
	mux.HandleFunc("POST /kill/{id}", handleKill(hub))
	mux.HandleFunc("POST /restart/{id}", handleRestart(hub))
	mux.HandleFunc("POST /favor/{id}", handleFavor(hub))
	mux.HandleFunc("POST /partition", handlePartition(hub))
	mux.HandleFunc("POST /heal", handleHeal(hub))
	mux.HandleFunc("POST /client-request", handleClientRequest(hub))
	return withCORS(mux)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func nodeIDFromPath(r *http.Request) (int, error) {
	return strconv.Atoi(r.PathValue("id"))
}

// handleState serves the most recently published snapshot as a one-shot
// GET — a plain poll or a curl check, no event-stream client required.
func handleState(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := hub.Snapshot()
		if data == nil {
			http.Error(w, "no snapshot published yet", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

func handleKill(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := nodeIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid node id", http.StatusBadRequest)
			return
		}
		hub.submit(action{kind: actionKill, nodeID: id})
		w.WriteHeader(http.StatusAccepted)
	}
}

func handleRestart(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := nodeIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid node id", http.StatusBadRequest)
			return
		}
		hub.submit(action{kind: actionRestart, nodeID: id})
		w.WriteHeader(http.StatusAccepted)
	}
}

// handleFavor is the honest substitute for "make this node the leader": it
// takes just enough of the target's rivals offline, temporarily, that it
// only has to out-race the rest for the next election. See Hub.startFavor.
func handleFavor(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := nodeIDFromPath(r)
		if err != nil {
			http.Error(w, "invalid node id", http.StatusBadRequest)
			return
		}
		hub.submit(action{kind: actionFavor, nodeID: id})
		w.WriteHeader(http.StatusAccepted)
	}
}

type partitionRequest struct {
	Groups [][]int `json:"groups"`
}

func handlePartition(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req partitionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Groups) == 0 {
			http.Error(w, `expected JSON body {"groups": [[...],[...]]}`, http.StatusBadRequest)
			return
		}
		hub.submit(action{kind: actionPartition, groups: req.Groups})
		w.WriteHeader(http.StatusAccepted)
	}
}

func handleHeal(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hub.submit(action{kind: actionHeal})
		w.WriteHeader(http.StatusAccepted)
	}
}

type clientRequestBody struct {
	Command string `json:"command"`
}

func handleClientRequest(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req clientRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Command == "" {
			http.Error(w, `expected JSON body {"command": "..."}`, http.StatusBadRequest)
			return
		}
		hub.submit(action{kind: actionClientRequest, command: []byte(req.Command)})
		w.WriteHeader(http.StatusAccepted)
	}
}

// handleStream serves a live text/event-stream of sim.Report snapshots,
// one "data: <json>\n\n" line per change, for as long as the client stays
// connected. This is what lets the browser watch the cluster live.
func handleStream(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		ch := hub.Subscribe()
		defer hub.Unsubscribe(ch)

		for {
			select {
			case data, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}
