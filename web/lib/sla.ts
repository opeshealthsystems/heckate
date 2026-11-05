export function slaRemaining(due: string | null, now = Date.now()): { label: string; overdue: boolean } | null {
  if (!due) return null;
  const ms = new Date(due).getTime() - now;
  const abs = Math.abs(ms);
  const h = Math.floor(abs / 3_600_000);
  const m = Math.floor((abs % 3_600_000) / 60_000);
  const text = h >= 48 ? `${Math.floor(h / 24)}d` : h > 0 ? `${h}h ${m}m` : `${m}m`;
  return ms < 0 ? { label: `${text} overdue`, overdue: true } : { label: `${text} left`, overdue: false };
}
