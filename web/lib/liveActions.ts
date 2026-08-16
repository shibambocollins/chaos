import type { TraceNodeState } from "./trace";

export type LiveActionKind = "kill" | "restart" | "favor" | "isolate" | "heal";

export interface LiveActionDef {
  kind: LiveActionKind;
  cmd: string;
  detail: string;
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
