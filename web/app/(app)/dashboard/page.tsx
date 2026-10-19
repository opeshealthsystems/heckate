"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import { useMe } from "@/lib/session";
import type { SupportReport, Ticket } from "@/lib/types";
import { SeverityBadge, SlaChip, StatusBadge } from "@/components/Badges";

export default function Dashboard() {
  const me = useMe();
  const staff = me.scope === "staff";
  const portal = me.roles.some((r) => !["auditor", "supplier"].includes(r));
  const [report, setReport] = useState<SupportReport | null>(null);
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [err, setErr] = useState("");
  useEffect(() => {
    if (!portal) return;
    api.tickets().then((r) => setTickets(r.data.filter((t) => t.status !== "closed").slice(0, 8))).catch((e) => setErr(e.message));
    if (me.roles.some((r) => ["ceo", "manager", "support"].includes(r))) api.report().then(setReport).catch(() => {});
  }, [me]);
  return (
    <>
      <h1>{!portal ? "Dashboard" : staff ? "Staff dashboard" : "Your support"}</h1>
      {report && (
        <section className="stats">
          <div className="stat"><span>{report.open}</span>Open</div>
          <div className="stat"><span>{report.p1_open}</span>P1 open</div>
          <div className="stat"><span className={report.sla_breached ? "bad" : ""}>{report.sla_breached}</span>SLA breached</div>
          <div className="stat"><span>{report.avg_rating?.toFixed(1) ?? "–"}</span>Avg rating</div>
        </section>
      )}
      {!portal && <p className="notice">Your role has no ticket queue. Use the menu to open {me.roles.includes("supplier") ? "the supplier portal" : "accounting and payroll (read-only)"}.</p>}
      {portal && <div className="row">
        <h2>{staff ? "Latest tickets" : "Your open tickets"}</h2>
        <Link className="btn primary" href="/tickets/new">{staff ? "New ticket" : "Report an incident"}</Link>
      </div>}
      {err && <p className="error">{err}</p>}
      {portal && <div className="scroll">
        <table>
          <thead><tr><th>Subject</th><th>Sev</th><th>Status</th><th>SLA</th></tr></thead>
          <tbody>
            {tickets.map((t) => (
              <tr key={t.id}>
                <td><Link href={`/tickets/${t.id}`}>{t.subject}</Link></td>
                <td><SeverityBadge s={t.severity} /></td>
                <td><StatusBadge s={t.status} /></td>
                <td><SlaChip due={t.sla_due_at} done={!!t.resolved_at} breached={t.sla_breached} /></td>
              </tr>
            ))}
            {!tickets.length && !err && <tr><td colSpan={4} className="muted">Nothing open.</td></tr>}
          </tbody>
        </table>
      </div>}
    </>
  );
}
