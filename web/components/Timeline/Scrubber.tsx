"use client";

import type { CSSProperties } from "react";
import type { TracePlayback } from "@/hooks/useTracePlayback";

const btnStyle: CSSProperties = {
  padding: "6px 10px",
  borderRadius: 7,
  cursor: "pointer",
  border: "1px solid light-dark(#dcdcd8, #262a31)",
  background: "light-dark(#fff, #171b20)",
  color: "light-dark(#24272c, #dde1e7)",
  fontSize: 12,
  fontFamily: "inherit",
};

export default function Scrubber({ playback }: { playback: TracePlayback }) {
  const { index, maxIndex, playing, speed, toggle, stepBack, stepForward, setIndex, setSpeed, reset, tick } = playback;

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 10,
        padding: "10px 14px",
        borderRadius: 10,
        border: "1px solid light-dark(#dcdcd8, #23272e)",
        background: "light-dark(#fff, #14171c)",
      }}
    >
      <button onClick={reset} style={btnStyle} title="restart from tick 0">
        ↺
      </button>
      <button onClick={stepBack} style={btnStyle} disabled={index === 0}>
        ◀
      </button>
      <button onClick={toggle} style={btnStyle}>
        {playing ? "⏸" : "▶"}
      </button>
      <button onClick={stepForward} style={btnStyle} disabled={index >= maxIndex}>
        ▶|
      </button>
      <input
        type="range"
        min={0}
        max={Math.max(0, maxIndex)}
        value={index}
        onChange={(e) => setIndex(Number(e.target.value))}
        style={{ flex: 1 }}
      />
      <span style={{ fontSize: 11, color: "#7c828b", minWidth: 110, textAlign: "right" }}>
        tick {tick?.at ?? 0} · {index + 1}/{maxIndex + 1}
      </span>
      <select
        value={speed}
        onChange={(e) => setSpeed(Number(e.target.value))}
        style={{ fontSize: 11, fontFamily: "inherit", padding: "4px 6px", borderRadius: 6 }}
      >
        <option value={0.5}>0.5×</option>
        <option value={1}>1×</option>
        <option value={2}>2×</option>
        <option value={4}>4×</option>
        <option value={8}>8×</option>
      </select>
    </div>
  );
}
