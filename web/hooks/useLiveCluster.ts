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
        return;
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
