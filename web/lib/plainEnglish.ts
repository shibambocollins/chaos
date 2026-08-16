export interface Translated {
  plain: string;
  raw: string;
  tone: Tone;
}

export type Tone = "leader" | "vote" | "fault" | "commit" | "info";

export function translate(raw: string): Translated {
  for (const rule of RULES) {
    const m = rule.pattern.exec(raw);
    if (m) return { plain: rule.plain(m), raw, tone: rule.tone };
  }
  return { plain: raw, raw, tone: "info" };
}

const RULES: { pattern: RegExp; tone: Tone; plain: (m: RegExpExecArray) => string }[] = [
  {
    pattern: /^node (\d+) became Leader \(term (\d+)\)$/,
    tone: "leader",
    plain: (m) => `Computer ${m[1]} won the vote and is now in charge (round ${m[2]}).`,
  },
  {
    pattern: /^node (\d+) became Candidate \(term (\d+)\)$/,
    tone: "vote",
    plain: (m) => `Computer ${m[1]} is asking the others to vote for it (round ${m[2]}).`,
  },
  {
    pattern: /^node (\d+) became Follower \(term (\d+)\)$/,
    tone: "vote",
    plain: (m) => `Computer ${m[1]} accepted someone else as the boss (round ${m[2]}).`,
  },
  {
    pattern: /^node (\d+) committed index (\d+)$/,
    tone: "commit",
    plain: (m) => `Computer ${m[1]} saved change #${m[2]} for good. It can never be undone.`,
  },
  {
    pattern: /^node (\d+) killed$/,
    tone: "fault",
    plain: (m) => `Computer ${m[1]} lost power and dropped out.`,
  },
  {
    pattern: /^node (\d+) restarted$/,
    tone: "fault",
    plain: (m) => `Computer ${m[1]} was switched back on.`,
  },
];

export const TONE_COLOR: Record<Tone, string> = {
  leader: "var(--up)",
  vote: "var(--busy)",
  fault: "var(--down)",
  commit: "var(--accent)",
  info: "#9a958e",
};
