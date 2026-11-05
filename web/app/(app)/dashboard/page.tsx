"use client";
import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { api, raw } from "@/lib/api";
import { useMe } from "@/lib/session";
import { roleLabel, xaf, day } from "@/lib/format";
import { slaRemaining } from "@/lib/sla";
import type { Client, SupportReport, Ticket } from "@/lib/types";
import type { Invoice, Quote } from "@/lib/sales-types";
import { SeverityBadge, StatusBadge } from "@/components/Badges";
import { Tabs } from "@/components/ui";

interface Leave { id: string; actor: string; kind: string; start_date: string; days: number; status: string }
interface Claim { id: string; actor: string; description: string; amount: number; status: string }
interface Sheet { id: string; actor: string; work_date: string; hours: number; status: string }
interface Project { id: string; name: string; status: string; start_date: string | null; end_date: string | null }

const isOpen = (t: Ticket) => t.status === "open" || t.status === "in_progress";
const mondayOf = (d: Date) => { const x = new Date(d); x.setHours(0, 0, 0, 0); x.setDate(x.getDate() - ((x.getDay() + 6) % 7)); return x; };
const list = async <T,>(path: string): Promise<T[]> => (await raw.get<{ data: T[] }>(path).catch(() => ({ data: [] as T[] }))).data;

function Kpi({ label, value, sub, tone }: { label: string; value: string | number; sub?: string; tone?: "bad" | "ok" }) {
  return (
    <div className={`kpi ${tone === "bad" ? "alert" : ""}`}>
      <span className="k-label">{label}</span>
      <span className={`k-value ${tone ?? ""}`}>{value}</span>
      {sub && <span className={`k-sub ${tone ?? ""}`}>{sub}</span>}
    </div>
  );
}

function Panel({ title, action, children }: { title: string; action?: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="panel">
      <div className="panel-head"><h2>{title}</h2>{action}</div>
      {children}
    </section>
  );
}

function TimeLeft({ t }: { t: Ticket }) {
  if (t.resolved_at) return <span className="muted">Resolved</span>;
  const r = slaRemaining(t.sla_due_at);
  return r ? <span className={r.overdue ? "tl bad" : "tl"}>{r.label}</span> : <span className="muted">–</span>;
}

export default function Dashboard() {
  const me = useMe();
  const staff = me.scope === "staff";
  const portal = me.roles.some((r) => !["auditor", "supplier"].includes(r));
  const has = (...r: string[]) => me.roles.some((x) => r.includes(x));
  const reviewer = has("hr", "manager", "ceo");
  const approver = has("manager", "ceo");
  const timeUser = has("ceo", "manager", "finance", "sales", "support", "engineer", "hr", "employee");
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [report, setReport] = useState<SupportReport | null>(null);
  const [clients, setClients] = useState<Client[]>([]);
  const [tab, setTab] = useState("Mine");
  const [leave, setLeave] = useState<Leave[]>([]);
  const [claims, setClaims] = useState<Claim[]>([]);
  const [sheets, setSheets] = useState<Sheet[]>([]);
  const [mine, setMine] = useState<Sheet[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [quotes, setQuotes] = useState<Quote[]>([]);
  const [invoices, setInvoices] = useState<Invoice[]>([]);
  const [err, setErr] = useState("");

  useEffect(() => {
    if (!portal) return;
    api.tickets().then((r) => { setTickets(r.data); if (!r.data.some((t) => isOpen(t) && t.assignee === me.sub)) setTab("Team"); }).catch((e) => setErr(e.message));
    if (has("ceo", "manager", "support")) api.report().then(setReport).catch(() => {});
    if (has("ceo", "manager", "sales", "support", "finance", "engineer")) api.clients().then((r) => setClients(r.data)).catch(() => {});
    void list<Project>("/v1/projects").then(setProjects);
    if (staff) {
      if (reviewer) { void list<Leave>("/v1/leave?all=1").then(setLeave); void list<Claim>("/v1/expenses?all=1").then(setClaims); }
      if (approver) void list<Sheet>("/v1/timesheets?all=1").then(setSheets);
      if (timeUser) void list<Sheet>("/v1/timesheets").then(setMine);
    } else {
      void list<Quote>("/v1/quotes").then(setQuotes);
      void list<Invoice>("/v1/invoices").then(setInvoices);
    }
  }, [me]); // eslint-disable-line react-hooks/exhaustive-deps

  const clientName = useMemo(() => new Map(clients.map((c) => [c.tenant_id, c.legal_name])), [clients]);
  const open = tickets.filter(isOpen);
  const today = new Date().toLocaleDateString("en-GB", { weekday: "long", day: "numeric", month: "long" });

  if (!portal) {
    return (
      <>
        <h1>Dashboard</h1>
        <p className="notice">Your role has no ticket queue. Use the menu to open {has("supplier") ? "the supplier portal" : "accounting and payroll (read-only)"}.</p>
      </>
    );
  }

  const approvals = [
    ...leave.filter((l) => l.status === "pending" && l.actor !== me.sub).map((l) => ({ key: "l" + l.id, text: `Leave request · ${l.actor} · ${l.days} day${l.days === 1 ? "" : "s"} from ${day(l.start_date)}`, href: "/hr" })),
    ...claims.filter((c) => c.status === "submitted" && c.actor !== me.sub).map((c) => ({ key: "c" + c.id, text: `Expense claim · ${c.actor} · ${xaf(c.amount)}`, href: "/hr" })),
    ...sheets.filter((s) => s.status === "submitted" && s.actor !== me.sub).map((s) => ({ key: "s" + s.id, text: `Timesheet · ${s.actor} · ${day(s.work_date)}`, href: "/timesheets" })),
  ];

  if (staff) {
    const queue = tab === "Mine" ? open.filter((t) => t.assignee === me.sub) : tab === "Unassigned" ? open.filter((t) => !t.assignee) : open;
    const atRisk = open.filter((t) => t.sla_breached || (slaRemaining(t.sla_due_at)?.overdue ?? false) || (t.sla_due_at && new Date(t.sla_due_at).getTime() - Date.now() < 4 * 3_600_000)).length;
    const monday = mondayOf(new Date()).getTime();
    const hours = mine.filter((s) => new Date(s.work_date).getTime() >= monday).reduce((a, s) => a + s.hours, 0);
    const live = projects.filter((p) => p.status === "active" || p.status === "planning").slice(0, 5);
    return (
      <>
        <header className="page-head">
          <div><p className="meta">{today} · {me.roles.map(roleLabel).join(", ")}</p><h1>My day</h1></div>
          <Link className="btn primary" href="/tickets/new">New ticket</Link>
        </header>
        {err && <p className="error" role="alert">{err}</p>}
        <div className="kpis">
          <Kpi label="Assigned to me" value={open.filter((t) => t.assignee === me.sub).length} />
          <Kpi label="SLA at risk" value={atRisk} tone={atRisk ? "bad" : undefined} sub={report ? `${report.sla_breached} breached in total` : undefined} />
          {(reviewer || approver) && <Kpi label="Approvals waiting" value={approvals.length} />}
          {timeUser && <Kpi label="Hours this week" value={hours.toFixed(1)} />}
          {report && <Kpi label="Open tickets" value={report.open} sub={`${report.p1_open} P1`} />}
          {report && <Kpi label="Avg rating" value={report.avg_rating == null ? "–" : report.avg_rating.toFixed(1)} sub="client satisfaction, out of 5" />}
        </div>
        <Panel title="Ticket queue" action={<Tabs tabs={["Mine", "Team", "Unassigned"]} value={tab} onChange={setTab} />}>
          <div className="scroll flush">
            <table>
              <thead><tr><th scope="col">Subject</th><th scope="col">Client</th><th scope="col">Priority</th><th scope="col">Status</th><th scope="col">Time left</th></tr></thead>
              <tbody>
                {queue.slice(0, 8).map((t) => (
                  <tr key={t.id}>
                    <td><Link href={`/tickets/${t.id}`}>{t.subject}</Link></td>
                    <td>{clientName.get(t.tenant_id) ?? "–"}</td>
                    <td><SeverityBadge s={t.severity} /></td>
                    <td><StatusBadge s={t.status} /></td>
                    <td><TimeLeft t={t} /></td>
                  </tr>
                ))}
                {!queue.length && !err && <tr><td colSpan={5} className="muted">Nothing here.</td></tr>}
              </tbody>
            </table>
          </div>
          {queue.length > 8 && <p className="panel-foot"><Link href="/tickets">View all {queue.length}</Link></p>}
        </Panel>
        <div className="two">
          {(reviewer || approver) && (
            <Panel title="Approvals">
              <ul className="rows">
                {approvals.slice(0, 6).map((a) => (<li key={a.key}><span>{a.text}</span><Link href={a.href}>Review</Link></li>))}
                {!approvals.length && <li className="muted">Nothing waiting for you.</li>}
              </ul>
            </Panel>
          )}
          <Panel title="Projects" action={<Link href="/projects">All projects</Link>}>
            <ul className="rows">
              {live.map((p) => {
                const s = p.start_date && p.end_date ? new Date(p.start_date).getTime() : null;
                const e = p.end_date ? new Date(p.end_date).getTime() : null;
                const pct = s !== null && e !== null && e > s ? Math.max(0, Math.min(100, Math.round(((Date.now() - s) / (e - s)) * 100))) : null;
                return (
                  <li key={p.id} className="proj">
                    <div className="row"><Link href={`/projects/${p.id}`}>{p.name}</Link><span className="muted">{p.status.replace(/_/g, " ")}</span></div>
                    {pct !== null && <div className="bar" role="img" aria-label={`${pct}% of the planned time has passed`}><i style={{ width: pct + "%" }} /></div>}
                  </li>
                );
              })}
              {!live.length && <li className="muted">No active projects.</li>}
            </ul>
          </Panel>
        </div>
      </>
    );
  }

  // Client portal overview
  const incidents = open.filter((t) => t.severity === "P1" || t.severity === "P2");
  const monthStart = new Date(new Date().getFullYear(), new Date().getMonth(), 1).getTime();
  const doneThisMonth = tickets.filter((t) => t.resolved_at && new Date(t.resolved_at).getTime() >= monthStart);
  const met = doneThisMonth.filter((t) => !t.sla_breached).length;
  const owing = invoices.filter((i) => i.status === "issued" && i.balance > 0);
  const balance = owing.reduce((a, i) => a + i.balance, 0);
  const toApprove = quotes.filter((q) => q.status === "sent");
  const actions = [
    ...toApprove.map((q) => ({ key: "q" + q.id, title: `Quotation · ${q.title}`, ref: q.ref, label: "Review & approve", href: "/quotes", primary: true })),
    ...owing.map((i) => ({ key: "i" + i.id, title: `Invoice · ${i.title}`, ref: i.ref, label: "Pay", href: "/invoices", primary: false })),
  ];
  return (
    <>
      <header className="page-head">
        <div><p className="meta">{today}</p><h1>Overview</h1></div>
        <Link className="btn danger" href="/tickets/new">Report incident</Link>
      </header>
      {err && <p className="error" role="alert">{err}</p>}
      <div className="kpis">
        <Kpi label="Open tickets" value={open.length} sub={incidents.length ? `${incidents.length} incident${incidents.length === 1 ? "" : "s"} in progress` : "No incidents"} />
        <Kpi label="SLA this month" value={doneThisMonth.length ? `${Math.round((met / doneThisMonth.length) * 100)}%` : "–"} tone={doneThisMonth.length && met === doneThisMonth.length ? "ok" : undefined} sub={doneThisMonth.length ? (met === doneThisMonth.length ? "All targets met" : `${doneThisMonth.length - met} missed`) : "Nothing resolved yet"} />
        <Kpi label="Balance due" value={xaf(balance)} sub={owing.length ? `${owing.length} invoice${owing.length === 1 ? "" : "s"}` : "Nothing owed"} />
      </div>
      <Panel title="Tickets & incidents" action={<Link href="/tickets">View all</Link>}>
        <div className="scroll flush">
          <table>
            <thead><tr><th scope="col">Subject</th><th scope="col">Priority</th><th scope="col">Status</th><th scope="col">Target</th></tr></thead>
            <tbody>
              {tickets.slice(0, 6).map((t) => (
                <tr key={t.id}>
                  <td><Link href={`/tickets/${t.id}`}>{t.subject}</Link></td>
                  <td><SeverityBadge s={t.severity} /></td>
                  <td><StatusBadge s={t.status} /></td>
                  <td><TimeLeft t={t} /></td>
                </tr>
              ))}
              {!tickets.length && !err && <tr><td colSpan={4} className="muted">No tickets yet.</td></tr>}
            </tbody>
          </table>
        </div>
      </Panel>
      <Panel title="Awaiting your action">
        <ul className="rows">
          {actions.slice(0, 6).map((a) => (
            <li key={a.key}>
              <span><strong>{a.title}</strong>{a.ref && <><br /><code>{a.ref}</code></>}</span>
              <Link className={a.primary ? "btn primary" : "btn"} href={a.href}>{a.label}</Link>
            </li>
          ))}
          {!actions.length && <li className="muted">Nothing is waiting for you.</li>}
        </ul>
      </Panel>
    </>
  );
}
