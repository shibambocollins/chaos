"use client";

import { useMemo } from "react";
import type { TracePlayback } from "@/hooks/useTracePlayback";
import { translate, TONE_COLOR } from "@/lib/plainEnglish";

export default function SimulationControl({ playback }: { playback: TracePlayback }) {
  const {
    trace, index, maxIndex, playing,
    toggle, stepBack, stepForward, setIndex, reset,
  } = playback;

  const cells = useMemo(() => {
    if (!trace) return [];
    return trace.ticks.map((t) => {
      const lines = t.narration ?? [];
      if (lines.length === 0) return { color: null as string | null, hint: "" };
      const first = translate(lines[0]);
      return {
        color: TONE_COLOR[first.tone],
        hint: lines.map((l) => translate(l).plain).join("\n"),
      };
    });
  }, [trace]);

  const eventCount = cells.filter((c) => c.color).length;

  return (
    <div className="toolbar" style={{ minHeight: 42, gap: 4, alignItems: "stretch" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 3 }}>
        <button className="btn" onClick={reset} title="Go back to the very beginning">
          Restart
        </button>
        <button className="btn" onClick={stepBack} disabled={index === 0} title="Go back one moment">
          &#9664; Back
        </button>
        <button
          className={`btn${playing ? " on" : ""}`}
          onClick={toggle}
          style={{ minWidth: 72 }}
          title={playing ? "Pause automatic playback" : "Run automatically through the recording"}
        >
          {playing ? "Pause" : "Run"}
        </button>
        <button className="btn" onClick={stepForward} disabled={index >= maxIndex} title="Advance one moment">
          Step &#9654;
        </button>
        <button className="btn" onClick={() => setIndex(maxIndex)} disabled={index >= maxIndex} title="Jump to the end">
          End
        </button>
      </div>

      <span className="sep" />

      <div style={{ flex: 1, minWidth: 0, display: "flex", flexDirection: "column", gap: 2, justifyContent: "center" }}>
        <div style={{ display: "flex", justifyContent: "space-between", fontSize: 10, color: "var(--ink-faint)" }}>
          <span>Start</span>
          <span>
            {eventCount} moments where something happened, out of {cells.length} recorded
          </span>
          <span>End</span>
        </div>
        <div className="sunken" style={{ display: "flex", gap: 1, height: 18, padding: 2, background: "#fff" }}>
          {cells.map((c, i) => {
            const current = i === index;
            return (
              <button
                key={i}
                onClick={() => setIndex(i)}
                title={c.hint || `Moment ${i + 1}: nothing changed`}
                aria-label={`Go to moment ${i + 1}`}
                aria-current={current}
                style={{
                  flex: 1, minWidth: 2, padding: 0, cursor: "pointer",
                  background: c.color ?? "#e6e3dd",
                  border: "1px solid transparent",
                  outline: current ? "2px solid var(--ink)" : "none",
                  outlineOffset: current ? "-1px" : undefined,
                  zIndex: current ? 1 : 0,
                }}
              />
            );
          })}
        </div>
      </div>

      <span className="sep" />

      <span style={{ display: "flex", alignItems: "center", fontSize: 11, color: "var(--ink-muted)", minWidth: 92, textAlign: "right" }}>
        Moment {index + 1} of {maxIndex + 1}
      </span>
    </div>
  );
}
