"use client";

import { useState } from "react";
import type { TraceTick } from "@/lib/trace";
import NodeCard, { type NodeKind } from "./NodeCard";

const POS: [number, number][] = [
  [700, 148],
  [1150, 322],
  [975, 616],
  [425, 616],
  [250, 322],
];
const KINDS: NodeKind[] = ["tower", "laptop", "aio", "laptop", "tower"];
const RING: [number, number][] = [
  [0, 1],
  [1, 2],
  [2, 3],
  [3, 4],
  [4, 0],
];

interface Props {
  tick: TraceTick;
  narrationByNode: Map<number, string[]>;
}

// ClusterView lays nodes out in a ring — makes majority/quorum instantly
// legible, since you can see at a glance whether a leader has enough
// reachable neighbors around it.
//
// Known simplification: link "connectivity" here is only "both endpoints
// alive" — the trace doesn't yet record which partition group each node
// was in at each tick, so a partition (nodes alive but unreachable from
// each other) doesn't visually cut a link the way a Kill does. Adding that
// would mean recording group membership per tick in internal/sim's
// TraceRecorder, deliberately deferred alongside packet-in-flight
// animation.
export default function ClusterView({ tick, narrationByNode }: Props) {
  const [focus, setFocus] = useState<number | null>(null);
  const nodes = tick.nodes;
  const leaderIdx = nodes.findIndex((n) => n.alive && n.role === "Leader");

  return (
    <div
      onClick={() => setFocus(null)}
      style={{
        position: "relative",
        maxWidth: 1460,
        margin: "0 auto",
        height: 560,
        border: "1px solid light-dark(#e0e0dc, #1e2228)",
        borderRadius: 16,
        background: "light-dark(#f9f9f7, #0f1216)",
        backgroundImage:
          "radial-gradient(circle at 1px 1px, light-dark(rgba(0,0,0,.05), rgba(255,255,255,.045)) 1px, transparent 0)",
        backgroundSize: "24px 24px",
        overflow: "hidden",
      }}
    >
      <div
        style={{
          position: "absolute",
          left: "50%",
          top: "50%",
          width: 1400,
          height: 800,
          transform: "translate(-50%, -50%) scale(0.68)",
          transformOrigin: "center center",
        }}
      >
        <svg viewBox="0 0 1400 800" width={1400} height={800} style={{ position: "absolute", left: 0, top: 0, zIndex: 1 }}>
          {nodes.map((a, ai) =>
            nodes.slice(ai + 1).map((b, bj) => {
              const bi = ai + 1 + bj;
              const ring = RING.some(([r0, r1]) => (r0 === ai && r1 === bi) || (r0 === bi && r1 === ai));
              const ok = a.alive && b.alive;
              const hot = ok && (leaderIdx === ai || leaderIdx === bi);
              return (
                <line
                  key={`${a.id}-${b.id}`}
                  x1={POS[ai][0]}
                  y1={POS[ai][1]}
                  x2={POS[bi][0]}
                  y2={POS[bi][1]}
                  stroke={
                    !ok
                      ? "oklch(0.55 0.14 25 / .6)"
                      : hot
                        ? "oklch(0.7 0.13 150 / .6)"
                        : "light-dark(rgba(30,34,40,.32), rgba(180,200,225,.28))"
                  }
                  strokeWidth={ring ? 1.6 : 1}
                  strokeDasharray={ok ? "none" : "4 8"}
                  strokeLinecap="round"
                  opacity={ring ? (ok ? 0.85 : 0.6) : ok ? 0.22 : 0.3}
                />
              );
            }),
          )}
        </svg>

        {nodes.map((n, i) => (
          <div
            key={n.id}
            style={{
              position: "absolute",
              left: POS[i][0],
              top: POS[i][1],
              transform: "translate(-50%, -50%)",
              zIndex: focus === i ? 6 : 2,
            }}
          >
            <NodeCard
              node={n}
              kind={KINDS[i]}
              narration={narrationByNode.get(n.id) ?? []}
              focused={focus === i}
              dimmed={focus !== null && focus !== i}
              onFocus={() => setFocus(i)}
            />
          </div>
        ))}
      </div>
    </div>
  );
}
