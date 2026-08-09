// Mirrors internal/sim/trace.go's JSON shape exactly. Role stays a plain
// string (matching Go's Role.String() output) rather than a numeric enum —
// same reasoning as the Go side: keep this self-contained, no shared
// marshaling contract to maintain between the two languages beyond "it's
// one of these four strings."
export type Role = "Follower" | "Candidate" | "Leader" | "Unknown";

export interface TraceNodeState {
  id: number;
  alive: boolean;
  role: Role;
  currentTerm: number;
  logLen: number;
  commitIndex: number;
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
