import type { Trace } from "./trace";

// Scenario names must match cmd/chaos's -trace flag values and the
// filenames actually exported into public/traces/.
export const SCENARIOS = [
  { id: "election", label: "Election" },
  { id: "partition-heal", label: "Partition & Heal" },
  { id: "kill-restart", label: "Kill & Restart" },
] as const;

export type ScenarioId = (typeof SCENARIOS)[number]["id"];

export async function loadTrace(scenario: ScenarioId): Promise<Trace> {
  const res = await fetch(`${process.env.NEXT_PUBLIC_BASE_PATH ?? ""}/traces/${scenario}.json`);
  if (!res.ok) {
    throw new Error(`failed to load trace "${scenario}": ${res.status} ${res.statusText}`);
  }
  return (await res.json()) as Trace;
}
