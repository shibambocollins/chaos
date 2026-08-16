export type Role = "Follower" | "Candidate" | "Leader" | "Unknown";

export interface TraceNodeState {
  id: number;
  alive: boolean;
  role: Role;
  currentTerm: number;
  logLen: number;
  commitIndex: number;
  group: number;
}

export interface TraceTick {
  at: number;
  nodes: TraceNodeState[];
  narration?: string[];
}

export interface TraceMeta {
  nodeCount: number;
  scenario: string;
}

export interface Trace {
  meta: TraceMeta;
  ticks: TraceTick[];
}
