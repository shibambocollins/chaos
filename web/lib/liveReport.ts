import type { Role, TraceNodeState, TraceTick } from "./trace";

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
