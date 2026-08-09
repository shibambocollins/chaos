"use client";

import { useMemo, useState } from "react";
import { useTracePlayback } from "@/hooks/useTracePlayback";
import ClusterView from "@/components/ClusterView/ClusterView";
import Scrubber from "@/components/Timeline/Scrubber";
import EventLog from "@/components/EventLog";
import ScenarioPicker from "@/components/ScenarioPicker";
import { narrationByNode } from "@/lib/narrationByNode";
import type { ScenarioId } from "@/lib/loadTrace";

export default function Home() {
  const [scenario, setScenario] = useState<ScenarioId>("election");
  const playback = useTracePlayback(scenario);
  const { trace, loading, error, tick, narrationSoFar } = playback;

  const byNode = useMemo(
    () => narrationByNode(narrationSoFar, trace?.meta.nodeCount ?? 0),
    [narrationSoFar, trace],
  );

  return (
    <div
      style={{
        minHeight: "100vh",
        boxSizing: "border-box",
        padding: "26px 30px 40px",
        fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          flexWrap: "wrap",
          gap: 14,
          margin: "0 auto 18px",
          maxWidth: 1460,
        }}
      >
        <div style={{ display: "flex", alignItems: "baseline", gap: 14 }}>
          <span style={{ font: "600 15px/1 inherit", letterSpacing: ".3em" }}>CHAOS</span>
          <span style={{ fontSize: 11, letterSpacing: ".16em", color: "#6c747f" }}>
            raft cluster · {trace?.meta.nodeCount ?? "?"} nodes · deterministic sim replay
          </span>
        </div>
        <ScenarioPicker value={scenario} onChange={setScenario} />
      </div>

      <div style={{ maxWidth: 1460, margin: "0 auto", display: "flex", flexDirection: "column", gap: 14 }}>
        {loading && <div style={{ fontSize: 12, color: "#7c828b" }}>loading trace…</div>}
        {error && <div style={{ fontSize: 12, color: "oklch(0.68 0.19 25)" }}>{error}</div>}
        {tick && (
          <>
            <ClusterView tick={tick} narrationByNode={byNode} />
            <Scrubber playback={playback} />
            <EventLog lines={narrationSoFar} />
          </>
        )}
      </div>
    </div>
  );
}
