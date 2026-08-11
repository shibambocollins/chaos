"use client";

import { useCallback, useMemo, useState } from "react";
import { useTracePlayback, type PlaybackRequest } from "@/hooks/useTracePlayback";
import ClusterView from "@/components/ClusterView/ClusterView";
import SimulationControl from "@/components/Timeline/SimulationControl";
import EventLog from "@/components/EventLog";
import { narrationByNode } from "@/lib/narrationByNode";
import { describeState } from "@/lib/plainEnglish";
import { SCENARIOS } from "@/lib/loadTrace";
import type { DeviceAction } from "@/lib/scenarioActions";

// The shell is a fixed-chrome desktop application layout rather than a
// scrolling web page: title bar, workspace, docked panel, controls,
// status bar. That is the point of the visual language. This is a
// network simulator you operate, not a dashboard you read.
//
// There is deliberately no scenario picker up here any more. Choosing
// what happens next is done by clicking a computer and using its DO
// screen, so the gesture matches the thing: you act on a machine.
export default function Home() {
  const [request, setRequest] = useState<PlaybackRequest>({ scenario: "election" });
  const playback = useTracePlayback(request);
  const { trace, loading, error, tick, narrationSoFar, index, maxIndex, playing } = playback;

  const byNode = useMemo(
    () => narrationByNode(narrationSoFar, trace?.meta.nodeCount ?? 0),
    [narrationSoFar, trace],
  );

  const runAction = useCallback((a: DeviceAction) => {
    setRequest({ scenario: a.scenario, seek: a.seek, autoplay: true, nonce: Date.now() });
  }, []);

  const meta = SCENARIOS.find((s) => s.id === request.scenario);
  const leader = tick?.nodes.find((n) => n.alive && n.role === "Leader") ?? null;
  const term = tick ? Math.max(...tick.nodes.map((n) => n.currentTerm)) : 0;
  const total = trace?.meta.nodeCount ?? 0;
  const alive = tick?.nodes.filter((n) => n.alive).length ?? 0;

  return (
    <div className="shell">
      <div className="titlebar">
        <span style={{ fontSize: 11.5, fontWeight: 600 }}>Chaos Raft Cluster Visualizer</span>
        <span style={{ flex: 1 }} />
        <span style={{ opacity: 0.62, fontSize: 10.5 }}>replaying a recorded simulation</span>
      </div>

      {/* Briefing strip: what this recording is, in one sentence, before
          any Raft vocabulary shows up anywhere else on screen. */}
      <div className="toolbar" style={{ minHeight: 0, alignItems: "flex-start", gap: 14, padding: "7px 10px" }}>
        <div style={{ minWidth: 0, flex: 1 }}>
          <div style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
            <span style={{ fontSize: 13, fontWeight: 600 }}>{meta?.plain}</span>
            <span style={{ fontSize: 10.5, color: "var(--ink-faint)", fontFamily: "var(--mono-font)" }}>
              {meta?.label} · traces/{request.scenario}.json
            </span>
          </div>
          <div style={{ fontSize: 11.5, color: "var(--ink-muted)", marginTop: 2, maxWidth: 900 }}>
            {meta?.blurb}
          </div>
        </div>
        <div className="sunken" style={{ flex: "none", padding: "5px 9px", fontSize: 11, color: "var(--ink-muted)", maxWidth: 250 }}>
          <strong style={{ color: "var(--ink)" }}>Try something:</strong> click any computer, open its{" "}
          <strong style={{ color: "var(--ink)" }}>DO</strong> screen, and pick what should happen to the cluster.
        </div>
      </div>

      <div className="workarea">
        <div className="panel">
          <div className="panel-title">
            <span>The cluster</span>
            <span style={{ flex: 1 }} />
            <span style={{ fontWeight: 400, opacity: 0.85 }}>click a computer to inspect or control it</span>
          </div>
          <div className="panel-body" style={{ padding: 3 }}>
            {loading && <Notice>Loading the recording…</Notice>}
            {error && <Notice tone="error">{error}</Notice>}
            {tick && (
              <ClusterView
                tick={tick}
                narrationByNode={byNode}
                activeScenario={request.scenario}
                onAction={runAction}
              />
            )}
          </div>
        </div>

        <div className="panel">
          <div className="panel-title">
            <span>What has happened</span>
            <span style={{ flex: 1 }} />
            <span style={{ fontWeight: 400, opacity: 0.85 }}>{narrationSoFar.length} events</span>
          </div>
          <div className="panel-body" style={{ display: "flex", flexDirection: "column", minHeight: 220 }}>
            <EventLog lines={narrationSoFar} />
          </div>
        </div>
      </div>

      {/* Right now, in one sentence. Describes the state rather than the
          last event, because someone stepping through the timeline wants
          to know where they are, not what just scrolled past. */}
      <div
        className="raised"
        style={{ display: "flex", alignItems: "center", gap: 9, padding: "6px 10px", fontSize: 12 }}
      >
        <Led tone={error ? "down" : leader ? "up" : "busy"} />
        <span style={{ fontWeight: 600, flex: "none" }}>Right now:</span>
        <span style={{ color: "var(--ink-muted)" }}>
          {tick ? describeState(alive, total, leader?.id ?? null) : "Waiting for the recording to load."}
        </span>
      </div>

      <SimulationControl playback={playback} />

      <div className="statusbar">
        <div className="status-cell" style={{ minWidth: 92 }}>
          <Led tone={error ? "down" : playing ? "up" : "idle"} />
          {error ? "Error" : loading ? "Loading" : playing ? "Running" : "Paused"}
        </div>
        <div className="status-cell">
          Computers up: {alive} of {total}
        </div>
        <div className="status-cell">In charge: {leader ? `PC${leader.id}` : "nobody"}</div>
        <div className="status-cell" style={{ fontFamily: "var(--mono-font)" }}>
          term {term}
        </div>
        <div className="status-cell" style={{ flex: 1 }} />
        <div className="status-cell" style={{ fontFamily: "var(--mono-font)" }}>
          tick {tick?.at ?? 0} · frame {index + 1}/{maxIndex + 1}
        </div>
      </div>
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

function Led({ tone }: { tone: "up" | "down" | "busy" | "idle" }) {
  const color =
    tone === "up" ? "var(--up)" : tone === "down" ? "var(--down)" : tone === "busy" ? "var(--busy)" : "#9a958e";
  return (
    <span
      style={{
        display: "block", flex: "none", width: 8, height: 8, borderRadius: "50%",
        background: color, border: "1px solid rgba(0,0,0,.35)",
      }}
    />
  );
}
