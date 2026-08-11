"use client";

import { useEffect, useRef } from "react";

// The simulation panel's event list: oldest-first, auto-scrolled to the
// newest line, with a status dot per row. Ordering matters more than it
// looks — a replay is a story, and reading it top-to-bottom in the order
// the simulator produced the events is the point. The dot classifies the
// line by keyword so a run's shape (steady green heartbeat, a burst of
// amber elections, a red fault) is legible before you read any words.
export default function EventLog({ lines }: { lines: string[] }) {
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    endRef.current?.scrollIntoView({ block: "nearest" });
  }, [lines.length]);

  return (
    <div
      className="sunken"
      style={{
        flex: 1,
        minHeight: 0,
        overflowY: "auto",
        background: "#fff",
        fontFamily: "var(--mono-font)",
        fontSize: 11,
        lineHeight: 1.5,
      }}
    >
      {lines.length === 0 ? (
        <div style={{ padding: "7px 8px", color: "var(--ink-faint)" }}>
          No events captured yet — press Play.
        </div>
      ) : (
        <>
          {lines.map((line, i) => (
            <div
              key={i}
              style={{
                display: "flex",
                gap: 6,
                padding: "1px 6px 2px",
                alignItems: "baseline",
                background: i === lines.length - 1 ? "var(--accent-pale)" : i % 2 ? "#f6f7f8" : "transparent",
              }}
            >
              <span style={{ flex: "none", width: 26, textAlign: "right", color: "var(--ink-faint)" }}>
                {i + 1}
              </span>
              <span
                style={{
                  flex: "none",
                  width: 6,
                  height: 6,
                  marginTop: 4,
                  borderRadius: "50%",
                  background: toneOf(line),
                  border: "1px solid rgba(0,0,0,.3)",
                }}
              />
              <span style={{ color: "var(--ink)" }}>{line}</span>
            </div>
          ))}
          <div ref={endRef} />
        </>
      )}
    </div>
  );
}

function toneOf(line: string): string {
  if (/\b(leader|committed|elected|healed|restart)/i.test(line)) return "var(--up)";
  if (/\b(candidate|election|vote|term)/i.test(line)) return "var(--busy)";
  if (/\b(kill|down|partition|drop|lost|fail|stale|reject)/i.test(line)) return "var(--down)";
  return "#9a958e";
}
