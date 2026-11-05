import { slaRemaining } from "./sla.ts";
import type { Me, Ticket } from "./types";

// Pure rules behind the Overview, My day and Documents screens. They mirror web/app/(app)/layout.tsx
// and web/app/(app)/dashboard/page.tsx so a role sees on the phone what it sees on the web, no more.

export const CLIENT_ROLES = ["client_admin", "client_user"];
const has = (me: Pick<Me, "roles">, ...r: string[]) => me.roles.some((x) => r.includes(x));

export interface Access {
  home: boolean;
  tickets: boolean;
  money: boolean;
  contracts: boolean;
  forms: boolean;
  documents: boolean;
  clientsLookup: boolean;
  report: boolean;
  reviewer: boolean;
  approver: boolean;
  timeUser: boolean;
}

export function accessFor(me: Pick<Me, "roles" | "scope">): Access {
  const staff = me.scope === "staff";
  const client = has(me, ...CLIENT_ROLES) && !staff;
  return {
    home: me.scope !== "other" && me.roles.some((r) => !["auditor", "supplier"].includes(r)),
    tickets: has(me, "ceo", "manager", "finance", "sales", "support", "engineer", "hr", "employee", "logistics", "procurement", ...CLIENT_ROLES),
    money: has(me, "ceo", "manager", "finance", "sales", ...CLIENT_ROLES),
    contracts: has(me, "ceo", "manager", "sales", "finance", "support", "engineer", ...CLIENT_ROLES),
    forms: has(me, "ceo", "manager", "sales", "support", "engineer", ...CLIENT_ROLES),
    // Staff see the whole register. A client sees their own company's Confidential and Public documents (the API
    // filters by tenant and class). The web has no client Documents page, so this is mobile-only and never wider than the API.
    documents: client || (staff && has(me, "ceo", "manager", "finance", "sales", "support", "engineer", "hr", "logistics", "procurement")),
    clientsLookup: has(me, "ceo", "manager", "sales", "support", "finance", "engineer"),
    report: has(me, "ceo", "manager", "support"),
    reviewer: has(me, "hr", "manager", "ceo"),
    approver: has(me, "manager", "ceo"),
    timeUser: has(me, "ceo", "manager", "finance", "sales", "support", "engineer", "hr", "employee"),
  };
}

export const isOpen = (t: Pick<Ticket, "status">) => t.status === "open" || t.status === "in_progress";

export function atRisk(open: Ticket[], now = Date.now()): number {
  return open.filter((t) => t.sla_breached || (slaRemaining(t.sla_due_at, now)?.overdue ?? false) || (!!t.sla_due_at && new Date(t.sla_due_at).getTime() - now < 4 * 3_600_000)).length;
}

/**
 * "Needs attention" on My day: open tickets that are at risk or overdue (the same rule as the At risk count)
 * and that this person can act on, meaning assigned to them or not yet assigned. Soonest deadline first.
 */
export function needsAttention(open: Ticket[], sub: string, now = Date.now(), n = 5): Ticket[] {
  const due = (t: Ticket) => (t.sla_due_at ? new Date(t.sla_due_at).getTime() : Infinity);
  return open
    .filter((t) => (t.assignee === sub || !t.assignee) && atRisk([t], now) === 1)
    .sort((a, b) => due(a) - due(b))
    .slice(0, n);
}

export function mondayOf(d: Date): Date {
  const x = new Date(d);
  x.setHours(0, 0, 0, 0);
  x.setDate(x.getDate() - ((x.getDay() + 6) % 7));
  return x;
}

export function hoursThisWeek(sheets: { work_date: string; hours: number }[], now = new Date()): number {
  const monday = mondayOf(now).getTime();
  return sheets.filter((s) => new Date(s.work_date).getTime() >= monday).reduce((a, s) => a + s.hours, 0);
}

export type QueueTab = "Mine" | "Team" | "Unassigned";
export function queueFor(open: Ticket[], tab: QueueTab, sub: string): Ticket[] {
  return tab === "Mine" ? open.filter((t) => t.assignee === sub) : tab === "Unassigned" ? open.filter((t) => !t.assignee) : open;
}

export interface Leave { id: string; actor: string; kind: string; start_date: string; days: number; status: string }
export interface Claim { id: string; actor: string; description: string; amount: number; status: string }
export interface Sheet { id: string; actor: string; work_date: string; hours: number; status: string }
export interface Approval { key: string; text: string }

/** Items waiting for this person to review. Nobody reviews their own request. */
export function approvalsFor(sub: string, leave: Leave[], claims: Claim[], sheets: Sheet[], fmtDay: (s: string) => string, fmtMoney: (n: number) => string): Approval[] {
  return [
    ...leave.filter((l) => l.status === "pending" && l.actor !== sub).map((l) => ({ key: "l" + l.id, text: `Leave request · ${l.actor} · ${l.days} day${l.days === 1 ? "" : "s"} from ${fmtDay(l.start_date)}` })),
    ...claims.filter((c) => c.status === "submitted" && c.actor !== sub).map((c) => ({ key: "c" + c.id, text: `Expense claim · ${c.actor} · ${fmtMoney(c.amount)}` })),
    ...sheets.filter((s) => s.status === "submitted" && s.actor !== sub).map((s) => ({ key: "s" + s.id, text: `Timesheet · ${s.actor} · ${fmtDay(s.work_date)}` })),
  ];
}

export interface QuoteRow { id: string; ref: string | null; title: string; status: string }
export interface InvoiceRow { id: string; ref: string | null; title: string; status: string; balance: number; due_date: string | null }
export interface FormRow { id: string; title: string; form_code: string; status: string; due_date: string | null }
export interface ContractRow { id: string; number: string; title: string; status: string; end_date: string }

export interface Action { key: string; kind: "quote" | "invoice" | "ticket" | "form" | "contract"; title: string; sub: string; label: string }

export const daysUntil = (iso: string, now = Date.now()) => Math.ceil((new Date(iso).getTime() - now) / 86_400_000);

/** The client's "Awaiting your action" list: quotes to approve, invoices due, open tickets, forms waiting, contracts ending in 90 days. */
export function clientActions(
  a: Pick<Access, "money" | "tickets" | "forms" | "contracts">,
  d: { quotes: QuoteRow[]; invoices: InvoiceRow[]; tickets: Ticket[]; forms: FormRow[]; contracts: ContractRow[] },
  now = Date.now(),
): Action[] {
  const out: Action[] = [];
  if (a.money) {
    for (const q of d.quotes.filter((q) => q.status === "sent")) out.push({ key: "q" + q.id, kind: "quote", title: `Quotation · ${q.title}`, sub: q.ref ?? "", label: "Review & approve" });
    for (const i of d.invoices.filter((i) => i.status === "issued" && i.balance > 0)) out.push({ key: "i" + i.id, kind: "invoice", title: `Invoice · ${i.title}`, sub: i.ref ?? "", label: "Pay" });
  }
  if (a.forms) {
    for (const f of d.forms.filter((f) => f.status === "sent" || f.status === "returned")) out.push({ key: "f" + f.id, kind: "form", title: `Form · ${f.title}`, sub: f.form_code, label: f.status === "returned" ? "Fix & resend" : "Fill in" });
  }
  if (a.contracts) {
    for (const c of d.contracts.filter((c) => c.status === "active")) {
      const left = daysUntil(c.end_date, now);
      if (left >= 0 && left <= 90) out.push({ key: "c" + c.id, kind: "contract", title: `Contract ending · ${c.title}`, sub: `${c.number} · ${left} day${left === 1 ? "" : "s"} left`, label: "Open" });
    }
  }
  if (a.tickets) {
    for (const t of d.tickets.filter(isOpen)) out.push({ key: "t" + t.id, kind: "ticket", title: `Open ticket · ${t.subject}`, sub: t.severity, label: "Open" });
  }
  return out;
}

export function clientKpis(tickets: Ticket[], invoices: InvoiceRow[], now = new Date()) {
  const open = tickets.filter(isOpen);
  const incidents = open.filter((t) => t.severity === "P1" || t.severity === "P2").length;
  const monthStart = new Date(now.getFullYear(), now.getMonth(), 1).getTime();
  const done = tickets.filter((t) => t.resolved_at && new Date(t.resolved_at).getTime() >= monthStart);
  const met = done.filter((t) => !t.sla_breached).length;
  const owing = invoices.filter((i) => i.status === "issued" && i.balance > 0);
  return { open: open.length, incidents, resolved: done.length, met, sla: done.length ? Math.round((met / done.length) * 100) : null, owing: owing.length, balance: owing.reduce((a, i) => a + i.balance, 0) };
}

export interface RegisterEntry { ref: string; code: string; title: string; status: "valid" | "revoked"; issued_by: string; issued_at: string }

/** Filters the register by reference, form code or title. */
export function filterRegister<T extends Pick<RegisterEntry, "ref" | "code" | "title">>(rows: T[], q: string): T[] {
  const n = q.trim().toLowerCase();
  return n ? rows.filter((r) => `${r.ref} ${r.code} ${r.title}`.toLowerCase().includes(n)) : rows;
}

/** Open tickets by severity, for the posture bar. */
export function sevCounts(open: Pick<Ticket, "severity">[]): Record<"P1" | "P2" | "P3" | "P4", number> {
  const out = { P1: 0, P2: 0, P3: 0, P4: 0 };
  for (const t of open) out[t.severity]++;
  return out;
}

/** The open tickets whose resolution deadline comes first (overdue ones first of all). */
export function nextDeadlines<T extends Pick<Ticket, "sla_due_at">>(open: T[], n: number): T[] {
  return open
    .filter((t) => !!t.sla_due_at)
    .sort((a, b) => new Date(a.sla_due_at as string).getTime() - new Date(b.sla_due_at as string).getTime())
    .slice(0, n);
}

/** The most recently issued register entries, newest first. */
export function latestDocuments<T extends Pick<RegisterEntry, "issued_at">>(rows: T[], n: number): T[] {
  return [...rows].sort((a, b) => new Date(b.issued_at).getTime() - new Date(a.issued_at).getTime()).slice(0, n);
}

export const HOME_ROUTE = "/(tabs)/home";
const LANDING = ["/home", "/tickets", "/report", "/field", "/documents", "/account"];

/** Where to go after sign-in: the in-app route that was asked for before the gate sent the person to sign in, else the home tab. */
export function landingRoute(requested: string | null | undefined): string {
  if (!requested) return HOME_ROUTE;
  const path = requested.split(/[?#]/)[0].replace(/^\/\(tabs\)/, "");
  if (LANDING.includes(path)) return requested;
  if ((path.startsWith("/ticket/") && path.length > 8) || (path.startsWith("/intervention/") && path.length > 14)) return requested;
  return HOME_ROUTE;
}
