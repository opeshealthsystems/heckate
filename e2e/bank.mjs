// Bank reconciliation in the browser: finance imports a statement, matches the unambiguous lines
// automatically, picks the right ledger line for the ambiguous one, and sees what is still open.
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

const fin = token("e2e-bank-fin", ["finance"]);
const acct = `5${String(Date.now() % 10_000_000).padStart(7, "0")}`;
await api("POST", "/v1/accounting/accounts", fin, { code: acct, name: `Bank e2e ${acct}` });
const post = (date, dr, cr) => api("POST", "/v1/accounting/entries", fin, { journal: "BQ", entry_date: date, narration: `e2e ${date}`, lines: dr ? [{ account: acct, debit: dr }, { account: "101", credit: dr }] : [{ account: acct, credit: cr }, { account: "101", debit: cr }] });
await post("2026-03-01", 300000, 0);
await post("2026-03-05", 125000, 0);
await post("2026-03-05", 125000, 0);
await post("2026-03-09", 0, 40000);

const b = await chromium.launch({ executablePath: process.env.CHROMIUM ?? "/opt/pw-browsers/chromium", args: ["--no-sandbox"] });
const p = await (await b.newContext({ bypassCSP: true, viewport: { width: 1280, height: 900 } })).newPage();
await p.goto(`${WEB}/login`);
await p.fill("#tok", fin);
await p.click("text=Continue");
await p.waitForURL("**/dashboard");
await p.goto(`${WEB}/accounting`);
await p.waitForSelector("h1");
await p.click("[role=tab]:has-text('Bank')");
await p.fill("#bk-acct", acct);
await p.fill("#bk-open", "0");
await p.fill("#bk-close", "385000");
await p.fill("#bk-csv", "2026-03-02,TRANSFER IN ACME,300000,TR1\n2026-03-06,TRANSFER IN GLOBEX,125000,TR2\n2026-03-09,BANK CHARGES,-40000,");
await p.click("button:has-text('Import')");
await p.waitForSelector("text=TRANSFER IN GLOBEX");
ok((await p.locator("button:has-text('Match 05')").count()) === 0 && (await p.locator("button:has-text('Match')").count()) >= 3, "ambiguous line offers two candidates, others one each");
await p.click("button:has-text('Match the unambiguous lines')");
await p.waitForSelector("text=2 line(s) matched automatically");
ok(true, "unambiguous lines matched automatically");
await p.click("tr:has-text('TRANSFER IN GLOBEX') button:has-text('Match')");
await p.waitForSelector("tr:has-text('TRANSFER IN GLOBEX') >> text=matched");
ok((await p.innerText("section[aria-label=Reconciliation]")).includes("open items"), "one identical receipt is still outstanding, so not reconciled");
ok((await p.locator("section[aria-label=Reconciliation] h3 + div, section[aria-label=Reconciliation] table").count()) >= 2, "outstanding ledger lines are listed");
await b.close();
