import type { TraceNodeState } from "./trace";

// The live counterpart to scenarioActions.ts's DEVICE_ACTIONS. Where those
// load a pre-recorded run and jump to a frame, these send a real HTTP
// request to a running chaos-server and do the thing on the spot, see
// lib/liveClient.ts for the actual requests.
//
// Presented as a command list rather than plain-English sentences: this
// screen is the one place in the app where the audience is assumed to
// already know what a partition or a quorum is, since these buttons are
// one keystroke away from Simulator's own method names.
//
// There is deliberately no "make this computer a follower" command. Raft
// doesn't let you assign that any more than it lets you assign a leader,
// every alive node that isn't currently leading or campaigning already is
// one, and killing or restarting whichever node currently leads is what
// puts it back to being one. Two commands cover it, so a third would just
// be redundant.
export type LiveActionKind = "kill" | "restart" | "favor" | "isolate" | "heal";

export interface LiveActionDef {
  kind: LiveActionKind;
  cmd: string; // the command name, as typed at the prompt
  detail: string; // one-line technical note, shown as a trailing comment
  enabled: (node: TraceNodeState) => boolean;
}

export const LIVE_ACTIONS: LiveActionDef[] = [
  {
    kind: "kill",
    cmd: "kill",
    detail: "marks the node down; no graceful shutdown, no event delivered to Step()",
    enabled: (n) => n.alive,
  },
  {
    kind: "restart",
    cmd: "restart",
    detail: "resets volatile state, preserves currentTerm/votedFor/log, fresh election timer",
    enabled: (n) => !n.alive,
  },
  {
    kind: "favor",
    cmd: "favor",
    detail: "downs quorum-1 rivals to shrink the field; not a forced result, Raft has no such primitive",
    enabled: (n) => n.alive,
  },
  {
    kind: "isolate",
    cmd: "partition",
    detail: "splits the cluster into {this node} | {everyone else}; minority side can never reach quorum",
    enabled: (n) => n.alive,
  },
  {
    kind: "heal",
    cmd: "heal",
    detail: "clears the active partition cluster-wide",
    enabled: () => true,
  },
];
