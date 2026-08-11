"use client";

import { useLayoutEffect, useRef, useState } from "react";
import type { TraceTick } from "@/lib/trace";
import NodeCard, { type NodeKind } from "./NodeCard";

const W = 1400;
const H = 800;

// The ring occupies well less than the authored space — the devices span
// roughly 1180×745 around the same centre. Fitting to the content box
// rather than to W×H is the difference between the cluster filling the
// workspace and floating in the middle of it.
const CONTENT_W = 1180;
const CONTENT_H = 745;

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
// Link rendering follows network-diagram convention rather than anything
// decorative: a solid line for a working link, a dashed red one for a
// broken link, and a small LED at each endpoint showing that end's
// status. That's the same vocabulary a network simulator uses, and it
// carries the same information the old glow did without competing with
// the device screens for attention.
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
  const hostRef = useRef<HTMLDivElement>(null);
  const [scale, setScale] = useState(0.6);

  // The device layout is authored in a fixed 1400×800 coordinate space so
  // the ring geometry stays hand-tuned; the workspace then scales that
  // space to whatever room the panel actually has.
  useLayoutEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    const ro = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect;
      setScale(Math.max(0.32, Math.min(1.2, Math.min(width / CONTENT_W, height / CONTENT_H))));
    });
    ro.observe(host);
    return () => ro.disconnect();
  }, []);

  const nodes = tick.nodes;
  const leaderIdx = nodes.findIndex((n) => n.alive && n.role === "Leader");

  // Selecting a device zooms it, which would push a node on the rim of
  // the ring off the edge of the workspace. Pull the whole layout partway
  // toward that device to compensate — partway rather than all the way so
  // the ring stays recognisable and you don't lose your bearings.
  const [panX, panY] = focus === null ? [0, 0] : [(W / 2 - POS[focus][0]) * 0.6, (H / 2 - POS[focus][1]) * 0.6];

  return (
    <div
      ref={hostRef}
      onClick={() => setFocus(null)}
      style={{
        position: "relative",
        width: "100%",
        height: "100%",
        minHeight: 420,
        border: "1px solid var(--workspace-edge)",
        borderRightColor: "var(--bevel-light)",
        borderBottomColor: "var(--bevel-light)",
        background: "var(--workspace)",
        backgroundImage:
          "linear-gradient(var(--workspace-grid) 1px, transparent 1px), linear-gradient(90deg, var(--workspace-grid) 1px, transparent 1px)",
        backgroundSize: "22px 22px",
        overflow: "hidden",
      }}
    >
      <div
        style={{
          position: "absolute",
          left: "50%",
          top: "50%",
          width: W,
          height: H,
          transform: `translate(-50%, -50%) translate(${panX * scale}px, ${panY * scale}px) scale(${scale})`,
          transformOrigin: "center center",
          transition: "transform .35s cubic-bezier(.2,.9,.25,1)",
        }}
      >
        <svg viewBox={`0 0 ${W} ${H}`} width={W} height={H} style={{ position: "absolute", left: 0, top: 0, zIndex: 1 }}>
          {nodes.map((a, ai) =>
            nodes.slice(ai + 1).map((b, bj) => {
              const bi = ai + 1 + bj;
              const ring = RING.some(([r0, r1]) => (r0 === ai && r1 === bi) || (r0 === bi && r1 === ai));
              const up = a.alive && b.alive;
              const toLeader = up && (leaderIdx === ai || leaderIdx === bi);
              const [x1, y1] = POS[ai];
              const [x2, y2] = POS[bi];
              // Backbone links carry the full LED treatment; the
              // remaining full-mesh links stay faint so the ring reads as
              // the primary topology.
              const stroke = !up ? "var(--down)" : toLeader ? "#3f6f4a" : "#7d8791";
              return (
                <line
                  key={`${a.id}-${b.id}`}
                  x1={x1}
                  y1={y1}
                  x2={x2}
                  y2={y2}
                  stroke={stroke}
                  strokeWidth={ring ? (toLeader ? 2.2 : 1.6) : 1}
                  strokeDasharray={up ? "none" : "9 7"}
                  opacity={ring ? (up ? 0.95 : 0.85) : 0.16}
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

        {/* Port LEDs ride above the devices rather than below them. The
            chassis are different widths, so any fixed offset along the
            link that clears a laptop is still swallowed by a tower — and
            a port light sitting on the edge of the box is what it looks
            like on real hardware anyway. */}
        <svg
          viewBox={`0 0 ${W} ${H}`}
          width={W}
          height={H}
          style={{ position: "absolute", left: 0, top: 0, zIndex: 4, pointerEvents: "none" }}
        >
          {RING.map(([ai, bi]) => (
            <g key={`led-${ai}-${bi}`}>
              <LinkLed x1={POS[ai][0]} y1={POS[ai][1]} x2={POS[bi][0]} y2={POS[bi][1]} up={nodes[ai].alive} />
              <LinkLed x1={POS[bi][0]} y1={POS[bi][1]} x2={POS[ai][0]} y2={POS[ai][1]} up={nodes[bi].alive} />
            </g>
          ))}
        </svg>
      </div>

      <Legend />
    </div>
  );
}

// A link LED sits just off its own endpoint, on the line, the way a port
// status light sits at the interface end of a cable in a network diagram.
function LinkLed({ x1, y1, x2, y2, up }: { x1: number; y1: number; x2: number; y2: number; up: boolean }) {
  const dx = x2 - x1;
  const dy = y2 - y1;
  const len = Math.hypot(dx, dy) || 1;
  const off = 140; // clear of the device chassis and its printed label
  return (
    <circle
      cx={x1 + (dx / len) * off}
      cy={y1 + (dy / len) * off}
      r={5}
      fill={up ? "var(--up)" : "var(--down)"}
      stroke="rgba(0,0,0,.4)"
      strokeWidth={1}
    />
  );
}

function Legend() {
  return (
    <div
      className="raised"
      style={{
        position: "absolute",
        left: 6,
        bottom: 6,
        display: "flex",
        flexDirection: "column",
        gap: 3,
        padding: "5px 8px 6px",
        fontSize: 10.5,
        color: "var(--ink-muted)",
        pointerEvents: "none",
      }}
    >
      <span style={{ fontSize: 9.5, letterSpacing: ".06em", color: "var(--ink-faint)" }}>LEGEND</span>
      <Row swatch={<Dot color="var(--up)" />} label="port up" />
      <Row swatch={<Dot color="var(--down)" />} label="port down" />
      <Row
        swatch={<svg width="18" height="6"><line x1="0" y1="3" x2="18" y2="3" stroke="#3f6f4a" strokeWidth="2.2" /></svg>}
        label="leader link"
      />
      <Row
        swatch={
          <svg width="18" height="6">
            <line x1="0" y1="3" x2="18" y2="3" stroke="var(--down)" strokeWidth="1.6" strokeDasharray="4 3" />
          </svg>
        }
        label="link down"
      />
    </div>
  );
}

function Row({ swatch, label }: { swatch: React.ReactNode; label: string }) {
  return (
    <span style={{ display: "flex", alignItems: "center", gap: 6 }}>
      <span style={{ width: 18, display: "flex", justifyContent: "center" }}>{swatch}</span>
      {label}
    </span>
  );
}

function Dot({ color }: { color: string }) {
  return (
    <span
      style={{ width: 7, height: 7, borderRadius: "50%", background: color, border: "1px solid rgba(0,0,0,.4)" }}
    />
  );
}
