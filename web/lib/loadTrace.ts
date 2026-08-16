import type { Trace } from "./trace";

export const SCENARIOS = [
  {
    id: "election",
    label: "Election",
    plain: "Choosing a boss",
    blurb: "Five computers switch on at once. None is in charge, so they vote until exactly one of them wins a majority.",
  },
  {
    id: "partition-heal",
    label: "Partition & Heal",
    plain: "The network splits",
    blurb: "A cable cut leaves two computers on one side and three on the other. Only the bigger side is allowed to keep working, and the smaller side stalls until it is reconnected.",
  },
  {
    id: "kill-restart",
    label: "Kill & Restart",
    plain: "The boss dies",
    blurb: "The computer in charge loses power. The four survivors notice, vote in a replacement, and carry on. When the dead one returns it has to catch up.",
  },
] as const;

export type ScenarioId = (typeof SCENARIOS)[number]["id"];

export async function loadTrace(scenario: ScenarioId): Promise<Trace> {
  const res = await fetch(`${process.env.NEXT_PUBLIC_BASE_PATH ?? ""}/traces/${scenario}.json`);
  if (!res.ok) {
    throw new Error(`failed to load trace "${scenario}": ${res.status} ${res.statusText}`);
  }
  return (await res.json()) as Trace;
}
