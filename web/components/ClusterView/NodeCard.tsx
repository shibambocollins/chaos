"use client";

import { useEffect, useRef, useState } from "react";
import type { TraceNodeState } from "@/lib/trace";

// Two palettes, on purpose. Phosphor colours are for pixels behind glass
// — they're bright because a CRT is emissive. Plate colours are for the
// device label printed on the white workspace, where the same green
// would be unreadable. Using one palette for both is what makes a UI
// look like it was picked from a swatch generator rather than built.
const P_GREEN = "#3fbf63";
const P_GREEN_DIM = "#2c7a45";
const P_AMBER = "#e0a33a";
const P_BLUE = "#6fa8dc";
const P_GREY = "#96a0ac";

export type NodeKind = "tower" | "laptop" | "aio";

interface Props {
  node: TraceNodeState;
  kind: NodeKind;
  narration: string[]; // this node's own narration lines up to the current tick
  focused: boolean;
  dimmed: boolean;
  onFocus: () => void;
}

type Transient = "none" | "dying" | "booting";

// NodeCard is the visual heart of the whole project: one Raft node,
// rendered as a small PC whose screen doubles as its data readout. Ported
// from the Chaos Cluster design prototype, with two real differences:
// there's no live fake simulation (every prop comes from a recorded trace
// tick) and no interactive controls (ROLE/NET overrides, a CLI you can
// type into) — you can't "kill" a node in a replay, that already did or
// didn't happen.
//
// Selection is a marching-ants marquee plus a printed label, the way a
// device is selected on a network-simulator canvas. It replaced a
// breathing coloured halo, which read as decoration and — worse — used
// the same visual channel (coloured glow) that already meant "role".
export default function NodeCard({ node, kind, narration, focused, dimmed, onFocus }: Props) {
  const [tab, setTab] = useState<"gui" | "cli">("gui");
  const prevAlive = useRef(node.alive);
  const [transient, setTransient] = useState<Transient>("none");

  // The trace only records discrete before/after states (a kill jumps
  // straight from alive:true to alive:false in one tick, no in-between
  // frame) — but the die/boot animations are worth keeping. Trigger them
  // as a transient overlay whenever `alive` actually flips between
  // renders, rather than needing the trace itself to carry a "dying" state.
  useEffect(() => {
    if (prevAlive.current && !node.alive) {
      setTransient("dying");
      const t = setTimeout(() => setTransient("none"), 1150);
      prevAlive.current = node.alive;
      return () => clearTimeout(t);
    }
    if (!prevAlive.current && node.alive) {
      setTransient("booting");
      const t = setTimeout(() => setTransient("none"), 1350);
      prevAlive.current = node.alive;
      return () => clearTimeout(t);
    }
    prevAlive.current = node.alive;
  }, [node.alive]);

  const on = node.alive;
  const booting = transient === "booting";
  const dying = transient === "dying";
  const isLeader = on && node.role === "Leader";
  const isCand = on && node.role === "Candidate";

  let roleText: string, roleColor: string;
  if (booting) {
    roleText = "BOOTING"; roleColor = P_BLUE;
  } else if (!on) {
    roleText = "OFFLINE"; roleColor = "#3a424c";
  } else if (isLeader) {
    roleText = "LEADER"; roleColor = P_GREEN;
  } else if (isCand) {
    roleText = "CANDIDATE"; roleColor = P_AMBER;
  } else {
    roleText = "FOLLOWER"; roleColor = P_GREY;
  }

  // Plate colours: same four states, darkened for print on white.
  const plateColor = booting
    ? "var(--accent)"
    : !on
      ? "var(--down)"
      : isLeader
        ? "var(--up)"
        : isCand
          ? "var(--busy)"
          : "var(--ink-muted)";

  let ticks = "";
  for (let k = 0; k < 12; k++) {
    const idx = Math.max(0, node.logLen - 12) + k;
    ticks += !on ? "·" : idx < node.commitIndex ? "█" : idx < node.logLen ? "▒" : "·";
  }

  const isTower = kind === "tower";
  const isLaptop = kind === "laptop";
  const isAio = kind === "aio";
  const hasStand = isTower || isAio;
  const closed = isLaptop && !on && !booting;

  const screenOpacity = !on && !dying ? 0 : 1;
  const screenAnim = dying
    ? "chaosDie 1.15s linear forwards"
    : booting
      ? "chaosBoot 1.35s linear forwards"
      : isCand
        ? "chaosAmber 1.1s ease-in-out infinite"
        : "none";
  // Inset phosphor bloom only — the outer coloured glow the chassis used
  // to throw onto the canvas is gone, since a monitor doesn't light up
  // the desk it stands on that much, and on a white workspace it read as
  // a UI effect rather than as a screen.
  const screenGlow = !on
    ? "inset 0 0 0 1px rgba(255,255,255,.03)"
    : booting
      ? "inset 0 0 26px rgba(111,168,220,.2)"
      : isLeader
        ? "inset 0 0 30px rgba(63,191,99,.16)"
        : isCand
          ? "inset 0 0 30px rgba(224,163,58,.18)"
          : "inset 0 0 22px rgba(150,170,200,.08)";
  const chassisShadow = "0 6px 12px rgba(30,34,40,.22), 0 1px 2px rgba(30,34,40,.3)";
  const bezelColor = isLeader ? "#3c4a41" : "#31363d";
  const pwrColor = !on ? "#2a3038" : booting ? P_BLUE : isLeader ? P_GREEN : isCand ? P_AMBER : "#7c8794";
  const pwrRing = on ? "#4d5661" : "#2b3138";
  const pwrGlow = !on
    ? "transparent"
    : isLeader
      ? "rgba(63,191,99,.42)"
      : isCand
        ? "rgba(224,163,58,.42)"
        : "rgba(140,160,180,.22)";
  const ledAnim = booting
    ? "chaosLed .35s ease-in-out infinite"
    : isCand
      ? "chaosLed .55s ease-in-out infinite"
      : isLeader
        ? "chaosLed 1.9s ease-in-out infinite"
        : "none";

  const displayH = isAio ? "158px" : "150px";
  const bezel = isAio ? "7px 7px 17px" : "8px";
  const displayRadius = isLaptop ? "6px 6px 2px 2px" : "5px";
  const lidTransform = closed ? "perspective(900px) rotateX(-86deg)" : "perspective(900px) rotateX(0deg)";

  const kindLabel = isTower ? "Tower" : isAio ? "All-in-One" : "Laptop";

  return (
    <div
      onClick={(e) => {
        e.stopPropagation();
        onFocus();
      }}
      style={{
        position: "relative",
        cursor: "pointer",
        transform: `scale(${focused ? 1.35 : 1})`,
        transformOrigin: "center center",
        transition: "transform .35s cubic-bezier(.2,.9,.25,1), opacity .35s ease",
        opacity: dimmed ? 0.34 : 1,
      }}
    >
      {focused && (
        <svg
          style={{ position: "absolute", inset: -12, width: "calc(100% + 24px)", height: "calc(100% + 24px)", pointerEvents: "none", zIndex: 5 }}
          aria-hidden
        >
          <rect
            x="0.5"
            y="0.5"
            width="99.6%"
            height="99.6%"
            fill="none"
            stroke="var(--accent)"
            strokeWidth="1"
            strokeDasharray="6 6"
            style={{ animation: "chaosMarquee .6s linear infinite" }}
          />
        </svg>
      )}

      <div style={{ position: "relative", display: "flex", alignItems: "flex-end", gap: 9 }}>
        {isTower && (
          <div
            style={{
              width: 48, height: 128, borderRadius: 3, boxSizing: "border-box", padding: "8px 7px",
              display: "flex", flexDirection: "column", gap: 6,
              background: "linear-gradient(160deg, #2c3138, #171a1e)", border: "1px solid #383e46",
              boxShadow: chassisShadow,
            }}
          >
            <div style={{ height: 7, borderRadius: 1, background: "#1b1f24" }} />
            <div style={{ height: 7, borderRadius: 1, background: "#1b1f24" }} />
            <div style={{ height: 4, width: "60%", borderRadius: 1, background: isLeader ? P_GREEN : isCand ? P_AMBER : "#3b434d", opacity: 0.8 }} />
            <div style={{ flex: 1 }} />
            <div
              title="power"
              style={{
                alignSelf: "center", width: 20, height: 20, borderRadius: "50%",
                display: "grid", placeItems: "center",
                border: `1px solid ${pwrRing}`, background: "#101317", color: pwrColor,
                fontSize: 10, lineHeight: 1,
                boxShadow: `0 0 8px ${pwrGlow}`,
                transition: "color .3s ease, box-shadow .3s ease, border-color .3s ease",
              }}
            >
              &#9211;
            </div>
          </div>
        )}

        <div style={{ position: "relative" }}>
          <div
            style={{
              position: "relative", width: 218, height: displayH, boxSizing: "border-box", padding: bezel,
              borderRadius: displayRadius,
              background: "linear-gradient(168deg, #2b3037, #14171b 66%)",
              border: `1px solid ${bezelColor}`, boxShadow: chassisShadow,
              transformOrigin: "bottom center", transform: lidTransform,
              transition: "transform .55s cubic-bezier(.3,.8,.3,1), box-shadow .5s ease, border-color .5s ease",
            }}
          >
            <div style={{ position: "relative", width: "100%", height: "100%", borderRadius: 2, overflow: "hidden", background: "#05070a", boxShadow: screenGlow, transition: "box-shadow .45s ease" }}>
              <div style={{ position: "absolute", inset: 0, opacity: screenOpacity, animation: screenAnim, transition: "opacity .35s ease", display: "flex", flexDirection: "column", fontFamily: "var(--mono-font)" }}>
                <div style={{ flex: "none", height: 17, display: "flex", alignItems: "center", gap: 6, padding: "0 5px", background: "linear-gradient(180deg, #232a33, #171d24)", borderBottom: "1px solid #0b0f14" }}>
                  <span style={{ width: 6, height: 6, background: roleColor }} />
                  <span style={{ fontSize: 9.5, letterSpacing: ".08em", color: "#98a2ae" }}>n{node.id}</span>
                  <span style={{ flex: 1 }} />
                  <ScreenTab label="GUI" active={tab === "gui"} onSelect={() => setTab("gui")} />
                  <ScreenTab label="CLI" active={tab === "cli"} onSelect={() => setTab("cli")} />
                </div>

                <div style={{ flex: 1, minHeight: 0, padding: "7px 8px", boxSizing: "border-box", display: "flex", flexDirection: "column", gap: 6 }}>
                  {tab === "gui" ? (
                    <div style={{ display: "flex", flexDirection: "column", gap: 6, height: "100%" }}>
                      <div style={{ display: "flex", alignItems: "baseline", justifyContent: "space-between" }}>
                        <span style={{ fontSize: 11, letterSpacing: ".07em", color: roleColor }}>{roleText}</span>
                        <span style={{ fontSize: 10, color: "#5d6673" }}>term {node.currentTerm}</span>
                      </div>
                      <div style={{ display: "flex", flexDirection: "column", gap: 1 }}>
                        <div style={{ fontSize: 10, letterSpacing: 0.5, color: isLeader ? P_GREEN : on ? P_GREEN_DIM : "#242a32" }}>{ticks}</div>
                        <div style={{ fontSize: 8.5, color: "#4d5765" }}>
                          log {on ? node.logLen : 0} &middot; commit {on ? node.commitIndex : 0} &middot;{" "}
                          {booting ? "restoring" : !on ? "halted" : isLeader ? "heartbeat" : isCand ? "election" : "idle"}
                        </div>
                      </div>
                    </div>
                  ) : (
                    <div style={{ flex: 1, minHeight: 0, overflow: "auto", whiteSpace: "pre-wrap", fontSize: 9, lineHeight: 1.42, color: "#7f8b99" }}>
                      {narration.length > 0 ? narration.join("\n") : "(no events yet)"}
                    </div>
                  )}
                </div>
              </div>

              <div style={{ position: "absolute", inset: 0, pointerEvents: "none", background: "repeating-linear-gradient(180deg, rgba(255,255,255,.05) 0 1px, transparent 1px 3px)" }} />
              <div style={{ position: "absolute", left: 0, right: 0, height: "32%", pointerEvents: "none", background: "linear-gradient(180deg, transparent, rgba(255,255,255,.035), transparent)", animation: "chaosSweep 6s linear infinite" }} />
              <div style={{ position: "absolute", inset: 0, pointerEvents: "none", boxShadow: "inset 0 0 0 1px rgba(255,255,255,.05), inset 0 12px 24px rgba(255,255,255,.025)" }} />
            </div>

            {isAio && (
              <div style={{ position: "absolute", left: 0, right: 0, bottom: 2, height: 12, display: "flex", alignItems: "center", justifyContent: "space-between", padding: "0 8px", boxSizing: "border-box" }}>
                <span style={{ fontSize: 7.5, letterSpacing: ".18em", color: "#565d67" }}>CHAOS AIO</span>
                <span style={{ width: 11, height: 11, borderRadius: "50%", border: `1px solid ${pwrRing}`, background: pwrColor, boxShadow: `0 0 6px ${pwrGlow}`, display: "block" }} />
              </div>
            )}
          </div>

          {hasStand && (
            <div style={{ display: "flex", flexDirection: "column", alignItems: "center" }}>
              <div style={{ width: 26, height: 14, background: "linear-gradient(180deg, #242930, #191d21)" }} />
              <div style={{ width: 92, height: 6, borderRadius: 2, background: "#242930", boxShadow: "0 4px 9px rgba(30,34,40,.28)" }} />
            </div>
          )}

          {isLaptop && (
            <div
              style={{
                position: "relative", width: 238, marginLeft: -10, height: 15, borderRadius: "2px 2px 5px 5px",
                background: "linear-gradient(180deg, #2b3037, #1b1f24)", border: "1px solid #383e46",
                boxShadow: chassisShadow, display: "flex", alignItems: "center",
                justifyContent: "space-between", padding: "0 10px", boxSizing: "border-box",
              }}
            >
              <div style={{ display: "flex", gap: 2 }}>
                <span style={{ width: 30, height: 4, borderRadius: 1, background: "#14181d" }} />
                <span style={{ width: 30, height: 4, borderRadius: 1, background: "#14181d" }} />
                <span style={{ width: 30, height: 4, borderRadius: 1, background: "#14181d" }} />
              </div>
              <span style={{ width: 5, height: 5, borderRadius: "50%", background: pwrColor, boxShadow: `0 0 5px ${pwrGlow}`, animation: ledAnim, display: "block" }} />
            </div>
          )}
        </div>
      </div>

      {isTower && (
        <div style={{ margin: "8px 0 0 0", width: 214, height: 22, borderRadius: 2, background: "linear-gradient(180deg, #262b31, #1a1e23)", border: "1px solid #343a42", display: "flex", flexDirection: "column", justifyContent: "center", gap: 3, padding: "0 8px", boxSizing: "border-box" }}>
          <div style={{ display: "flex", gap: 2 }}>
            <span style={{ flex: 1, height: 3, borderRadius: 1, background: "#14181d" }} />
            <span style={{ flex: 1, height: 3, borderRadius: 1, background: "#14181d" }} />
            <span style={{ flex: 1, height: 3, borderRadius: 1, background: "#14181d" }} />
          </div>
        </div>
      )}

      {/* Device label, printed on the workspace under the device — the
          network-diagram convention, in the UI font rather than the
          screen font, because it isn't part of the machine. */}
      <div
        style={{
          marginTop: 8,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          gap: 6,
          fontFamily: "var(--ui-font)",
          fontSize: 11,
          color: "var(--ink-muted)",
        }}
      >
        <strong style={{ color: "var(--ink)", fontWeight: 600 }}>Node{node.id}</strong>
        <span style={{ color: "var(--ink-faint)" }}>{kindLabel}</span>
        <span
          style={{
            padding: "0 5px",
            fontSize: 10,
            color: plateColor,
            border: `1px solid ${plateColor}`,
            background: "rgba(255,255,255,.72)",
          }}
        >
          {roleText.toLowerCase()}
        </span>
      </div>
    </div>
  );
}

function ScreenTab({ label, active, onSelect }: { label: string; active: boolean; onSelect: () => void }) {
  return (
    <button
      onClick={(e) => {
        e.stopPropagation();
        onSelect();
      }}
      style={{
        padding: "2px 5px",
        cursor: "pointer",
        font: "inherit",
        fontSize: 8.5,
        letterSpacing: ".08em",
        border: `1px solid ${active ? "#4c5865" : "#252c34"}`,
        background: active ? "#2f3944" : "transparent",
        color: active ? "#dfe5ec" : "#6b7581",
      }}
    >
      {label}
    </button>
  );
}
