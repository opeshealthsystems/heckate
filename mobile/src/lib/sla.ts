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

const p2 = (n: number) => String(n).padStart(2, "0");

/** "04:12:09" for under 100 hours, "3d 04h" beyond; negative values are shown as their absolute size. */
export function countdown(ms: number): string {
  const abs = Math.abs(ms);
  const s = Math.floor(abs / 1000);
  const h = Math.floor(s / 3600);
  if (h >= 100) return `${Math.floor(h / 24)}d ${p2(h % 24)}h`;
  return `${p2(h)}:${p2(Math.floor((s % 3600) / 60))}:${p2(s % 60)}`;
}

/** Circle stroke maths: dash length of the used arc and the gap that follows for a given radius. */
export function arcDash(used: number, radius: number): { dash: number; gap: number; circumference: number } {
  const circumference = 2 * Math.PI * radius;
  const dash = Math.min(1, Math.max(0, used)) * circumference;
  return { dash, gap: circumference - dash, circumference };
}

/** Posture score for the Home header: the share of open tickets that are not at risk, or null with nothing open. */
export function postureScore(open: number, atRisk: number): number | null {
  if (open <= 0) return null;
  return Math.round(((open - Math.min(atRisk, open)) / open) * 100);
}
