import type { TraceNodeState } from "./trace";

// The live counterpart to scenarioActions.ts's DEVICE_ACTIONS. Where those
// load a pre-recorded run and jump to a frame, these send a real HTTP
// request to a running chaos-server and do the thing on the spot, see
// lib/liveClient.ts for the actual requests.
//
// There is deliberately no "make this computer a follower" action. Raft
// doesn't let you assign that any more than it lets you assign a leader,
// every alive node that isn't currently leading or campaigning already is
// a follower, and killing or restarting whichever node is currently
// leader is what puts it back to being one. Two actions cover it, so a
// third one would just be redundant.
export type LiveActionKind = "kill" | "restart" | "favor" | "isolate" | "heal";

export interface LiveActionDef {
  kind: LiveActionKind;
  label: string;
  detail: string;
  enabled: (node: TraceNodeState) => boolean;
}

export const LIVE_ACTIONS: LiveActionDef[] = [
  {
    kind: "kill",
    label: "Cut the power to this computer",
    detail: "It stops responding immediately, as if unplugged.",
    enabled: (n) => n.alive,
  },
  {
    kind: "restart",
    label: "Switch this computer back on",
    detail: "It comes back as a follower and catches up on whatever it missed.",
    enabled: (n) => !n.alive,
  },
  {
    kind: "favor",
    label: "Try to put this computer in charge",
    detail: "Takes a couple of its rivals offline for a moment so it only has to out-race the rest, then brings them back. Not guaranteed: Raft doesn't let you assign a leader directly, only improve its odds.",
    enabled: (n) => n.alive,
  },
  {
    kind: "isolate",
    label: "Cut this computer off from the network",
    detail: "It stays powered on but can't reach, or be reached by, any other computer until reconnected.",
    enabled: (n) => n.alive,
  },
  {
    kind: "heal",
    label: "Reconnect the whole network",
    detail: "Undoes any network split currently in effect, for every computer, not just this one.",
    enabled: () => true,
  },
];
