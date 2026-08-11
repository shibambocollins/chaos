"use client";

import { useEffect, useRef, useState } from "react";
import type { TraceNodeState, TraceTick } from "@/lib/trace";
import { reportToTick, snapshotMap, type LiveReport } from "@/lib/liveReport";

export interface LiveCluster {
  tick: TraceTick | null;
  narrationSoFar: string[];
  connected: boolean;
  error: string | null;
}

// useLiveCluster subscribes to a running chaos-server's SSE feed
// (GET /stream) and exposes it in exactly the shape the replay path
// already renders with — ClusterView, NodeCard, and EventLog need zero
// changes to show a live cluster instead of a recorded one.
//
// baseUrl is nullable on purpose: passing null tears down any open
// connection and leaves the hook idle. Without that, switching to Replay
// mode would still leave an EventSource open in the background, quietly
// retrying against a server the user may not even have running.
//
// Narration accumulates for as long as a connection stays open.
// Reconnecting after a drop picks up wherever the server currently is —
// this is a live view of a real process, not a durable log, so whatever
// happened while disconnected is genuinely not recoverable here.
export function useLiveCluster(baseUrl: string | null): LiveCluster {
  const [tick, setTick] = useState<TraceTick | null>(null);
  const [narrationSoFar, setNarrationSoFar] = useState<string[]>([]);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const prevRef = useRef<Map<number, TraceNodeState> | null>(null);

  useEffect(() => {
    prevRef.current = null;
    setTick(null);
    setNarrationSoFar([]);
    setConnected(false);
    setError(null);

    if (!baseUrl) return;

    const source = new EventSource(`${baseUrl}/stream`);

    source.onopen = () => {
      setConnected(true);
      setError(null);
    };
    source.onerror = () => {
      setConnected(false);
      setError(`Can't reach ${baseUrl}. Is chaos-server running?`);
    };
    source.onmessage = (evt) => {
      let report: LiveReport;
      try {
        report = JSON.parse(evt.data);
      } catch {
        return; // a malformed line shouldn't tear down the whole connection
      }
      const next = reportToTick(report, prevRef.current);
      prevRef.current = snapshotMap(next.nodes);
      setTick(next);
      const narration = next.narration;
      if (narration && narration.length > 0) {
        setNarrationSoFar((lines) => lines.concat(narration));
      }
    };

    return () => source.close();
  }, [baseUrl]);

  return { tick, narrationSoFar, connected, error };
}
