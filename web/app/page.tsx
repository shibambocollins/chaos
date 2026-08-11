"use client";

import { useMemo, useState } from "react";
import { useTracePlayback } from "@/hooks/useTracePlayback";
import ClusterView from "@/components/ClusterView/ClusterView";
import Scrubber from "@/components/Timeline/Scrubber";
import EventLog from "@/components/EventLog";
import ScenarioPicker from "@/components/ScenarioPicker";
import { narrationByNode } from "@/lib/narrationByNode";
import { SCENARIOS, type ScenarioId } from "@/lib/loadTrace";

// The shell is deliberately a fixed-chrome desktop application layout —
// title bar, device palette, workspace, docked simulation panel, status
// bar — rather than a scrolling web page. That's the whole point of the
// visual language: this is a network simulator you operate, not a
// dashboard you read.
export default function Home() {
  const [scenario, setScenario] = useState<ScenarioId>("election");
  const playback = useTracePlayback(scenario);
  const { trace, loading, error, tick, narrationSoFar, index, maxIndex, playing } = playback;

  const byNode = useMemo(
    () => narrationByNode(narrationSoFar, trace?.meta.nodeCount ?? 0),
    [narrationSoFar, trace],
  );

  const label = SCENARIOS.find((s) => s.id === scenario)?.label ?? scenario;
  const leader = tick?.nodes.find((n) => n.alive && n.role === "Leader");
  const term = tick ? Math.max(...tick.nodes.map((n) => n.currentTerm)) : 0;
  const alive = tick?.nodes.filter((n) => n.alive).length ?? 0;

  return (
    <div className="shell">
      <div className="titlebar">
        <ClusterIcon />
        <span style={{ fontSize: 11.5, fontWeight: 600 }}>
          Chaos — Raft Cluster Visualizer
        </span>
        <span style={{ opacity: 0.62, fontSize: 11 }}>[{label}]</span>
        <span style={{ flex: 1 }} />
        <span style={{ opacity: 0.62, fontSize: 10.5 }}>deterministic trace replay</span>
      </div>

      <div className="toolbar">
        <ScenarioPicker value={scenario} onChange={setScenario} />
        <span className="sep" />
        <div style={{ display: "flex", flexDirection: "column", gap: 2, padding: "0 8px", fontSize: 11 }}>
          <span style={{ color: "var(--ink-faint)", fontSize: 10 }}>Cluster</span>
          <span style={{ fontFamily: "var(--mono-font)" }}>
            {trace?.meta.nodeCount ?? "—"} nodes · quorum{" "}
            {trace ? Math.floor(trace.meta.nodeCount / 2) + 1 : "—"}
          </span>
        </div>
        <span className="sep" />
        <div style={{ display: "flex", flexDirection: "column", gap: 2, padding: "0 8px", fontSize: 11 }}>
          <span style={{ color: "var(--ink-faint)", fontSize: 10 }}>Source</span>
          <span style={{ fontFamily: "var(--mono-font)" }}>internal/sim · traces/{scenario}.json</span>
        </div>
      </div>

      <div className="workarea">
        <div className="panel">
          <div className="panel-title">
            <span>Logical Workspace</span>
            <span style={{ flex: 1 }} />
            <span style={{ fontWeight: 400, opacity: 0.8 }}>click a device to inspect</span>
          </div>
          <div className="panel-body" style={{ padding: 3 }}>
            {loading && <Notice>Loading trace…</Notice>}
            {error && <Notice tone="error">{error}</Notice>}
            {tick && <ClusterView tick={tick} narrationByNode={byNode} />}
          </div>
        </div>

        <div className="panel">
          <div className="panel-title">
            <span>Simulation Panel</span>
            <span style={{ flex: 1 }} />
            <span style={{ fontWeight: 400, opacity: 0.8 }}>{narrationSoFar.length} events</span>
          </div>
          <div className="panel-body" style={{ display: "flex", flexDirection: "column", gap: 5, minHeight: 220 }}>
            <EventLog lines={narrationSoFar} />
          </div>
        </div>
      </div>

      <Scrubber playback={playback} />

      <div className="statusbar">
        <div className="status-cell" style={{ minWidth: 92 }}>
          <Led tone={error ? "down" : playing ? "up" : "idle"} />
          {error ? "Error" : loading ? "Loading" : playing ? "Running" : "Paused"}
        </div>
        <div className="status-cell">Scenario: {label}</div>
        <div className="status-cell">
          Nodes: {alive}/{trace?.meta.nodeCount ?? 0} up
        </div>
        <div className="status-cell">Term: {term}</div>
        <div className="status-cell">Leader: {leader ? `n${leader.id}` : "none"}</div>
        <div className="status-cell" style={{ flex: 1 }} />
        <div className="status-cell" style={{ fontFamily: "var(--mono-font)" }}>
          Tick {tick?.at ?? 0} · frame {index + 1}/{maxIndex + 1}
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

function Led({ tone }: { tone: "up" | "down" | "idle" }) {
  const color = tone === "up" ? "var(--up)" : tone === "down" ? "var(--down)" : "#9a958e";
  return (
    <span
      style={{
        display: "block",
        width: 7,
        height: 7,
        borderRadius: "50%",
        background: color,
        border: "1px solid rgba(0,0,0,.35)",
      }}
    />
  );
}

function ClusterIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 16 16" aria-hidden>
      <g fill="none" stroke="#fff" strokeWidth="1.1" opacity=".92">
        <path d="M8 3.2 13 8 8 12.8 3 8Z" />
        <circle cx="8" cy="3.2" r="1.5" fill="#fff" stroke="none" />
        <circle cx="13" cy="8" r="1.5" fill="#fff" stroke="none" />
        <circle cx="8" cy="12.8" r="1.5" fill="#fff" stroke="none" />
        <circle cx="3" cy="8" r="1.5" fill="#fff" stroke="none" />
      </g>
    </svg>
  );
}
