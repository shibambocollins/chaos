// Splits the flat narration-so-far list (e.g. "node 2 became Candidate
// (term 3)") out by which node each line is about, for that node's CLI
// readout. Lines that don't start with "node N " (there aren't any today,
// but the format isn't contractually guaranteed) are simply dropped from
// every per-node view, though they still appear in the main EventLog.
export function narrationByNode(lines: string[], nodeCount: number): Map<number, string[]> {
  const map = new Map<number, string[]>();
  for (let id = 1; id <= nodeCount; id++) map.set(id, []);

  for (const line of lines) {
    const m = /^node (\d+) /.exec(line);
    if (!m) continue;
    const id = Number(m[1]);
    const arr = map.get(id);
    if (arr) arr.push(line);
  }

  return map;
}
