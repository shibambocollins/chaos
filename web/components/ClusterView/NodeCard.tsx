"use client";

import { useEffect, useRef, useState } from "react";
import type { TraceNodeState } from "@/lib/trace";

const GREEN = "oklch(0.82 0.17 150)";
const GREEN_DIM = "oklch(0.6 0.11 150)";
const AMBER = "oklch(0.85 0.16 82)";
const BLUE = "oklch(0.78 0.11 240)";
const GREY = "#9aa4b0";

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

  let roleText: string, roleColor: string, roleGlow: string;
  if (booting) {
    roleText = "booting…"; roleColor = BLUE; roleGlow = `0 0 12px ${BLUE}`;
  } else if (!on) {
    roleText = "offline"; roleColor = "#39404a"; roleGlow = "none";
  } else if (isLeader) {
    roleText = "LEADER"; roleColor = GREEN; roleGlow = "0 0 14px oklch(0.82 0.17 150 / .6)";
  } else if (isCand) {
    roleText = "candidate"; roleColor = AMBER; roleGlow = "0 0 13px oklch(0.85 0.16 82 / .55)";
  } else {
    roleText = "follower"; roleColor = GREY; roleGlow = "none";
  }

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
  const screenGlow = !on
    ? "inset 0 0 0 1px rgba(255,255,255,.03)"
    : booting
      ? "inset 0 0 30px oklch(0.78 0.11 240 / .28)"
      : isLeader
        ? "inset 0 0 36px oklch(0.82 0.17 150 / .24)"
        : isCand
          ? "inset 0 0 36px oklch(0.85 0.16 82 / .28)"
          : "inset 0 0 24px rgba(150,170,200,.1)";
  const chassisShadow = isLeader
    ? "0 0 0 1px oklch(0.82 0.17 150 / .45), 0 0 30px oklch(0.82 0.17 150 / .24), 0 18px 34px rgba(0,0,0,.32)"
    : "0 16px 30px rgba(0,0,0,.3)";
  const bezelColor = isLeader ? "oklch(0.5 0.09 150)" : "#31363d";
  const halo = isLeader ? "oklch(0.82 0.17 150 / .45)" : isCand ? "oklch(0.85 0.16 82 / .38)" : "transparent";
  const haloOpacity = isLeader || isCand ? 1 : 0;
  const haloAnim = isLeader
    ? "chaosBreathe 2.6s ease-in-out infinite"
    : isCand
      ? "chaosBreathe 1.05s ease-in-out infinite"
      : "none";
  const pwrColor = !on ? "#2a3038" : booting ? BLUE : isLeader ? GREEN : isCand ? AMBER : "#7c8794";
  const pwrRing = on ? "#4d5661" : "#2b3138";
  const pwrGlow = !on
    ? "transparent"
    : isLeader
      ? "oklch(0.82 0.17 150 / .5)"
      : isCand
        ? "oklch(0.85 0.16 82 / .5)"
        : "rgba(140,160,180,.25)";
  const ledAnim = booting
    ? "chaosLed .35s ease-in-out infinite"
    : isCand
      ? "chaosLed .55s ease-in-out infinite"
      : isLeader
        ? "chaosLed 1.9s ease-in-out infinite"
        : "none";

  const displayH = isAio ? "158px" : "150px";
  const bezel = isAio ? "7px 7px 17px" : "8px";
  const displayRadius = isLaptop ? "9px 9px 3px 3px" : "10px";
  const lidTransform = closed ? "perspective(900px) rotateX(-86deg)" : "perspective(900px) rotateX(0deg)";

  const kindLabel = isTower ? "tower" : isAio ? "all-in-one" : "laptop";
  const shortState = !on ? "off" : booting ? "boot" : isLeader ? "leader" : node.role.toLowerCase();

  return (
    <div
      onClick={(e) => {
        e.stopPropagation();
        onFocus();
      }}
      style={{
        position: "relative",
        cursor: "pointer",
        transform: `scale(${focused ? 1.5 : 1})`,
        transformOrigin: "center center",
        transition: "transform .45s cubic-bezier(.2,.9,.25,1), filter .45s ease, opacity .45s ease",
        filter: dimmed ? "blur(3.5px) saturate(.7)" : "none",
        opacity: dimmed ? 0.42 : 1,
      }}
    >
      <div style={{ position: "relative", display: "flex", alignItems: "flex-end", gap: 9 }}>
        {isTower && (
          <div
            style={{
              width: 48, height: 128, borderRadius: 5, boxSizing: "border-box", padding: "8px 7px",
              display: "flex", flexDirection: "column", gap: 6,
              background: "linear-gradient(160deg, #2c3138, #171a1e)", border: "1px solid #383e46",
              boxShadow: "0 12px 22px rgba(0,0,0,.3)",
            }}
          >
            <div style={{ height: 7, borderRadius: 2, background: "#1b1f24" }} />
            <div style={{ height: 7, borderRadius: 2, background: "#1b1f24" }} />
            <div style={{ height: 4, width: "60%", borderRadius: 2, background: isLeader ? GREEN : isCand ? AMBER : "#3b434d", opacity: 0.8 }} />
            <div style={{ flex: 1 }} />
            <div
              title="power"
              style={{
                alignSelf: "center", width: 20, height: 20, borderRadius: "50%",
                display: "grid", placeItems: "center",
                border: `1px solid ${pwrRing}`, background: "#101317", color: pwrColor,
                fontSize: 10, lineHeight: 1,
                boxShadow: `0 0 10px ${pwrGlow}`,
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
              position: "absolute", left: -16, right: -16, top: -16, bottom: -10,
              borderRadius: 24, pointerEvents: "none", filter: "blur(15px)",
              background: halo, opacity: haloOpacity, animation: haloAnim,
              transition: "opacity .5s ease",
            }}
          />

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
            <div style={{ position: "relative", width: "100%", height: "100%", borderRadius: 4, overflow: "hidden", background: "#05070a", boxShadow: screenGlow, transition: "box-shadow .45s ease" }}>
              <div style={{ position: "absolute", inset: 0, opacity: screenOpacity, animation: screenAnim, transition: "opacity .35s ease", display: "flex", flexDirection: "column" }}>
                <div style={{ flex: "none", height: 17, display: "flex", alignItems: "center", gap: 6, padding: "0 5px", background: "linear-gradient(180deg, #232a33, #171d24)", borderBottom: "1px solid #0b0f14" }}>
                  <span style={{ width: 6, height: 6, borderRadius: 1, background: roleColor, boxShadow: `0 0 6px ${roleColor}` }} />
                  <span style={{ fontSize: 9.5, letterSpacing: ".12em", color: "#98a2ae" }}>n{node.id}</span>
                  <span style={{ flex: 1 }} />
                  <button
                    onClick={(e) => { e.stopPropagation(); setTab("gui"); }}
                    style={{
                      padding: "2px 5px", cursor: "pointer", fontSize: 8.5, letterSpacing: ".1em",
                      border: `1px solid ${tab === "gui" ? "#4c5865" : "#252c34"}`,
                      background: tab === "gui" ? "#2f3944" : "transparent",
                      color: tab === "gui" ? "#dfe5ec" : "#6b7581",
                    }}
                  >
                    GUI
                  </button>
                  <button
                    onClick={(e) => { e.stopPropagation(); setTab("cli"); }}
                    style={{
                      padding: "2px 5px", cursor: "pointer", fontSize: 8.5, letterSpacing: ".1em",
                      border: `1px solid ${tab === "cli" ? "#4c5865" : "#252c34"}`,
                      background: tab === "cli" ? "#2f3944" : "transparent",
                      color: tab === "cli" ? "#dfe5ec" : "#6b7581",
                    }}
                  >
                    CLI
                  </button>
                </div>

                <div style={{ flex: 1, minHeight: 0, padding: "7px 8px", boxSizing: "border-box", display: "flex", flexDirection: "column", gap: 6 }}>
                  {tab === "gui" ? (
                    <div style={{ display: "flex", flexDirection: "column", gap: 6, height: "100%" }}>
                      <div style={{ display: "flex", alignItems: "baseline", justifyContent: "space-between" }}>
                        <span style={{ fontSize: 11.5, letterSpacing: ".09em", color: roleColor, textShadow: roleGlow }}>{roleText}</span>
                        <span style={{ fontSize: 10, color: "#5d6673" }}>term {node.currentTerm}</span>
                      </div>
                      <div style={{ display: "flex", flexDirection: "column", gap: 1 }}>
                        <div style={{ fontSize: 10, letterSpacing: 0.5, color: isLeader ? GREEN : on ? GREEN_DIM : "#242a32" }}>{ticks}</div>
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
              <div style={{ position: "absolute", left: 0, right: 0, height: "32%", pointerEvents: "none", background: "linear-gradient(180deg, transparent, rgba(255,255,255,.04), transparent)", animation: "chaosSweep 6s linear infinite" }} />
              <div style={{ position: "absolute", inset: 0, pointerEvents: "none", borderRadius: 4, boxShadow: "inset 0 0 0 1px rgba(255,255,255,.05), inset 0 12px 24px rgba(255,255,255,.028)" }} />
            </div>

            {isAio && (
              <div style={{ position: "absolute", left: 0, right: 0, bottom: 2, height: 12, display: "flex", alignItems: "center", justifyContent: "space-between", padding: "0 8px", boxSizing: "border-box" }}>
                <span style={{ fontSize: 7.5, letterSpacing: ".2em", color: "#565d67" }}>CHAOS AIO</span>
                <span style={{ width: 11, height: 11, borderRadius: "50%", border: `1px solid ${pwrRing}`, background: pwrColor, boxShadow: `0 0 8px ${pwrGlow}`, display: "block" }} />
              </div>
            )}
          </div>

          {hasStand && (
            <div style={{ display: "flex", flexDirection: "column", alignItems: "center" }}>
              <div style={{ width: 26, height: 14, background: "linear-gradient(180deg, #242930, #191d21)" }} />
              <div style={{ width: 92, height: 6, borderRadius: 4, background: "#242930", boxShadow: "0 6px 14px rgba(0,0,0,.3)" }} />
            </div>
          )}

          {isLaptop && (
            <div
              style={{
                position: "relative", width: 238, marginLeft: -10, height: 15, borderRadius: "2px 2px 8px 8px",
                background: "linear-gradient(180deg, #2b3037, #1b1f24)", border: "1px solid #383e46",
                boxShadow: "0 14px 26px rgba(0,0,0,.34)", display: "flex", alignItems: "center",
                justifyContent: "space-between", padding: "0 10px", boxSizing: "border-box",
              }}
            >
              <div style={{ display: "flex", gap: 2 }}>
                <span style={{ width: 30, height: 4, borderRadius: 1, background: "#14181d" }} />
                <span style={{ width: 30, height: 4, borderRadius: 1, background: "#14181d" }} />
                <span style={{ width: 30, height: 4, borderRadius: 1, background: "#14181d" }} />
              </div>
              <span style={{ width: 5, height: 5, borderRadius: "50%", background: pwrColor, boxShadow: `0 0 7px ${pwrGlow}`, animation: ledAnim, display: "block" }} />
            </div>
          )}
        </div>
      </div>

      {isTower && (
        <div style={{ margin: "8px 0 0 0", width: 214, height: 22, borderRadius: 3, background: "linear-gradient(180deg, #262b31, #1a1e23)", border: "1px solid #343a42", display: "flex", flexDirection: "column", justifyContent: "center", gap: 3, padding: "0 8px", boxSizing: "border-box" }}>
          <div style={{ display: "flex", gap: 2 }}>
            <span style={{ flex: 1, height: 3, borderRadius: 1, background: "#14181d" }} />
            <span style={{ flex: 1, height: 3, borderRadius: 1, background: "#14181d" }} />
            <span style={{ flex: 1, height: 3, borderRadius: 1, background: "#14181d" }} />
          </div>
        </div>
      )}

      <div style={{ marginTop: 9, display: "flex", alignItems: "center", gap: 7, fontSize: 9.5, letterSpacing: ".12em", color: "light-dark(#7c828b, #6d7581)" }}>
        <span>n{node.id} &middot; {kindLabel}</span>
        <span style={{ color: roleColor }}>{shortState}</span>
      </div>
    </div>
  );
}
