export function slaRemaining(due: string | null, now = Date.now()): { label: string; overdue: boolean } | null {
  if (!due) return null;
  const ms = new Date(due).getTime() - now;
  const abs = Math.abs(ms);
  const h = Math.floor(abs / 3_600_000);
  const m = Math.floor((abs % 3_600_000) / 60_000);
  const text = h >= 48 ? `${Math.floor(h / 24)}d` : h > 0 ? `${h}h ${m}m` : `${m}m`;
  return ms < 0 ? { label: `${text} overdue`, overdue: true } : { label: `${text} left`, overdue: false };
}

export type SlaState = "ok" | "warn" | "bad" | "met" | "missed" | "none";

export interface SlaRing {
  /** share of the window already used, 0..1 (clamped); the ring arc length */
  used: number;
  remainingMs: number;
  state: SlaState;
}

/** Warn when 75% of the window is gone, or under 4 hours remain (the same 4 h rule as the at-risk KPI). */
export const WARN_USED = 0.75;
export const WARN_MS = 4 * 3_600_000;

/**
 * Ring maths for a ticket's resolution window [created, due].
 * A resolved ticket is "met" or "missed" (from the server's sla_breached); otherwise the state follows the clock.
 */
export function slaRing(
  t: { created_at: string; sla_due_at: string | null; resolved_at: string | null; sla_breached: boolean },
  now = Date.now(),
): SlaRing {
  const created = new Date(t.created_at).getTime();
  if (!t.sla_due_at) return { used: 0, remainingMs: 0, state: "none" };
  const due = new Date(t.sla_due_at).getTime();
  const span = Math.max(1, due - created);
  if (t.resolved_at) {
    const end = new Date(t.resolved_at).getTime();
    return { used: Math.min(1, Math.max(0, (end - created) / span)), remainingMs: due - end, state: t.sla_breached ? "missed" : "met" };
  }
  const used = Math.min(1, Math.max(0, (now - created) / span));
  const remainingMs = due - now;
  const state: SlaState = remainingMs <= 0 || t.sla_breached ? "bad" : used >= WARN_USED || remainingMs < WARN_MS ? "warn" : "ok";
  return { used, remainingMs, state };
}

/** "3 h 59 m", "12 m" or "2 d 4 h": the size of a span, never negative. */
export function span(ms: number): string {
  const abs = Math.abs(ms);
  const h = Math.floor(abs / 3_600_000);
  const m = Math.floor((abs % 3_600_000) / 60_000);
  if (h >= 48) return `${Math.floor(h / 24)} d ${h % 24} h`;
  return h > 0 ? (m === 0 ? `${h} h` : `${h} h ${m} m`) : `${m} m`;
}

export interface SlaLine { text: string; tone: "label2" | "orange" | "red" }

/**
 * The single SLA line for a ticket: "3 h 59 m left" (neutral), the same in orange when at risk,
 * "Overdue by 1 h 12 m" in red once breached. A resolved ticket says whether it met its SLA. No deadline: null.
 */
export function slaLine(
  t: { created_at: string; sla_due_at: string | null; resolved_at: string | null; sla_breached: boolean },
  now = Date.now(),
  short = false,
): SlaLine | null {
  if (!t.sla_due_at) return null;
  const r = slaRing(t, now);
  if (t.resolved_at) return r.state === "missed" ? { text: short ? "Missed" : "Resolved after its deadline", tone: "red" } : { text: short ? "Met" : "Resolved within its deadline", tone: "label2" };
  if (r.remainingMs <= 0) return { text: short ? `Overdue ${span(r.remainingMs)}` : `Overdue by ${span(r.remainingMs)}`, tone: "red" };
  if (t.sla_breached) return { text: short ? "Breached" : "Deadline breached", tone: "red" };
  return { text: short ? span(r.remainingMs) : `${span(r.remainingMs)} left`, tone: r.state === "warn" ? "orange" : "label2" };
}
