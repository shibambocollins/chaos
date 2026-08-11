import type { Trace } from "./trace";
import type { ScenarioId } from "./loadTrace";

// The actions a device offers when you click it.
//
// The honest framing matters here: these are not live controls. Every
// scenario was already simulated by internal/sim and exported as a fixed
// trace, so "cut the power" cannot kill an arbitrary node on demand.
// What it can do is load the recorded run where that fault happens and
// jump straight to the moment it happens, which is what someone clicking
// the button actually wants to see.
//
// That constraint also decides the wording. Labels say "the leader"
// rather than "computer 5", because which node wins the first election
// is a property of the recorded run, not something the button chooses.
export interface DeviceAction {
  key: string;
  scenario: ScenarioId;
  label: string;
  detail: string;
  // Where to land in the loaded trace. Resolved against the trace once
  // it arrives, rather than hardcoded, so regenerating the traces from
  // cmd/chaos with a different seed cannot silently point these at the
  // wrong frame.
  seek: (t: Trace) => number;
}

export const DEVICE_ACTIONS: DeviceAction[] = [
  {
    key: "boot",
    scenario: "election",
    label: "Switch the cluster on",
    detail: "All five computers start up and hold a vote to choose one boss.",
    seek: () => 0,
  },
  {
    key: "split",
    scenario: "partition-heal",
    label: "Split the network in two",
    detail: "Two computers get cut off from the other three. Watch which side is still allowed to work.",
    seek: (t) => frameLeaderLost(t),
  },
  {
    key: "heal",
    scenario: "partition-heal",
    label: "Reconnect the network",
    detail: "The two sides can talk again, and the cut-off computers catch up on what they missed.",
    seek: (t) => frameLeaderRegained(t),
  },
  {
    key: "kill",
    scenario: "kill-restart",
    label: "Cut the power to the boss",
    detail: "The computer in charge dies. The survivors notice and vote in a replacement.",
    seek: (t) => frameMatching(t, /killed/),
  },
  {
    key: "restart",
    scenario: "kill-restart",
    label: "Switch the boss back on",
    detail: "The dead computer returns, discovers it is no longer in charge, and copies what it missed.",
    seek: (t) => frameMatching(t, /restarted/),
  },
];

function hasLeader(t: Trace, i: number): boolean {
  return t.ticks[i].nodes.some((n) => n.alive && n.role === "Leader");
}

function frameMatching(t: Trace, re: RegExp): number {
  const i = t.ticks.findIndex((tick) => (tick.narration ?? []).some((line) => re.test(line)));
  return i < 0 ? 0 : i;
}

// A network split is not narrated by the trace recorder the way a kill
// is, because no node is told about it. Its first observable symptom is
// that the cluster stops having a leader, so that is what we look for:
// the first frame with no leader after there had been one.
function frameLeaderLost(t: Trace): number {
  let sawLeader = false;
  for (let i = 0; i < t.ticks.length; i++) {
    if (hasLeader(t, i)) sawLeader = true;
    else if (sawLeader) return i;
  }
  return 0;
}

function frameLeaderRegained(t: Trace): number {
  const lost = frameLeaderLost(t);
  if (lost === 0) return 0;
  for (let i = lost; i < t.ticks.length; i++) {
    if (hasLeader(t, i)) return i;
  }
  return t.ticks.length - 1;
}
