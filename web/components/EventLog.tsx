"use client";

import { useEffect, useRef } from "react";
import { translate, TONE_COLOR } from "@/lib/plainEnglish";

// Every event is shown twice: the plain sentence, then the raw line that
// internal/sim actually emitted, in mono underneath.
//
// That is a deliberate refusal to pick an audience. Showing only "node 5
// became Leader (term 2)" locks out anyone who has not read the Raft
// paper; showing only "Computer 5 is now in charge" throws away the term
// number, which is the single most important quantity in the protocol.
// Stacking them costs one line of vertical space and means the same
// screen works whether or not you know what a term is.
export default function EventLog({ lines }: { lines: string[] }) {
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    endRef.current?.scrollIntoView({ block: "nearest" });
  }, [lines.length]);

  return (
    <div
      className="sunken"
      style={{ flex: 1, minHeight: 0, overflowY: "auto", background: "#fff" }}
    >
      {lines.length === 0 ? (
        <div style={{ padding: "8px 9px", fontSize: 11.5, color: "var(--ink-faint)" }}>
          Nothing has happened yet. Click a computer, open its DO screen, and pick something to try.
        </div>
      ) : (
        <>
          {lines.map((line, i) => {
            const t = translate(line);
            const last = i === lines.length - 1;
            return (
              <div
                key={i}
                style={{
                  display: "flex",
                  gap: 6,
                  padding: "3px 7px 4px",
                  borderBottom: "1px solid #f0eeea",
                  background: last ? "var(--accent-pale)" : "transparent",
                }}
              >
                <span style={{ flex: "none", width: 20, textAlign: "right", fontSize: 10, color: "var(--ink-faint)", paddingTop: 1 }}>
                  {i + 1}
                </span>
                <span
                  style={{
                    flex: "none", width: 7, height: 7, marginTop: 4, borderRadius: "50%",
                    background: TONE_COLOR[t.tone], border: "1px solid rgba(0,0,0,.3)",
                  }}
                />
                <span style={{ minWidth: 0 }}>
                  <span style={{ display: "block", fontSize: 11.5, lineHeight: 1.35, color: "var(--ink)" }}>
                    {t.plain}
                  </span>
                  <span style={{ display: "block", fontFamily: "var(--mono-font)", fontSize: 10, lineHeight: 1.35, color: "var(--ink-faint)" }}>
                    {t.raw}
                  </span>
                </span>
              </div>
            );
          })}
          <div ref={endRef} />
        </>
      )}
    </div>
  );
}
