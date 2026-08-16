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

export function isolateNode(baseUrl: string, id: number, allIds: number[]): Promise<void> {
  const rest = allIds.filter((other) => other !== id);
  return post(baseUrl, "/partition", { groups: [[id], rest] });
}

export function healNetwork(baseUrl: string): Promise<void> {
  return post(baseUrl, "/heal");
}
