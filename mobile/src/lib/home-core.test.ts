import { test } from "node:test";
import assert from "node:assert/strict";
import { accessFor, approvalsFor, atRisk, clientActions, clientKpis, filterRegister, hoursThisWeek, landingRoute, latestDocuments, nextDeadlines, queueFor, sevCounts } from "./home-core.ts";
import type { Ticket } from "./types";

const now = Date.parse("2026-10-19T10:00:00Z");
const t = (o: Partial<Ticket>): Ticket => ({ id: "1", tenant_id: "x", subject: "S", description: "", severity: "P3", channel: "app", status: "open", reporter: "r", assignee: null, respond_due_at: null, sla_due_at: null, first_response_at: null, resolved_at: null, sla_breached: false, rating: null, rating_comment: null, version: 1, created_at: "", ...o });

test("RBAC mirrors the web menu", () => {
  const client = accessFor({ roles: ["client_user"], scope: "client" });
  assert.ok(client.money && client.contracts && client.forms && client.tickets && client.documents);
  assert.ok(!accessFor({ roles: ["supplier"], scope: "other" }).documents);
  const eng = accessFor({ roles: ["engineer"], scope: "staff" });
  assert.ok(!eng.money && eng.contracts && eng.forms && eng.documents && eng.timeUser);
  const hr = accessFor({ roles: ["hr"], scope: "staff" });
  assert.ok(!hr.contracts && !hr.forms && hr.reviewer && !hr.approver);
  const aud = accessFor({ roles: ["auditor"], scope: "staff" });
  assert.ok(!aud.home && !aud.tickets && !aud.documents);
  assert.ok(!accessFor({ roles: ["supplier"], scope: "other" }).home);
});

test("SLA at risk counts overdue, breached and due within 4h", () => {
  const rows = [t({ sla_due_at: "2026-10-19T09:00:00Z" }), t({ sla_due_at: "2026-10-19T13:00:00Z" }), t({ sla_due_at: "2026-10-20T13:00:00Z" }), t({ sla_breached: true })];
  assert.equal(atRisk(rows, now), 3);
});

test("queue tabs", () => {
  const rows = [t({ id: "a", assignee: "me" }), t({ id: "b" }), t({ id: "c", assignee: "you" })];
  assert.deepEqual(queueFor(rows, "Mine", "me").map((x) => x.id), ["a"]);
  assert.deepEqual(queueFor(rows, "Unassigned", "me").map((x) => x.id), ["b"]);
  assert.equal(queueFor(rows, "Team", "me").length, 3);
});

test("hours this week start on Monday", () => {
  const wed = new Date(2026, 9, 21, 12);
  const sheets = [{ work_date: "2026-10-19T00:00:00", hours: 8 }, { work_date: "2026-10-21T00:00:00", hours: 3.5 }, { work_date: "2026-10-16T00:00:00", hours: 8 }];
  assert.equal(hoursThisWeek(sheets, wed), 11.5);
});

test("approvals skip your own requests", () => {
  const res = approvalsFor("me",
    [{ id: "1", actor: "me", kind: "annual", start_date: "d", days: 1, status: "pending" }, { id: "2", actor: "ann", kind: "annual", start_date: "d", days: 2, status: "pending" }],
    [{ id: "3", actor: "bob", description: "", amount: 5000, status: "submitted" }, { id: "4", actor: "bob", description: "", amount: 1, status: "approved" }],
    [{ id: "5", actor: "cy", work_date: "d", hours: 8, status: "submitted" }], (s) => s, (n) => `${n} XAF`);
  assert.deepEqual(res.map((a) => a.key), ["l2", "c3", "s5"]);
  assert.match(res[0].text, /2 days/);
});

const data = {
  quotes: [{ id: "q1", ref: "HS-QT-1", title: "Pentest", status: "sent" }, { id: "q2", ref: null, title: "Old", status: "approved" }],
  invoices: [{ id: "i1", ref: "HS-INV-1", title: "Oct", status: "issued", balance: 1000, due_date: null }, { id: "i2", ref: null, title: "Paid", status: "paid", balance: 0, due_date: null }],
  tickets: [t({ id: "t1", subject: "Mail down", severity: "P2" }), t({ id: "t2", status: "resolved" })],
  forms: [{ id: "f1", title: "KYC", form_code: "HS-FRM-13", status: "sent", due_date: null }, { id: "f2", title: "Done", form_code: "X", status: "accepted", due_date: null }],
  contracts: [{ id: "c1", number: "C-1", title: "MSA", status: "active", end_date: "2026-11-18T00:00:00Z" }, { id: "c2", number: "C-2", title: "Far", status: "active", end_date: "2027-10-01T00:00:00Z" }, { id: "c3", number: "C-3", title: "Gone", status: "terminated", end_date: "2026-11-01T00:00:00Z" }],
};

test("client actions cover quotes, invoices, forms, contracts and open tickets", () => {
  const a = accessFor({ roles: ["client_admin"], scope: "client" });
  assert.deepEqual(clientActions(a, data, now).map((x) => x.key), ["qq1", "ii1", "ff1", "cc1", "tt1"]);
});

test("a role without money or forms does not see them", () => {
  const a = accessFor({ roles: ["engineer"], scope: "staff" });
  const keys = clientActions({ ...a, money: false, forms: false }, data, now).map((x) => x.kind);
  assert.deepEqual(keys, ["contract", "ticket"]);
});

test("client KPIs", () => {
  const k = clientKpis([t({ severity: "P1" }), t({ resolved_at: "2026-10-05T00:00:00Z", status: "resolved" }), t({ resolved_at: "2026-10-06T00:00:00Z", status: "resolved", sla_breached: true })], data.invoices, new Date(2026, 9, 19));
  assert.deepEqual(k, { open: 1, incidents: 1, resolved: 2, met: 1, sla: 50, owing: 1, balance: 1000 });
  assert.equal(clientKpis([], [], new Date()).sla, null);
});

test("register search", () => {
  const rows = [{ ref: "HS-INV-2026-000012", code: "INV", title: "Invoice October" }, { ref: "HS-QT-2026-000003", code: "QT", title: "Pentest" }];
  assert.equal(filterRegister(rows, " pentest ").length, 1);
  assert.equal(filterRegister(rows, "hs-inv").length, 1);
  assert.equal(filterRegister(rows, "").length, 2);
});

test("latestDocuments returns the newest first and caps the count", () => {
  const rows = [{ issued_at: "2026-01-01T00:00:00Z", n: 1 }, { issued_at: "2026-03-01T00:00:00Z", n: 3 }, { issued_at: "2026-02-01T00:00:00Z", n: 2 }];
  assert.deepEqual(latestDocuments(rows, 2).map((r) => r.n), [3, 2]);
  assert.equal(rows[0].n, 1, "input is not mutated");
});

test("landingRoute keeps a requested in-app route and defaults to home", () => {
  assert.equal(landingRoute(undefined), "/(tabs)/home");
  assert.equal(landingRoute("/"), "/(tabs)/home");
  assert.equal(landingRoute("/login"), "/(tabs)/home");
  assert.equal(landingRoute("/documents"), "/documents");
  assert.equal(landingRoute("/(tabs)/documents"), "/(tabs)/documents");
  assert.equal(landingRoute("/ticket/abc"), "/ticket/abc");
  assert.equal(landingRoute("/ticket/"), "/(tabs)/home");
  assert.equal(landingRoute("//evil.example"), "/(tabs)/home");
});

test("sevCounts and nextDeadlines", () => {
  const open = [t({ severity: "P2", sla_due_at: "2026-01-02T00:00:00Z" }), t({ severity: "P1", sla_due_at: "2026-01-01T00:00:00Z" }), t({ severity: "P3" }), t({ severity: "P2", sla_due_at: "2026-01-03T00:00:00Z" })];
  assert.deepEqual(sevCounts(open), { P1: 1, P2: 2, P3: 1, P4: 0 });
  assert.deepEqual(nextDeadlines(open, 2).map((x) => x.sla_due_at), ["2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z"]);
});
