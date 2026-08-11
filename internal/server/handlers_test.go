package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"chaos/internal/sim"
)

// readStreamUntil reads SSE "data: <json>\n\n" lines from body until cond
// reports true against the decoded sim.Report, or the context expires.
func readStreamUntil(t *testing.T, ctx context.Context, body *bufio.Reader, cond func(sim.Report) bool) {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("stream context expired before observing expected condition")
		default:
		}

		line, err := body.ReadString('\n')
		if err != nil {
			t.Fatalf("stream ended before observing expected condition: %v", err)
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var report sim.Report
		if jsonErr := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &report); jsonErr != nil {
			continue
		}
		if cond(report) {
			return
		}
	}
}

func TestHandlers_KillViaHTTPReflectedInStream(t *testing.T) {
	hub := newTestHub(t)
	srv := httptest.NewServer(NewMux(hub))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/kill/2", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /kill/2: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/stream", nil)
	streamResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /stream: %v", err)
	}
	defer streamResp.Body.Close()

	readStreamUntil(t, ctx, bufio.NewReader(streamResp.Body), func(r sim.Report) bool {
		return !nodeAlive(r, 2)
	})
}

func TestHandlers_KillWithBadNodeIDReturns400(t *testing.T) {
	hub := newTestHub(t)
	srv := httptest.NewServer(NewMux(hub))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/kill/not-a-number", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /kill/not-a-number: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandlers_PartitionWithEmptyBodyReturns400(t *testing.T) {
	hub := newTestHub(t)
	srv := httptest.NewServer(NewMux(hub))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/partition", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST /partition: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty groups, got %d", resp.StatusCode)
	}
}

func TestHandlers_ClientRequestViaHTTPEventuallyApplied(t *testing.T) {
	hub := newTestHub(t)
	srv := httptest.NewServer(NewMux(hub))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/stream", nil)
	streamResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /stream: %v", err)
	}
	defer streamResp.Body.Close()
	body := bufio.NewReader(streamResp.Body)

	readStreamUntil(t, ctx, body, hasLeader)

	resp, err := http.Post(srv.URL+"/client-request", "application/json", strings.NewReader(`{"command":"x=1"}`))
	if err != nil {
		t.Fatalf("POST /client-request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	readStreamUntil(t, ctx, body, func(r sim.Report) bool {
		for _, n := range r.Nodes {
			if n.AppliedCount > 0 {
				return true
			}
		}
		return false
	})
}

func TestHandlers_FavorViaHTTPDownsExactlyTwoRivals(t *testing.T) {
	hub := newTestHubN(t, 5, 300*time.Millisecond)
	srv := httptest.NewServer(NewMux(hub))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/stream", nil)
	streamResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /stream: %v", err)
	}
	defer streamResp.Body.Close()
	body := bufio.NewReader(streamResp.Body)

	resp, err := http.Post(srv.URL+"/favor/4", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /favor/4: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	readStreamUntil(t, ctx, body, func(r sim.Report) bool {
		alive := 0
		for _, n := range r.Nodes {
			if n.Alive {
				alive++
			}
		}
		return alive == 3 && nodeAlive(r, 4)
	})
}

func TestHandlers_StateReturnsCurrentSnapshot(t *testing.T) {
	hub := newTestHub(t)
	srv := httptest.NewServer(NewMux(hub))
	defer srv.Close()

	// Give Run's first publish() a moment to land.
	time.Sleep(20 * time.Millisecond)

	resp, err := http.Get(srv.URL + "/state")
	if err != nil {
		t.Fatalf("GET /state: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var report sim.Report
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		t.Fatalf("decode /state body: %v", err)
	}
	if len(report.Nodes) != 3 {
		t.Fatalf("expected 3 nodes in snapshot, got %d", len(report.Nodes))
	}
}

func TestHandlers_CORSHeaderIsPresent(t *testing.T) {
	hub := newTestHub(t)
	srv := httptest.NewServer(NewMux(hub))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/state")
	if err != nil {
		t.Fatalf("GET /state: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin: *, got %q", got)
	}
}
