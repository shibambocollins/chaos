package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// NewMux wires every HTTP endpoint to hub and returns a ready-to-serve
// handler. Actions are POST requests (fire-and-forget: 202 Accepted means
// "queued," not "applied" — watch /stream to see the effect). /stream is a
// GET returning a live server-sent-events feed of sim.Report snapshots.
func NewMux(hub *Hub) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /kill/{id}", handleKill(hub))
	mux.HandleFunc("POST /restart/{id}", handleRestart(hub))
	mux.HandleFunc("POST /partition", handlePartition(hub))
	mux.HandleFunc("POST /heal", handleHeal(hub))
	mux.HandleFunc("POST /client-request", handleClientRequest(hub))
	mux.HandleFunc("GET /stream", handleStream(hub))
	return mux
}

func nodeIDFromPath(r *http.Request) (int, error) {
	return strconv.Atoi(r.PathValue("id"))
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

// handleStream serves a live text/event-stream of sim.Report snapshots —
// one "data: <json>\n\n" line per change — for as long as the client stays
// connected. This is the "watch the cluster live" half of multiplayer.
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
