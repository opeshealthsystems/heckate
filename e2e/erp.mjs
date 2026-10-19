// Drives the ERP screens end to end: quote -> client approval -> invoice -> payment -> ledger.
// Needs the API (HEKATE_DEV_JWT_SECRET set) and the web app running, as described in e2e/README.md.
import { chromium } from "playwright-core";
import crypto from "node:crypto";

const API = process.env.API ?? "http://localhost:8080";
const WEB = process.env.WEB ?? "http://localhost:3000";
const SECRET = process.env.HEKATE_DEV_JWT_SECRET ?? "e2e-secret";
const b64 = (o) => Buffer.from(JSON.stringify(o)).toString("base64url");
function token(sub, roles, tenant = "") {
  const head = b64({ alg: "HS256", typ: "JWT" });
  const body = b64({ sub, roles, tenant_id: tenant, exp: Math.floor(Date.now() / 1000) + 3600 });
  const sig = crypto.createHmac("sha256", SECRET).update(`${head}.${body}`).digest("base64url");
  return `${head}.${body}.${sig}`;
}
async function api(method, path, tok, body) {
  const res = await fetch(API + path, { method, headers: { Authorization: `Bearer ${tok}`, "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: body && JSON.stringify(body) });
  return { status: res.status, body: await res.json().catch(() => ({})) };
}

// Numbers are formatted with narrow no-break spaces, so compare with all whitespace removed.
const waitText = (p, text) => p.waitForFunction((t) => document.body.innerText.replace(/\s/g, "").includes(t), text);
const ok = (c, m) => { console.log(c ? "PASS" : "FAIL", m); if (!c) process.exitCode = 1; };
const b = await chromium.launch({ executablePath: process.env.CHROMIUM ?? "/opt/pw-browsers/chromium", args: ["--no-sandbox"] });
async function session(tok, width = 1280) {
  const ctx = await b.newContext({ viewport: { width, height: 900 } });
  const p = await ctx.newPage();
  p.on("pageerror", (e) => console.log("pageerror:", e.message));
  await go(p, "/login");
  await p.fill("#tok", tok);
  await p.click("text=Continue");
  await p.waitForURL("**/dashboard");
  return p;
}

// Navigate and wait for the page's data requests to settle before interacting.
const go = async (p, path) => { await p.goto(WEB + path); await p.waitForLoadState("networkidle"); await p.waitForSelector("h1"); };

const sfx = Date.now();
const mgr = token("e2e-mgr", ["manager"]);
const sales = token("e2e-sales", ["sales"]);
const fin = token("e2e-fin", ["finance"]);
const client = await api("POST", "/v1/clients", mgr, { legal_name: `E2E Client ${sfx}` });
const tid = client.body.tenant_id;
const admin = token("e2e-admin", ["client_admin"], tid);

// 1. Sales drafts and sends a quotation.
const s = await session(sales);
await go(s, "/quotes");
await s.selectOption("#q-client", { label: `E2E Client ${sfx}` });
await s.fill("#q-title", `E2E pentest ${sfx}`);
await s.fill('[aria-label="Description 1"]', "External penetration test");
await s.fill('[aria-label="Unit price 1"]', "1000000");
await s.click("text=Create draft");
await s.click(`text=E2E pentest ${sfx}`);
await waitText(s, "1192500XAF");
ok((await s.locator(".totals").innerText()).replace(/\s/g, "").includes("1192500XAF"), "quote total is 1,192,500 XAF (TVA + CAC)");
await s.click("text=Send to client");
await s.waitForSelector("text=HS-QT-");
ok(true, "quote sent and given an HS-QT reference");

// 2. The client admin approves it in their portal.
const c = await session(admin);
await go(c, "/quotes");
await c.click(`text=E2E pentest ${sfx}`);
await c.click("button:has-text('Approve')");
await c.waitForSelector("text=approved");
ok(true, "client admin approved the quote");

// 3. Finance creates the invoice from the quote, issues it and records a MoMo payment.
const f = await session(token("e2e-fin2", ["finance", "sales"]));
await go(f, "/quotes");
await f.click(`text=E2E pentest ${sfx}`);
await f.click("text=Create invoice");
await f.waitForURL(/\/invoices\//);
await f.click("button:has-text('Issue')");
await f.waitForSelector("text=HS-INV-");
ok(true, "invoice issued with an HS-INV reference");
await f.fill("#p-amt", "500000");
await f.selectOption("#p-method", "momo");
await f.click("text=Record payment");
await waitText(f, "balance692500XAF");
ok(true, "payment recorded; balance 692,500 XAF");

// 4. The client sees the issued invoice and what is outstanding.
await go(c, "/invoices");
await c.click(`text=E2E pentest ${sfx}`);
await c.waitForSelector("text=Paid");
ok((await c.innerText("body")).replace(/\s/g, "").includes("692500XAF"), "client sees the outstanding balance");
ok((await c.locator("text=Record a payment").count()) === 0, "client cannot record payments");

// 5. Accounting: balanced trial balance, intact chain.
const a = await session(fin);
await go(a, "/accounting");
await a.waitForSelector("text=balanced");
ok((await a.innerText("body")).includes("5211"), "mobile-money receipt posted to account 5211");
ok((await a.locator(".chip.bad").count()) === 0, "trial balance is balanced");
await a.click("[role=tab]:has-text('Integrity')");
await a.click("text=Verify the ledger");
await a.waitForSelector("text=Intact");
ok(true, "ledger hash chain verified intact");

// 6. HR: an employee requests leave and a manager approves it.
const emp = `e2e-emp-${sfx}`;
await api("POST", "/v1/employees", token("e2e-hr", ["hr"]), { actor: emp, full_name: "E2E Employee", hire_date: "2020-01-01" });
const e = await session(token(emp, ["employee"]));
await go(e, "/hr");
await e.waitForSelector("text=18 of 18 days left");
await e.fill("#lv-from", "2026-12-14");
await e.fill("#lv-to", "2026-12-16");
await e.click("button:has-text('Request')");
await e.waitForSelector("text=pending");
const m = await session(mgr);
await go(m, "/hr");
await m.click("[role=tab]:has-text('Approvals')");
await m.click("button:has-text('Approve')");
await m.waitForSelector("text=approved");
await e.reload();
await e.waitForSelector("text=15 of 18 days left");
ok(true, "leave approved and the balance fell by 3 working days");

// 7. Small screens: no sideways page scroll on the dense screens.
const phone = await session(token("e2e-phone", ["finance", "manager", "procurement", "hr", "logistics"]), 360);
for (const path of ["/invoices", "/quotes", "/accounting", "/procurement", "/inventory", "/hr", "/projects", "/interventions", "/timesheets"]) {
  await go(phone, path);
  ok(new URL(phone.url()).pathname === path, `stayed signed in on ${path} (not bounced to /login)`);
  const fits = await phone.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth);
  ok(fits, `no horizontal page scroll at 360px on ${path}`);
}
await phone.screenshot({ path: "erp-mobile.png" });
await b.close();
