"use client";

import { useCallback, useEffect, useState } from "react";
import { loadTrace, type ScenarioId } from "@/lib/loadTrace";
import type { Trace, TraceTick } from "@/lib/trace";

export interface TracePlayback {
  trace: Trace | null;
  loading: boolean;
  error: string | null;
  index: number;
  maxIndex: number;
  tick: TraceTick | null;
  playing: boolean;
  speed: number; // ticks advanced per real second while playing
  narrationSoFar: string[];
  play: () => void;
  pause: () => void;
  toggle: () => void;
  stepForward: () => void;
  stepBack: () => void;
  setIndex: (i: number) => void;
  setSpeed: (s: number) => void;
  reset: () => void;
}

// useTracePlayback drives a pre-recorded Trace via a scrubber: current
// index, play/pause, speed, and derived per-tick data. There is no live
// simulation here — every tick was already computed by internal/sim and
// exported as JSON; this hook only decides which recorded tick is
// currently in view and how fast to advance through them.
export function useTracePlayback(scenario: ScenarioId): TracePlayback {
  const [trace, setTrace] = useState<Trace | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [index, setIndexState] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(2); // ticks/sec

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    setTrace(null);
    setIndexState(0);
    setPlaying(false);

    loadTrace(scenario)
      .then((t) => {
        if (!cancelled) setTrace(t);
      })
      .catch((err) => {
        if (!cancelled) setError(String(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [scenario]);

  const maxIndex = trace ? trace.ticks.length - 1 : 0;

  const setIndex = useCallback(
    (i: number) => setIndexState(Math.max(0, Math.min(maxIndex, i))),
    [maxIndex],
  );

  const stepForward = useCallback(() => setIndex(index + 1), [index, setIndex]);
  const stepBack = useCallback(() => setIndex(index - 1), [index, setIndex]);
  const reset = useCallback(() => {
    setIndexState(0);
    setPlaying(false);
  }, []);

  const play = useCallback(() => {
    setIndexState((i) => (i >= maxIndex ? 0 : i));
    setPlaying(true);
  }, [maxIndex]);
  const pause = useCallback(() => setPlaying(false), []);
  const toggle = useCallback(() => setPlaying((p) => !p), []);

  useEffect(() => {
    if (!playing || !trace) return;
    const intervalMs = 1000 / Math.max(0.1, speed);
    const id = setInterval(() => {
      setIndexState((i) => {
        if (i >= maxIndex) {
          setPlaying(false);
          return i;
        }
        return i + 1;
      });
    }, intervalMs);
    return () => clearInterval(id);
  }, [playing, speed, trace, maxIndex]);

  const tick = trace ? (trace.ticks[index] ?? null) : null;
  const narrationSoFar = trace
    ? trace.ticks.slice(0, index + 1).flatMap((t) => t.narration ?? [])
    : [];

  return {
    trace,
    loading,
    error,
    index,
    maxIndex,
    tick,
    playing,
    speed,
    narrationSoFar,
    play,
    pause,
    toggle,
    stepForward,
    stepBack,
    setIndex,
    setSpeed,
    reset,
  };
}
