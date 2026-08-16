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
