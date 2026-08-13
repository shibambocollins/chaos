"use client";

import { useEffect } from "react";
import { SCENARIOS } from "@/lib/loadTrace";

// The scenario blurb used to sit permanently on screen above the
// workspace, which meant every visitor read a paragraph of explanation
// before they could see anything. Someone who already knows what a
// partition is does not want that text parked in their view on every
// visit; someone who does not want it needs more than one sentence, not
// less. So it moved here: an on-demand reference, opened by choice, that
// can afford to go into actual depth glossary-style, and closed by
// default gets the workspace back for everyone else.
export default function HowItWorks({ open, onClose }: { open: boolean; onClose: () => void }) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      onClick={onClose}
      style={{
        position: "fixed", inset: 0, zIndex: 50,
        background: "rgba(20,18,14,.38)",
        display: "flex", alignItems: "center", justifyContent: "center",
        padding: 24,
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="panel"
        style={{ width: "min(760px, 100%)", maxHeight: "min(80vh, 720px)", boxShadow: "0 18px 40px rgba(20,18,14,.35)" }}
      >
        <div className="panel-title">
          <span>How this works</span>
          <span style={{ flex: 1 }} />
          <button
            onClick={onClose}
            aria-label="Close"
            style={{
              cursor: "pointer", color: "#fff", background: "transparent", border: "none",
              font: "inherit", fontSize: 13, padding: "0 4px",
            }}
          >
            &#10005;
          </button>
        </div>

        <div className="panel-body" style={{ overflowY: "auto", padding: "12px 16px 16px", fontSize: 12.5, lineHeight: 1.55 }}>
          <Section title="What you are looking at">
            <p>
              This page shows five computers running Raft, a protocol that lets a group of machines
              agree on one shared, ordered history of data even when machines crash or the network
              between them breaks. It runs in two modes. <strong>Replay</strong> plays back a run
              already simulated and saved by the Go program behind this project (<code>internal/sim</code>);
              clicking an action jumps to the moment it happens in the recording rather than performing
              it on the spot. <strong>Live</strong> connects to an actual <code>chaos-server</code> process
              over HTTP and drives it for real, right now, if one happens to be running.
            </p>
          </Section>

          <Section title="The objective">
            <p>
              There is no score, but there is a goal, and it doubles as a challenge: try to break one of
              Raft&apos;s safety guarantees by clicking around. Specifically, try to cause any of these:
            </p>
            <ul style={{ margin: "6px 0 8px", paddingLeft: 18, display: "flex", flexDirection: "column", gap: 4 }}>
              <li><strong>Split brain</strong>: get two computers to both believe they are leader in the same term.</li>
              <li><strong>Silent data loss</strong>: make a write that was already committed disappear or change.</li>
              <li><strong>A stuck majority</strong>: partition the cluster so badly that no side can ever elect a leader, even though a majority is technically still reachable somewhere.</li>
            </ul>
            <p>
              You will not manage it. <code>internal/sim</code>&apos;s property-based test suite already runs
              50 seeds × 200 rounds of randomized kills, restarts, partitions, and heals, checking for exactly
              these failures after every single simulated event, not just at the end of a run. That
              continuous checking, not the visualization, is the actual engineering behind this project.
            </p>
          </Section>

          <Section title="Terms used on screen, plain and technical">
            <Glossary />
          </Section>

          <Section title="The three recordings">
            <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
              {SCENARIOS.map((s) => (
                <div key={s.id}>
                  <div style={{ fontWeight: 600 }}>
                    {s.plain} <span style={{ fontWeight: 400, color: "var(--ink-faint)" }}>({s.label})</span>
                  </div>
                  <div style={{ color: "var(--ink-muted)" }}>{s.blurb}</div>
                </div>
              ))}
            </div>
          </Section>

          <Section title="Why it matters">
            <p>
              A single computer holding all your data is a single point of failure: if it dies, the
              data (or the service) goes with it. Raft&apos;s answer is to keep several full copies in
              sync and to make sure that, at any moment, at most one of them is allowed to accept new
              writes. The interesting engineering is in the guarantees: a majority of computers must
              agree before anything is considered permanently saved, and a computer is never allowed to
              overwrite already-agreed history, even during a leader change. Those two rules are what
              this project&apos;s tests exist to check.
            </p>
          </Section>

          <Section title="Contribute">
            <p>
              This project is open source:{" "}
              <a
                href="https://github.com/shibambocollins/chaos"
                target="_blank"
                rel="noopener noreferrer"
                style={{ color: "var(--accent)", textDecoration: "underline" }}
              >
                github.com/shibambocollins/chaos
              </a>
              . Issues and pull requests are welcome. A concrete place to start: the workspace&apos;s
              message pulses are a role-based approximation of traffic, not a replay of literal
              messages, because the trace format does not record individual RPCs yet (see the comment
              above <code>ClusterView</code>&apos;s <code>pulses</code>). Recording message-level events
              in <code>internal/sim</code>&apos;s <code>TraceRecorder</code> and animating the real thing
              would be a solid first pull request.
            </p>
          </Section>
        </div>
      </div>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 16 }}>
      <div
        style={{
          fontSize: 10.5, fontWeight: 700, letterSpacing: ".06em", textTransform: "uppercase",
          color: "var(--accent)", borderBottom: "1px solid var(--rule)", paddingBottom: 3, marginBottom: 7,
        }}
      >
        {title}
      </div>
      {children}
    </div>
  );
}

function Glossary() {
  return (
    <dl style={{ display: "grid", gridTemplateColumns: "148px 1fr", rowGap: 8, columnGap: 10 }}>
      {ENTRIES.flatMap(([term, def]) => [
        <dt key={term} style={{ fontWeight: 600, color: "var(--ink)" }}>{term}</dt>,
        <dd key={term + "-def"} style={{ color: "var(--ink-muted)" }}>{def}</dd>,
      ])}
    </dl>
  );
}

const ENTRIES: [string, string][] = [
  ["computer / node", "One machine in the cluster. This project always shows 5 of them."],
  ["term / round", "A number that only ever goes up. Every time a new leader election starts, the term increases, so old messages from an earlier term can be told apart from current ones."],
  ["leader", "The one computer currently allowed to accept new writes and tell the others what order to apply them in."],
  ["follower", "A computer that is not the leader. It accepts instructions from the leader and copies its data."],
  ["candidate", "A computer that thinks there is no leader and is asking the others to vote for it."],
  ["majority / quorum", "More than half the computers. With 5 computers that is 3. A leader needs a majority of votes to win, and a write needs a majority of copies before it is safe."],
  ["log", "Each computer's ordered list of writes. Followers copy the leader's log entry by entry."],
  ["committed", "A write that a majority of computers now have a copy of. Once committed, it is permanent and can never be rolled back, even if the leader that accepted it later dies."],
  ["heartbeat", "A small message the leader sends every computer, regularly, just to say \"I am still here.\" If followers stop hearing it, they assume the leader died and start an election."],
  ["kill / power off", "A computer stops responding entirely, as if unplugged. It cannot send or receive anything until it restarts."],
  ["restart", "A killed computer comes back. It remembers what it had saved before, but has to be told anything it missed while it was off."],
  ["partition / network split", "The computers can no longer all reach each other, as if a cable were cut, even though every computer is still powered on. Only a side with a majority can keep accepting writes; a minority side stalls rather than risk disagreeing with the other side. Shown on screen as a violet dashed line between the computers that can no longer reach each other."],
  ["heal", "A partition is repaired and every computer can reach every other computer again."],
];
