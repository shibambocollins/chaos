"use client";

import { useCallback, useMemo, useState } from "react";
import { useTracePlayback, type PlaybackRequest } from "@/hooks/useTracePlayback";
import { useLiveCluster } from "@/hooks/useLiveCluster";
import ClusterView from "@/components/ClusterView/ClusterView";
import SimulationControl from "@/components/Timeline/SimulationControl";
import EventLog from "@/components/EventLog";
import HowItWorks from "@/components/HowItWorks";
import { narrationByNode } from "@/lib/narrationByNode";
import { SCENARIOS } from "@/lib/loadTrace";
import type { DeviceAction } from "@/lib/scenarioActions";
import type { LiveActionKind } from "@/lib/liveActions";
import { killNode, restartNode, favorNode, isolateNode, healNetwork } from "@/lib/liveClient";

const LIVE_SERVER_URL = process.env.NEXT_PUBLIC_LIVE_SERVER_URL ?? "http://localhost:8080";

type Mode = "replay" | "live";

// The shell is a fixed-chrome desktop application layout rather than a
// scrolling web page: title bar, workspace, docked panel, controls,
// status bar. That is the point of the visual language. This is a
// network simulator you operate, not a dashboard you read.
//
// Two modes share the same workspace. Replay plays back a recorded
// internal/sim run (the original design); Live watches an actual running
// chaos-server process in real time. They deliberately share every
// visual component unchanged. ClusterView and NodeCard have no idea
// which one is feeding them, because both ultimately hand them the same
// TraceTick shape (see lib/liveReport.ts for the live side of that).
// NodeCard's DO screen shows a completely different button list per
// mode instead: Live's buttons send real HTTP requests to a running
// chaos-server (see lib/liveClient.ts) and the effect shows up over the
// SSE stream a moment later, rather than jumping to a recorded frame.
export default function Home() {
  const [mode, setMode] = useState<Mode>("replay");
  const [request, setRequest] = useState<PlaybackRequest>({ scenario: "election" });
  const [helpOpen, setHelpOpen] = useState(false);

  const playback = useTracePlayback(request);
  const live = useLiveCluster(mode === "live" ? LIVE_SERVER_URL : null);

  const isLive = mode === "live";
  const tick = isLive ? live.tick : playback.tick;
  const narrationSoFar = isLive ? live.narrationSoFar : playback.narrationSoFar;
  const loading = isLive ? !live.connected && !live.error : playback.loading;
  const error = isLive ? live.error : playback.error;

  const nodeCount = tick?.nodes.length ?? 0;
  const byNode = useMemo(() => narrationByNode(narrationSoFar, nodeCount), [narrationSoFar, nodeCount]);

  const runAction = useCallback((a: DeviceAction) => {
    setMode("replay");
    setRequest({ scenario: a.scenario, seek: a.seek, autoplay: true, nonce: Date.now() });
  }, []);

  // The five live actions all resolve to one HTTP call each against the
  // running chaos-server. isolate needs every configured node id (not
  // just the currently alive ones) to build a correct partition, which
  // is why it reads from the current tick rather than taking a fixed
  // roster, since the cluster size isn't hardcoded into the frontend anywhere
  // else either.
  const doLiveAction = useCallback(
    (nodeId: number, kind: LiveActionKind) => {
      switch (kind) {
        case "kill":
          killNode(LIVE_SERVER_URL, nodeId);
          break;
        case "restart":
          restartNode(LIVE_SERVER_URL, nodeId);
          break;
        case "favor":
          favorNode(LIVE_SERVER_URL, nodeId);
          break;
        case "isolate":
          isolateNode(LIVE_SERVER_URL, nodeId, live.tick?.nodes.map((n) => n.id) ?? []);
          break;
        case "heal":
          healNetwork(LIVE_SERVER_URL);
          break;
      }
    },
    [live.tick],
  );

  const meta = SCENARIOS.find((s) => s.id === request.scenario);
  const leader = tick?.nodes.find((n) => n.alive && n.role === "Leader") ?? null;
  const term = tick ? Math.max(...tick.nodes.map((n) => n.currentTerm)) : 0;
  const alive = tick?.nodes.filter((n) => n.alive).length ?? 0;

  return (
    <div className="shell">
      <div className="titlebar">
        <span style={{ fontSize: 11.5, fontWeight: 600 }}>Chaos Raft Cluster Visualizer</span>
        <span style={{ opacity: 0.55, fontSize: 10.5 }}>
          &middot; {isLive ? "Live" : meta?.label}
        </span>
        <span style={{ flex: 1 }} />
        <ModeSwitch mode={mode} onChange={setMode} />
        <button
          onClick={() => setHelpOpen(true)}
          style={{
            cursor: "pointer", color: "#fff", background: "rgba(255,255,255,.14)",
            border: "1px solid rgba(255,255,255,.4)", borderRadius: 2,
            font: "inherit", fontSize: 10.5, padding: "2px 8px", marginLeft: 8,
          }}
        >
          How this works
        </button>
      </div>

      <div className="workarea">
        <div className="panel">
          <div className="panel-title">
            <span>The cluster</span>
            <span style={{ flex: 1 }} />
            <span style={{ fontWeight: 400, opacity: 0.85 }}>click a computer to inspect or control it</span>
          </div>
          <div className="panel-body" style={{ padding: 3 }}>
            {loading && <Notice>{isLive ? "Connecting to chaos-server…" : "Loading the recording…"}</Notice>}
            {error && (
              <Notice tone="error">
                {error}
                {isLive && (
                  <>
                    {" "}Start it with <code style={{ fontFamily: "var(--mono-font)" }}>go run ./cmd/chaos-server</code>, then this reconnects on its own.
                  </>
                )}
              </Notice>
            )}
            {tick && (
              <ClusterView
                tick={tick}
                narrationByNode={byNode}
                mode={mode}
                activeScenario={isLive ? "election" : request.scenario}
                onAction={runAction}
                onLiveAction={doLiveAction}
              />
            )}
          </div>
        </div>

        <div className="panel">
          <div className="panel-title">
            <span>Logs</span>
            <span style={{ flex: 1 }} />
            <span style={{ fontWeight: 400, opacity: 0.85 }}>{narrationSoFar.length} events</span>
          </div>
          <div className="panel-body" style={{ display: "flex", flexDirection: "column", minHeight: 220 }}>
            <EventLog lines={narrationSoFar} />
          </div>
        </div>
      </div>

      {isLive ? (
        <LiveStatusStrip connected={live.connected} url={LIVE_SERVER_URL} />
      ) : (
        <SimulationControl playback={playback} />
      )}

      <div className="statusbar">
        <div className="status-cell" style={{ minWidth: 92 }}>
          <Led tone={error ? "down" : isLive ? (live.connected ? "up" : "idle") : playback.playing ? "up" : "idle"} />
          {isLive
            ? live.connected ? "Live" : "Disconnected"
            : error ? "Error" : loading ? "Loading" : playback.playing ? "Running" : "Paused"}
        </div>
        <div className="status-cell">
          Computers up: {alive} of {nodeCount || (isLive ? 5 : 0)}
        </div>
        <div className="status-cell">In charge: {leader ? `PC${leader.id}` : "nobody"}</div>
        <div className="status-cell" style={{ fontFamily: "var(--mono-font)" }}>
          term {term}
        </div>
        <div className="status-cell" style={{ flex: 1 }} />
        {!isLive && (
          <div className="status-cell" style={{ fontFamily: "var(--mono-font)" }}>
            tick {tick?.at ?? 0} · frame {playback.index + 1}/{playback.maxIndex + 1}
          </div>
        )}
      </div>

      <HowItWorks open={helpOpen} onClose={() => setHelpOpen(false)} />
    </div>
  );
}

function ModeSwitch({ mode, onChange }: { mode: Mode; onChange: (m: Mode) => void }) {
  const btn = (m: Mode, label: string) => (
    <button
      onClick={() => onChange(m)}
      aria-pressed={mode === m}
      style={{
        cursor: "pointer", font: "inherit", fontSize: 10.5, padding: "2px 9px",
        color: mode === m ? "var(--accent)" : "#fff",
        background: mode === m ? "#fff" : "rgba(255,255,255,.14)",
        border: "1px solid rgba(255,255,255,.4)",
        borderRadius: m === "replay" ? "2px 0 0 2px" : "0 2px 2px 0",
        marginLeft: m === "live" ? -1 : 0,
      }}
    >
      {label}
    </button>
  );
  return (
    <span>
      {btn("replay", "Replay")}
      {btn("live", "Live")}
    </span>
  );
}

function LiveStatusStrip({ connected, url }: { connected: boolean; url: string }) {
  return (
    <div className="toolbar" style={{ minHeight: 34, alignItems: "center", gap: 8, padding: "0 10px" }}>
      <Led tone={connected ? "up" : "idle"} />
      <span style={{ fontSize: 11.5, color: "var(--ink)" }}>
        {connected ? "Watching a live cluster" : "Waiting for a live cluster"}
      </span>
      <span style={{ fontSize: 10.5, color: "var(--ink-faint)", fontFamily: "var(--mono-font)" }}>{url}</span>
      <span style={{ flex: 1 }} />
      <span style={{ fontSize: 10.5, color: "var(--ink-faint)" }}>
        Open a computer&apos;s DO screen to act on the real, running cluster.
      </span>
    </div>
  );
}

function Notice({ children, tone }: { children: React.ReactNode; tone?: "error" }) {
  return (
    <div
      style={{
        margin: 8,
        padding: "6px 9px",
        fontSize: 11.5,
        color: tone === "error" ? "var(--down)" : "var(--ink-muted)",
        background: tone === "error" ? "var(--down-pale)" : "var(--chrome-sunken)",
        border: `1px solid ${tone === "error" ? "var(--down)" : "var(--bevel-dark)"}`,
      }}
    >
      {children}
    </div>
  );
}

function Led({ tone }: { tone: "up" | "down" | "idle" }) {
  const color = tone === "up" ? "var(--up)" : tone === "down" ? "var(--down)" : "#9a958e";
  return (
    <span
      style={{
        display: "block", flex: "none", width: 8, height: 8, borderRadius: "50%",
        background: color, border: "1px solid rgba(0,0,0,.35)",
      }}
    />
  );
}
