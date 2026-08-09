"use client";

import { SCENARIOS, type ScenarioId } from "@/lib/loadTrace";

export default function ScenarioPicker({
  value,
  onChange,
}: {
  value: ScenarioId;
  onChange: (s: ScenarioId) => void;
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value as ScenarioId)}
      style={{
        padding: "8px 12px",
        borderRadius: 7,
        fontSize: 11,
        fontFamily: "inherit",
        letterSpacing: ".08em",
        border: "1px solid light-dark(#dcdcd8, #262a31)",
        background: "light-dark(#fff, #171b20)",
        color: "light-dark(#24272c, #dde1e7)",
      }}
    >
      {SCENARIOS.map((s) => (
        <option key={s.id} value={s.id}>
          {s.label}
        </option>
      ))}
    </select>
  );
}
