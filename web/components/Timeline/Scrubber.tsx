"use client";

import { useMemo } from "react";
import type { TracePlayback } from "@/hooks/useTracePlayback";

// A transport strip: rewind / step / play / step / end, then the
// timeline itself, then speed. The timeline is a sunken well with a tick
// mark over every frame that produced narration, so the interesting
// moments in a replay are visible before you scrub to them — the flat
// stretches between marks are the cluster quietly heartbeating.
export default function Scrubber({ playback }: { playback: TracePlayback }) {
  const {
    trace, index, maxIndex, playing, speed,
    toggle, stepBack, stepForward, setIndex, setSpeed, reset, tick,
  } = playback;

  const marks = useMemo(() => {
    if (!trace || trace.ticks.length < 2) return [];
    const last = trace.ticks.length - 1;
    return trace.ticks
      .map((t, i) => ((t.narration?.length ?? 0) > 0 ? (i / last) * 100 : -1))
      .filter((p) => p >= 0);
  }, [trace]);

  return (
    <div className="toolbar" style={{ minHeight: 34, gap: 3 }}>
      <button className="btn" onClick={reset} title="Rewind to the first frame" aria-label="Rewind">
        |◀
      </button>
      <button className="btn" onClick={stepBack} disabled={index === 0} title="Step back one frame">
        ◀
      </button>
      <button
        className={`btn${playing ? " on" : ""}`}
        onClick={toggle}
        style={{ minWidth: 62 }}
        title={playing ? "Pause replay" : "Play replay"}
      >
        {playing ? "❚❚ Pause" : "▶ Play"}
      </button>
      <button className="btn" onClick={stepForward} disabled={index >= maxIndex} title="Step forward one frame">
        ▶
      </button>
      <button
        className="btn"
        onClick={() => setIndex(maxIndex)}
        disabled={index >= maxIndex}
        title="Jump to the last frame"
      >
        ▶|
      </button>

      <span className="sep" />

      <div className="sunken" style={{ position: "relative", flex: 1, height: 26, padding: "0 6px", background: "#fff" }}>
        <div style={{ position: "absolute", left: 8, right: 8, top: 3, height: 5, pointerEvents: "none" }}>
          {marks.map((pct, i) => (
            <span
              key={i}
              style={{
                position: "absolute",
                left: `${pct}%`,
                width: 1,
                height: 5,
                background: "var(--accent-lit)",
                opacity: 0.65,
              }}
            />
          ))}
        </div>
        <input
          type="range"
          min={0}
          max={Math.max(0, maxIndex)}
          value={index}
          onChange={(e) => setIndex(Number(e.target.value))}
          aria-label="Timeline position"
          style={{ position: "absolute", left: 4, right: 4, bottom: 1, width: "calc(100% - 8px)", margin: 0 }}
        />
      </div>

      <span
        style={{
          minWidth: 116,
          padding: "0 6px",
          textAlign: "right",
          fontFamily: "var(--mono-font)",
          fontSize: 11,
          color: "var(--ink-muted)",
        }}
      >
        t={tick?.at ?? 0} · {index + 1}/{maxIndex + 1}
      </span>

      <span className="sep" />

      <label style={{ display: "flex", alignItems: "center", gap: 4, fontSize: 11, color: "var(--ink-muted)" }}>
        Speed
        <select value={speed} onChange={(e) => setSpeed(Number(e.target.value))}>
          <option value={0.5}>0.5×</option>
          <option value={1}>1×</option>
          <option value={2}>2×</option>
          <option value={4}>4×</option>
          <option value={8}>8×</option>
        </select>
      </label>
    </div>
  );
}
