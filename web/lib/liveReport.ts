import type { Role, TraceNodeState, TraceTick } from "./trace";

// Mirrors internal/sim.Report and NodeSummary's JSON shape exactly, Go's
// default field-name-as-key encoding, since that type carries no json
// struct tags (unlike TraceTick/TraceNodeState in internal/sim/trace.go,
// which do, and which this file deliberately does NOT duplicate, see
// reportToTick below).
export interface LiveNodeSummary {
  ID: number;
  Alive: boolean;
  Role: Role;
  CurrentTerm: number;
  LogLen: number;
  CommitIndex: number;
  AppliedCount: number;
  Group: number;
}

export interface LiveViolation {
  Property: string;
  Detail: string;
}

export interface LiveReport {
  Seed: number;
  EventsProcessed: number;
  FinalTick: number;
  Nodes: LiveNodeSummary[];
  Violations: LiveViolation[];
}

function toTraceNode(n: LiveNodeSummary): TraceNodeState {
  return {
    id: n.ID,
    alive: n.Alive,
    role: n.Role,
    currentTerm: n.CurrentTerm,
    logLen: n.LogLen,
    commitIndex: n.CommitIndex,
    group: n.Group,
  };
}

// diffNarration is a line-for-line port of internal/sim/trace.go's
// function of the same name: same priority order (alive/dead first, since
// it makes everything else about that node moot), same wording. Keeping
// it identical is what lets every component that already reads narration
// lines (EventLog, NodeCard's LOG tab, lib/plainEnglish's translate) work
// unchanged whether a line came from a recorded trace or a live snapshot
// From their side, "node 5 became Leader (term 2)" doesn't say which.
function diffNarration(prev: TraceNodeState, cur: TraceNodeState): string[] {
  if (prev.alive && !cur.alive) {
    return [`node ${cur.id} killed`];
  }

  const lines: string[] = [];
  if (!prev.alive && cur.alive) {
    lines.push(`node ${cur.id} restarted`);
  }
  if (cur.alive && prev.role !== cur.role) {
    lines.push(`node ${cur.id} became ${cur.role} (term ${cur.currentTerm})`);
  }
  if (cur.alive && cur.commitIndex > prev.commitIndex) {
    lines.push(`node ${cur.id} committed index ${cur.commitIndex}`);
  }
  if (cur.alive && prev.group !== cur.group) {
    lines.push(
      cur.group === -1
        ? `node ${cur.id} reconnected to the rest of the cluster`
        : `node ${cur.id} lost contact with part of the cluster`,
    );
  }
  return lines;
}

// reportToTick converts one live snapshot into the same TraceTick shape
// the replay path already renders, diffing against whatever the previous
// snapshot looked like (by node id) to derive narration the same way
// internal/sim/trace.go's TraceRecorder does server-side. This is the
// entire point of this file: ClusterView, NodeCard, and EventLog need to
// know nothing about where a tick came from.
export function reportToTick(report: LiveReport, prev: Map<number, TraceNodeState> | null): TraceTick {
  const nodes = report.Nodes.map(toTraceNode).sort((a, b) => a.id - b.id);

  let narration: string[] = [];
  if (prev) {
    for (const n of nodes) {
      const p = prev.get(n.id);
      if (p) narration = narration.concat(diffNarration(p, n));
    }
  }

  return { at: report.FinalTick, nodes, narration: narration.length ? narration : undefined };
}

export function snapshotMap(nodes: TraceNodeState[]): Map<number, TraceNodeState> {
  return new Map(nodes.map((n) => [n.id, n]));
}
