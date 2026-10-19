"use client";
import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import type { Ticket } from "@/lib/types";
import { SeverityBadge, SlaChip, StatusBadge } from "@/components/Badges";

export default function Tickets() {
  const [rows, setRows] = useState<Ticket[]>([]);
  const [next, setNext] = useState<string | null>(null);
  const [status, setStatus] = useState("");
  const [severity, setSeverity] = useState("");
  const [err, setErr] = useState("");
  const load = useCallback(
    (cursor?: string) =>
      api.tickets({ status, severity, cursor }).then((r) => {
        setRows((prev) => (cursor ? [...prev, ...r.data] : r.data));
        setNext(r.next_cursor);
      }).catch((e) => setErr(e.message)),
    [status, severity],
  );
  useEffect(() => { void load(); }, [load]);
  return (
    <>
      <div className="row">
        <h1>Tickets</h1>
        <Link className="btn primary" href="/tickets/new">New</Link>
      </div>
      <div className="filters">
        <label>Status
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="">All</option>
            {["open", "in_progress", "resolved", "closed"].map((s) => <option key={s} value={s}>{s.replace("_", " ")}</option>)}
          </select>
        </label>
        <label>Severity
          <select value={severity} onChange={(e) => setSeverity(e.target.value)}>
            <option value="">All</option>
            {["P1", "P2", "P3", "P4"].map((s) => <option key={s}>{s}</option>)}
          </select>
        </label>
      </div>
      {err && <p className="error">{err}</p>}
      <div className="scroll">
        <table>
          <thead><tr><th>Subject</th><th>Sev</th><th>Status</th><th>Assignee</th><th>SLA</th><th>Opened</th></tr></thead>
          <tbody>
            {rows.map((t) => (
              <tr key={t.id}>
                <td><Link href={`/tickets/${t.id}`}>{t.subject}</Link></td>
                <td><SeverityBadge s={t.severity} /></td>
                <td><StatusBadge s={t.status} /></td>
                <td>{t.assignee ?? "–"}</td>
                <td><SlaChip due={t.sla_due_at} done={!!t.resolved_at} breached={t.sla_breached} /></td>
                <td>{new Date(t.created_at).toLocaleString()}</td>
              </tr>
            ))}
            {!rows.length && !err && <tr><td colSpan={6} className="muted">No tickets.</td></tr>}
          </tbody>
        </table>
      </div>
      {next && <button className="btn" onClick={() => load(next)}>Load more</button>}
    </>
  );
}
