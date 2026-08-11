"use client";

import { useEffect, useRef, useState } from "react";
import type { TraceNodeState } from "@/lib/trace";
import type { ScenarioId } from "@/lib/loadTrace";
import { DEVICE_ACTIONS, type DeviceAction } from "@/lib/scenarioActions";
import { translate } from "@/lib/plainEnglish";

// Two palettes, on purpose. Phosphor colours are for pixels behind
// glass, and they are bright because a CRT is emissive. Plate colours
// are for the device label printed on the white workspace, where the
// same green would be unreadable. Using one palette for both is what
// makes a UI look like it was picked from a swatch generator.
const P_GREEN = "#46d16d";
const P_GREEN_DIM = "#2f8a4c";
const P_AMBER = "#e8ac42";
const P_BLUE = "#79b2e6";
const P_GREY = "#9aa4b0";

export type NodeKind = "tower" | "laptop" | "aio";

interface Props {
  node: TraceNodeState;
  kind: NodeKind;
  narration: string[]; // this node's own narration lines up to the current tick
  focused: boolean;
  dimmed: boolean;
  activeScenario: ScenarioId;
  onFocus: () => void;
  onAction: (a: DeviceAction) => void;
}

type Transient = "none" | "dying" | "booting";
type Tab = "status" | "log" | "act";

// NodeCard is the visual heart of the project: one Raft node drawn as a
// beige desktop machine whose screen doubles as its readout, and whose
// screen is also where you drive the thing from.
//
// Putting the scenario controls on the devices instead of in a toolbar
// is the difference between "pick a data file" and "do something to this
// computer". The actions are still replays underneath, and the ACTIONS
// screen says so, but the gesture is the one a person expects: click the
// machine, tell it to switch off.
export default function NodeCard({
  node,
  kind,
  narration,
  focused,
  dimmed,
  activeScenario,
  onFocus,
  onAction,
}: Props) {
  const [tab, setTab] = useState<Tab>("status");
  const prevAlive = useRef(node.alive);
  const [transient, setTransient] = useState<Transient>("none");

  // The trace only records discrete before/after states (a kill jumps
  // straight from alive:true to alive:false in one tick, no in-between
  // frame) but the die/boot animations are worth keeping. Trigger them
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

  // Plain wording first, exact Raft term underneath. Someone who knows
  // the protocol still gets "Leader / term 2"; someone who does not can
  // read the cluster off the screens without being taught the words.
  let plainState: string, techState: string, roleColor: string;
  if (booting) {
    plainState = "STARTING UP"; techState = "restoring state"; roleColor = P_BLUE;
  } else if (!on) {
    plainState = "POWERED OFF"; techState = "unreachable"; roleColor = "#3a424c";
  } else if (isLeader) {
    plainState = "IN CHARGE"; techState = `Leader · term ${node.currentTerm}`; roleColor = P_GREEN;
  } else if (isCand) {
    plainState = "ASKING FOR VOTES"; techState = `Candidate · term ${node.currentTerm}`; roleColor = P_AMBER;
  } else {
    plainState = "FOLLOWING"; techState = `Follower · term ${node.currentTerm}`; roleColor = P_GREY;
  }

  const plateColor = booting
    ? "var(--accent)"
    : !on
      ? "var(--down)"
      : isLeader
        ? "var(--up)"
        : isCand
          ? "var(--busy)"
          : "var(--ink-muted)";
  const plateWord = booting ? "starting" : !on ? "powered off" : isLeader ? "in charge" : isCand ? "voting" : "following";

  let ticks = "";
  for (let k = 0; k < 12; k++) {
    const idx = Math.max(0, node.logLen - 12) + k;
    ticks += !on ? "·" : idx < node.commitIndex ? "█" : idx < node.logLen ? "▒" : "·";
  }

  const isTower = kind === "tower";
  const isLaptop = kind === "laptop";
  const isAio = kind === "aio";
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
    ? "inset 0 0 0 1px rgba(0,0,0,.5)"
    : booting
      ? "inset 0 0 26px rgba(121,178,230,.2)"
      : isLeader
        ? "inset 0 0 30px rgba(70,209,109,.15)"
        : isCand
          ? "inset 0 0 30px rgba(232,172,66,.17)"
          : "inset 0 0 22px rgba(150,170,200,.07)";

  const pwrColor = !on ? "#6f6a5c" : booting ? P_BLUE : isLeader ? "#2fbf5c" : isCand ? "#e0a01f" : "#5f9b6f";
  const ledAnim = booting
    ? "chaosLed .35s ease-in-out infinite"
    : isCand
      ? "chaosLed .55s ease-in-out infinite"
      : isLeader
        ? "chaosLed 1.9s ease-in-out infinite"
        : "none";

  const bezelPad = isLaptop ? 11 : 15;
  const screenH = isAio ? 132 : 128;
  const lidTransform = closed ? "perspective(900px) rotateX(-84deg)" : "perspective(900px) rotateX(0deg)";
  const kindLabel = isTower ? "Desktop" : isAio ? "All-in-One" : "Laptop";

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
            x="0.5" y="0.5" width="99.6%" height="99.6%"
            fill="none" stroke="var(--accent)" strokeWidth="1" strokeDasharray="6 6"
            style={{ animation: "chaosMarquee .6s linear infinite" }}
          />
        </svg>
      )}

      <div style={{ position: "relative", display: "flex", alignItems: "flex-end", gap: 9 }}>
        {isTower && <Tower on={on} pwrColor={pwrColor} ledAnim={ledAnim} />}

        <div style={{ position: "relative" }}>
          {/* Monitor: thick putty bezel around a recessed tube. */}
          <div
            style={{
              position: "relative", width: 236, boxSizing: "border-box",
              padding: `${bezelPad}px ${bezelPad}px ${isLaptop ? bezelPad : 22}px`,
              borderRadius: isLaptop ? "6px 6px 2px 2px" : "7px 7px 4px 4px",
              background: "linear-gradient(168deg, var(--case-lit), var(--case) 45%, var(--case-dim))",
              border: "1px solid var(--case-edge)",
              borderTopColor: "#f2edde",
              borderLeftColor: "#eee9d9",
              boxShadow: "0 7px 13px rgba(60,55,40,.26), 0 1px 2px rgba(60,55,40,.3)",
              transformOrigin: "bottom center", transform: lidTransform,
              transition: "transform .55s cubic-bezier(.3,.8,.3,1)",
            }}
          >
            <div
              style={{
                position: "relative", height: screenH, overflow: "hidden",
                background: "var(--crt)",
                // Inner shadow on the top/left reads as the tube sitting
                // down inside the bezel rather than pasted onto it.
                boxShadow: `${screenGlow}, inset 0 2px 5px rgba(0,0,0,.9), 0 0 0 1px var(--case-shadow)`,
                transition: "box-shadow .45s ease",
              }}
            >
              <div style={{ position: "absolute", inset: 0, opacity: screenOpacity, animation: screenAnim, transition: "opacity .35s ease", display: "flex", flexDirection: "column", fontFamily: "var(--mono-font)" }}>
                <div style={{ flex: "none", height: 16, display: "flex", alignItems: "center", gap: 5, padding: "0 4px", background: "#1a2028", borderBottom: "1px solid #000" }}>
                  <span style={{ width: 6, height: 6, background: roleColor }} />
                  <span style={{ fontSize: 9, letterSpacing: ".06em", color: "#8f99a5" }}>PC{node.id}</span>
                  <span style={{ flex: 1 }} />
                  <ScreenTab label="STATUS" active={tab === "status"} onSelect={() => setTab("status")} />
                  <ScreenTab label="LOG" active={tab === "log"} onSelect={() => setTab("log")} />
                  <ScreenTab label="DO" active={tab === "act"} onSelect={() => setTab("act")} />
                </div>

                <div style={{ flex: 1, minHeight: 0, padding: 6, boxSizing: "border-box" }}>
                  {tab === "status" && (
                    <StatusScreen
                      plainState={plainState}
                      techState={techState}
                      roleColor={roleColor}
                      ticks={ticks}
                      tickColor={isLeader ? P_GREEN : on ? P_GREEN_DIM : "#242a32"}
                      logLen={on ? node.logLen : 0}
                      commitIndex={on ? node.commitIndex : 0}
                    />
                  )}
                  {tab === "log" && <LogScreen narration={narration} />}
                  {tab === "act" && <ActionScreen activeScenario={activeScenario} onAction={onAction} />}
                </div>
              </div>

              <div style={{ position: "absolute", inset: 0, pointerEvents: "none", background: "repeating-linear-gradient(180deg, rgba(255,255,255,.05) 0 1px, transparent 1px 3px)" }} />
              <div style={{ position: "absolute", left: 0, right: 0, height: "32%", pointerEvents: "none", background: "linear-gradient(180deg, transparent, rgba(255,255,255,.03), transparent)", animation: "chaosSweep 6s linear infinite" }} />
            </div>

            {!isLaptop && (
              <div style={{ position: "absolute", left: bezelPad, right: bezelPad, bottom: 5, height: 13, display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                <span style={{ fontSize: 7.5, letterSpacing: ".16em", color: "#8c8471", fontFamily: "var(--ui-font)" }}>CHAOS</span>
                <span style={{ display: "flex", alignItems: "center", gap: 5 }}>
                  <span style={{ width: 14, height: 3, background: "var(--case-vent)", borderTop: "1px solid var(--case-shadow)" }} />
                  <span style={{ width: 14, height: 3, background: "var(--case-vent)", borderTop: "1px solid var(--case-shadow)" }} />
                  <span style={{ width: 6, height: 6, borderRadius: "50%", background: pwrColor, animation: ledAnim, border: "1px solid rgba(0,0,0,.35)" }} />
                </span>
              </div>
            )}
          </div>

          {!isLaptop && <Stand />}
          {isLaptop && <LaptopBase pwrColor={pwrColor} ledAnim={ledAnim} />}
        </div>
      </div>

      {isTower && <Keyboard />}

      {/* Device label printed on the workspace, network-diagram style:
          in the UI font, not the screen font, because it is not part of
          the machine. */}
      <div
        style={{
          marginTop: 8, display: "flex", alignItems: "center", justifyContent: "center", gap: 6,
          fontFamily: "var(--ui-font)", fontSize: 11, color: "var(--ink-muted)",
        }}
      >
        <strong style={{ color: "var(--ink)", fontWeight: 600 }}>PC{node.id}</strong>
        <span style={{ color: "var(--ink-faint)" }}>{kindLabel}</span>
        <span
          style={{
            padding: "0 5px", fontSize: 10, color: plateColor,
            border: `1px solid ${plateColor}`, background: "rgba(255,255,255,.75)",
          }}
        >
          {plateWord}
        </span>
      </div>
    </div>
  );
}

/* ---- Screens ------------------------------------------------------ */

function StatusScreen({
  plainState, techState, roleColor, ticks, tickColor, logLen, commitIndex,
}: {
  plainState: string; techState: string; roleColor: string;
  ticks: string; tickColor: string; logLen: number; commitIndex: number;
}) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 7, height: "100%" }}>
      <div>
        <div style={{ fontSize: 12, letterSpacing: ".05em", color: roleColor }}>{plainState}</div>
        <div style={{ fontSize: 8.5, color: "#5d6673", marginTop: 1 }}>{techState}</div>
      </div>
      <div>
        <div style={{ fontSize: 10, letterSpacing: 0.5, color: tickColor }}>{ticks}</div>
        <div style={{ fontSize: 8.5, color: "#68727f", marginTop: 2 }}>
          {commitIndex} of {logLen} changes saved for good
        </div>
        <div style={{ fontSize: 8, color: "#454e59" }}>
          log {logLen} · commit {commitIndex}
        </div>
      </div>
    </div>
  );
}

function LogScreen({ narration }: { narration: string[] }) {
  if (narration.length === 0) {
    return <div style={{ fontSize: 9, color: "#5d6673" }}>Nothing has happened to this computer yet.</div>;
  }
  return (
    <div style={{ height: "100%", overflow: "auto", display: "flex", flexDirection: "column", gap: 4 }}>
      {narration
        .slice()
        .reverse()
        .map((line, i) => {
          const t = translate(line);
          return (
            <div key={i}>
              <div style={{ fontSize: 8.5, lineHeight: 1.3, color: "#98a6b4" }}>{t.plain}</div>
              <div style={{ fontSize: 7.5, lineHeight: 1.3, color: "#4d5765" }}>{t.raw}</div>
            </div>
          );
        })}
    </div>
  );
}

function ActionScreen({
  activeScenario,
  onAction,
}: {
  activeScenario: ScenarioId;
  onAction: (a: DeviceAction) => void;
}) {
  return (
    <div style={{ height: "100%", overflow: "auto", display: "flex", flexDirection: "column", gap: 2 }}>
      {DEVICE_ACTIONS.map((a) => {
        const live = a.scenario === activeScenario;
        return (
          <button
            key={a.key}
            title={a.detail}
            onClick={(e) => {
              e.stopPropagation();
              onAction(a);
            }}
            style={{
              display: "block", width: "100%", textAlign: "left", cursor: "pointer",
              padding: "3px 5px", font: "inherit", fontSize: 8.5, lineHeight: 1.25,
              color: live ? "#dfe8f2" : "#93a0ad",
              background: live ? "#243447" : "#161c23",
              border: `1px solid ${live ? "#3d5a7a" : "#242c35"}`,
            }}
          >
            {a.label}
          </button>
        );
      })}
      <div style={{ fontSize: 7, lineHeight: 1.3, color: "#4d5765", paddingTop: 2 }}>
        These jump to a recorded run of that event.
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
        padding: "2px 4px", cursor: "pointer", font: "inherit", fontSize: 7.5, letterSpacing: ".06em",
        border: `1px solid ${active ? "#49576a" : "#20272f"}`,
        background: active ? "#2c3846" : "transparent",
        color: active ? "#dde5ee" : "#68727f",
      }}
    >
      {label}
    </button>
  );
}

/* ---- Chassis ------------------------------------------------------ */

const CASE_FACE = "linear-gradient(160deg, var(--case-lit), var(--case) 50%, var(--case-dim))";
const CASE_EDGE = "1px solid var(--case-edge)";
const CASE_SHADOW = "0 6px 12px rgba(60,55,40,.24), 0 1px 2px rgba(60,55,40,.28)";

function Tower({ on, pwrColor, ledAnim }: { on: boolean; pwrColor: string; ledAnim: string }) {
  return (
    <div
      style={{
        width: 54, height: 138, borderRadius: 3, boxSizing: "border-box", padding: "7px 6px",
        display: "flex", flexDirection: "column", gap: 5,
        background: CASE_FACE, border: CASE_EDGE, borderTopColor: "#f2edde", borderLeftColor: "#eee9d9",
        boxShadow: CASE_SHADOW,
      }}
    >
      {/* Optical drive and floppy slot. */}
      <div style={{ height: 9, background: "var(--case-dim)", border: "1px solid var(--case-shadow)", display: "flex", alignItems: "center", justifyContent: "flex-end", paddingRight: 3 }}>
        <span style={{ width: 5, height: 2, background: "var(--case-shadow)" }} />
      </div>
      <div style={{ height: 6, background: "var(--case-dim)", border: "1px solid var(--case-shadow)" }} />
      <div style={{ flex: 1 }} />
      {/* Vent slots. */}
      <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {[0, 1, 2, 3].map((i) => (
          <span key={i} style={{ height: 2, background: "var(--case-vent)", borderTop: "1px solid var(--case-shadow)" }} />
        ))}
      </div>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <span
          title="power"
          style={{
            width: 17, height: 17, borderRadius: "50%",
            background: "radial-gradient(circle at 35% 30%, var(--case-lit), var(--case-dim))",
            border: "1px solid var(--case-shadow)",
            display: "grid", placeItems: "center", fontSize: 8, color: "#7d7563",
          }}
        >
          &#9211;
        </span>
        <span
          style={{
            width: 5, height: 5, borderRadius: "50%", background: pwrColor,
            border: "1px solid rgba(0,0,0,.35)", animation: ledAnim,
            opacity: on ? 1 : 0.5,
          }}
        />
      </div>
    </div>
  );
}

function Stand() {
  return (
    <div style={{ display: "flex", flexDirection: "column", alignItems: "center" }}>
      <div style={{ width: 62, height: 13, background: "linear-gradient(180deg, var(--case-dim), var(--case-edge))", borderLeft: CASE_EDGE, borderRight: CASE_EDGE }} />
      <div style={{ width: 108, height: 8, borderRadius: "2px 2px 4px 4px", background: "linear-gradient(180deg, var(--case), var(--case-edge))", border: CASE_EDGE, boxShadow: "0 4px 9px rgba(60,55,40,.26)" }} />
    </div>
  );
}

function LaptopBase({ pwrColor, ledAnim }: { pwrColor: string; ledAnim: string }) {
  return (
    <div
      style={{
        position: "relative", width: 256, marginLeft: -10, height: 17, borderRadius: "2px 2px 6px 6px",
        background: "linear-gradient(180deg, var(--case-lit), var(--case-dim))",
        border: CASE_EDGE, borderTopColor: "#f2edde",
        boxShadow: CASE_SHADOW, display: "flex", alignItems: "center",
        justifyContent: "space-between", padding: "0 10px", boxSizing: "border-box",
      }}
    >
      <div style={{ display: "flex", gap: 3 }}>
        {[0, 1, 2].map((i) => (
          <span key={i} style={{ width: 34, height: 5, background: "var(--case-vent)", border: "1px solid var(--case-shadow)" }} />
        ))}
      </div>
      <span style={{ width: 5, height: 5, borderRadius: "50%", background: pwrColor, border: "1px solid rgba(0,0,0,.35)", animation: ledAnim }} />
    </div>
  );
}

function Keyboard() {
  return (
    <div
      style={{
        margin: "9px 0 0", width: 232, height: 24, borderRadius: "2px 2px 4px 4px",
        background: CASE_FACE, border: CASE_EDGE, borderTopColor: "#f2edde",
        boxShadow: CASE_SHADOW, display: "flex", flexDirection: "column",
        justifyContent: "center", gap: 3, padding: "0 9px", boxSizing: "border-box",
      }}
    >
      {[0, 1].map((row) => (
        <div key={row} style={{ display: "flex", gap: 2 }}>
          {Array.from({ length: 14 }, (_, i) => (
            <span key={i} style={{ flex: 1, height: 4, background: "var(--case-dim)", border: "1px solid var(--case-shadow)" }} />
          ))}
        </div>
      ))}
    </div>
  );
}
