// Thin fetch wrappers over chaos-server's HTTP API (internal/server's
// handlers.go). Every call is fire-and-forget from the caller's point of
// view: the server answers 202 Accepted once the action is queued, not
// once it's applied, and the actual effect shows up a moment later over
// the live SSE stream the rest of the app is already subscribed to via
// useLiveCluster. There is no loading state or error banner wired to
// these calls for that reason. The workspace itself is the
// confirmation, the same way flipping a light switch doesn't show you a
// spinner while the room brightens. A failed request is logged, not
// surfaced, so one dropped click doesn't need its own UI.
async function post(baseUrl: string, path: string, body?: unknown): Promise<void> {
  try {
    const res = await fetch(`${baseUrl}${path}`, {
      method: "POST",
      headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
    if (!res.ok) {
      console.error(`chaos-server ${path} responded ${res.status}: ${await res.text()}`);
    }
  } catch (err) {
    console.error(`chaos-server ${path} unreachable:`, err);
  }
}

export function killNode(baseUrl: string, id: number): Promise<void> {
  return post(baseUrl, `/kill/${id}`);
}

export function restartNode(baseUrl: string, id: number): Promise<void> {
  return post(baseUrl, `/restart/${id}`);
}

export function favorNode(baseUrl: string, id: number): Promise<void> {
  return post(baseUrl, `/favor/${id}`);
}

// isolateNode partitions the cluster into {this one node} and {everyone
// else it currently knows about}. allIds must be every configured node
// id, not just the currently alive ones. internal/sim.Partition expects
// a full grouping of the cluster, and quorum math depends on the full
// roster regardless of who's alive.
export function isolateNode(baseUrl: string, id: number, allIds: number[]): Promise<void> {
  const rest = allIds.filter((other) => other !== id);
  return post(baseUrl, "/partition", { groups: [[id], rest] });
}

export function healNetwork(baseUrl: string): Promise<void> {
  return post(baseUrl, "/heal");
}
