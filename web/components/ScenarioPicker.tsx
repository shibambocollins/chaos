"use client";

import { SCENARIOS, type ScenarioId } from "@/lib/loadTrace";

// Styled as a device-palette tray rather than a dropdown: latching
// toolbar buttons with a glyph over a label. A <select> hides the fact
// that there are exactly three curated scenarios; a tray shows all of
// them at once and makes the current one visibly depressed.
export default function ScenarioPicker({
  value,
  onChange,
}: {
  value: ScenarioId;
  onChange: (s: ScenarioId) => void;
}) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 2 }} role="group" aria-label="Scenario">
      {SCENARIOS.map((s) => (
        <button
          key={s.id}
          type="button"
          onClick={() => onChange(s.id)}
          className={`tool${value === s.id ? " on" : ""}`}
          aria-pressed={value === s.id}
          title={HINT[s.id]}
        >
          <ScenarioGlyph id={s.id} active={value === s.id} />
          <span>{SHORT[s.id]}</span>
        </button>
      ))}
    </div>
  );
}

const SHORT: Record<ScenarioId, string> = {
  election: "Election",
  "partition-heal": "Partition",
  "kill-restart": "Kill",
};

const HINT: Record<ScenarioId, string> = {
  election: "Leader election from a cold start",
  "partition-heal": "Split the cluster, then heal it",
  "kill-restart": "Kill a node, then restart it",
};

function ScenarioGlyph({ id, active }: { id: ScenarioId; active: boolean }) {
  const stroke = active ? "var(--accent)" : "var(--ink-muted)";
  const common = { fill: "none", stroke, strokeWidth: 1.3, strokeLinecap: "round" as const };

  if (id === "election") {
    // A node polling its peers: centre plus outbound ballots.
    return (
      <svg width="22" height="18" viewBox="0 0 22 18" aria-hidden>
        <circle cx="11" cy="9" r="3.2" {...common} fill={active ? "var(--accent-pale)" : "none"} />
        <path d="M11 5.8V2M11 12.2V16M7.8 9H3M14.2 9H19" {...common} />
        <circle cx="3" cy="9" r="1.4" fill={stroke} />
        <circle cx="19" cy="9" r="1.4" fill={stroke} />
        <circle cx="11" cy="2" r="1.4" fill={stroke} />
        <circle cx="11" cy="16" r="1.4" fill={stroke} />
      </svg>
    );
  }

  if (id === "partition-heal") {
    // A link severed down the middle.
    return (
      <svg width="22" height="18" viewBox="0 0 22 18" aria-hidden>
        <path d="M2 9h6M14 9h6" {...common} />
        <circle cx="2.5" cy="9" r="1.6" fill={stroke} />
        <circle cx="19.5" cy="9" r="1.6" fill={stroke} />
        <path d="M11.6 2.5 9.4 8l2.8 2-2.6 5.5" {...common} stroke="var(--down)" />
      </svg>
    );
  }

  // Kill & restart: a power symbol.
  return (
    <svg width="22" height="18" viewBox="0 0 22 18" aria-hidden>
      <path d="M6.9 4.6a6 6 0 1 0 8.2 0" {...common} />
      <path d="M11 1.6V8" {...common} stroke="var(--down)" />
    </svg>
  );
}
