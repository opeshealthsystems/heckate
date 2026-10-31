// Payroll in the browser: finance prepares a month, a second finance user approves it, a third
// person pays it, and the employee then finds the payslip under People. Each role sees only the
// buttons it is allowed to press.
import { chromium } from "playwright-core";
import crypto from "node:crypto";

const API = process.env.API ?? "http://localhost:8080";
const WEB = process.env.WEB ?? "http://localhost:3000";
const SECRET = process.env.HEKATE_DEV_JWT_SECRET ?? "e2e-secret";
const b64 = (o) => Buffer.from(JSON.stringify(o)).toString("base64url");
function token(sub, roles, tenant = "") {
  const head = b64({ alg: "HS256", typ: "JWT" });
  const body = b64({ sub, roles, tenant_id: tenant, exp: Math.floor(Date.now() / 1000) + 3600 });
  return `${head}.${body}.${crypto.createHmac("sha256", SECRET).update(`${head}.${body}`).digest("base64url")}`;
}
async function api(method, path, tok, body) {
  const res = await fetch(API + path, { method, headers: { Authorization: `Bearer ${tok}`, "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: body && JSON.stringify(body) });
  return { status: res.status, body: await res.json().catch(() => ({})) };
}
const ok = (c, m) => { console.log(c ? "PASS" : "FAIL", m); if (!c) process.exitCode = 1; };

const b = await chromium.launch({ executablePath: process.env.CHROMIUM ?? "/opt/pw-browsers/chromium", args: ["--no-sandbox"] });
async function session(tok) {
  const ctx = await b.newContext({ bypassCSP: true, viewport: { width: 1280, height: 900 } });
  const p = await ctx.newPage();
  await p.goto(`${WEB}/login`);
  await p.fill("#tok", tok);
  await p.click("text=Continue");
  await p.waitForURL("**/dashboard");
  return p;
}
const go = async (p, path) => { await p.goto(WEB + path); await p.waitForLoadState("networkidle"); await p.waitForSelector("h1"); };

const n = Date.now();
const emp = `e2e-pay-${n}`;
const hr = token("e2e-pay-hr", ["hr"]);
const fin1 = token("e2e-pay-f1", ["finance"]);
const fin2 = token("e2e-pay-f2", ["finance"]);
const ceo = token("e2e-pay-ceo", ["ceo"]);
const me = token(emp, ["employee"]);
const month = `${1995 + (n % 25)}-${String(1 + (Math.floor(n / 25) % 12)).padStart(2, "0")}`;

const e = (await api("POST", "/v1/employees", hr, { actor: emp, full_name: `Pay Test ${n}`, hire_date: "1990-01-01" })).body;
await api("PUT", `/v1/payroll/compensation/${e.id}`, hr, { effective_from: "1990-01-01", base_salary: 900000, allowances: 100000 });
// Rules: edited by one finance user, validated by another.
const cfg = (await api("GET", "/v1/payroll/config", fin1)).body.config;
await api("PUT", "/v1/payroll/config", fin1, { ...cfg, cac_bp: cfg.cac_bp === 1000 ? 1001 : 1000 });
await api("PUT", "/v1/payroll/config", fin1, cfg);
await api("POST", "/v1/payroll/config/validate", fin2);

const f1 = await session(fin1);
await go(f1, "/payroll");
await f1.fill("#pr-period", month);
await f1.click("text=Prepare draft");
await f1.waitForSelector(`text=Pay Test ${n}`);
ok((await f1.innerText("body")).replace(/\s/g, "").includes("806126XAF"), "payslip for 1,000,000 gross shows net 806,126 XAF");
ok((await f1.locator("button:has-text('Approve and post to ledger')").count()) === 0, "preparer is not offered the approve button");
ok((await f1.innerText("body")).includes("Someone else must approve"), "preparer is told someone else must approve");

const f2 = await session(fin2);
await go(f2, "/payroll");
await f2.click(`tr:has-text("${month}") >> text=Open`);
await f2.click("button:has-text('Approve and post to ledger')");
await f2.waitForSelector(`text=approved by e2e-pay-f2`);
ok(true, "second finance user approved and posted the run");
ok((await f2.locator("button:has-text('Record salaries as paid')").count()) === 0, "approver cannot also pay");

const c = await session(ceo);
await go(c, "/payroll");
await c.click(`tr:has-text("${month}") >> text=Open`);
await c.click("button:has-text('Record salaries as paid')");
await c.waitForSelector("text=paid by e2e-pay-ceo");
ok(true, "a third person recorded the payment");

const w = await session(me);
await go(w, "/hr");
await w.click("[role=tab]:has-text('My payslips')");
await w.waitForSelector(`td:has-text("${month}")`);
ok((await w.innerText("body")).replace(/\s/g, "").includes("806126XAF"), "employee sees their own payslip with net pay");
ok((await w.locator("a[href='/payroll']").count()) === 0, "employee has no Payroll menu");
await b.close();
