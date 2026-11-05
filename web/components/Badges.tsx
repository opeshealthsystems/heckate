import type { Severity, Status } from "@/lib/types";
import { slaRemaining } from "@/lib/sla";

export const SeverityBadge = ({ s }: { s: Severity }) => <span className={`badge sev-${s}`}>{s}</span>;
export const StatusBadge = ({ s }: { s: Status }) => <span className={`badge st-${s}`}>{s.replace("_", " ")}</span>;

export function SlaChip({ due, done, breached }: { due: string | null; done: boolean; breached: boolean }) {
  if (done) return <span className={`chip ${breached ? "bad" : "ok"}`}>{breached ? "SLA missed" : "SLA met"}</span>;
  const r = slaRemaining(due);
  if (!r) return null;
  return <span className={`chip ${r.overdue ? "bad" : "ok"}`}>{r.label}</span>;
}
