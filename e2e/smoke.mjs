import { chromium } from "playwright-core";
import crypto from "node:crypto";

const API = process.env.API ?? "http://localhost:8080";
const SECRET = process.env.HEKATE_DEV_JWT_SECRET ?? "e2e-secret";
const b64 = (o) => Buffer.from(JSON.stringify(o)).toString("base64url");
function token(sub, roles, tenant = "") {
  const head = b64({ alg: "HS256", typ: "JWT" });
  const body = b64({ sub, roles, tenant_id: tenant, exp: Math.floor(Date.now() / 1000) + 3600 });
  return `${head}.${body}.${crypto.createHmac("sha256", SECRET).update(`${head}.${body}`).digest("base64url")}`;
}
const staff = token("smoke-staff", ["manager", "support"]);
const created = await fetch(`${API}/v1/clients`, { method: "POST", headers: { Authorization: `Bearer ${staff}`, "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify({ legal_name: `Smoke Client ${Date.now()}` }) });
const client = token("smoke-client", ["client_user"], (await created.json()).tenant_id);
const b = await chromium.launch({ executablePath: process.env.CHROMIUM ?? "/opt/pw-browsers/chromium", args: ["--no-sandbox"] });
const ok = (c, m) => { console.log(c ? "PASS" : "FAIL", m); if (!c) process.exitCode = 1; };

async function login(tok, w = 1280) {
  const ctx = await b.newContext({ bypassCSP: true, viewport: { width: w, height: 800 } });
  const p = await ctx.newPage();
  p.on("pageerror", (e) => console.log("pageerror", e.message));
  await p.goto("http://localhost:3000/login");
  await p.fill("#tok", tok);
  await p.click("text=Continue");
  await p.waitForURL("**/dashboard");
  return p;
}

// client reports a P1
const c = await login(client);
await c.waitForSelector("text=Your support");
await c.click("text=Report an incident");
await c.selectOption("#sev", "P1");
await c.fill("#subject", "Core switch is down");
await c.fill("#desc", "Whole site offline");
await c.click("text=Submit");
await c.waitForURL(/\/tickets\/[0-9a-f-]{36}/);
await c.waitForSelector("h1:has-text('Core switch is down')");
ok(await c.locator(".sev-P1").count() === 1, "client created P1 ticket");
const url = c.url();

// staff picks it up, replies, adds internal note, resolves
const s = await login(staff);
await s.goto(url);
await s.click("text=Start work");
await s.waitForSelector("text=in progress");
await s.fill("#reply", "Engineer dispatched");
await s.click("button:has-text('Send')");
await s.waitForSelector("text=Engineer dispatched");
await s.fill("#reply", "Likely PSU failure");
await s.check("text=Internal note");
await s.click("button:has-text('Send')");
await s.waitForSelector("text=internal note");
await s.click("text=Mark resolved");
await s.waitForSelector("text=SLA met");
ok(true, "staff worked the ticket");

// client sees public reply only, then confirms + rates
await c.goto(url);
await c.waitForSelector("text=Engineer dispatched");
ok((await c.locator("text=Likely PSU failure").count()) === 0, "client cannot see internal note");
await c.click("text=Confirm resolved");
await c.waitForSelector("text=Rate this support");
await c.selectOption("#rt", "4");
await c.locator("section button:has-text('Submit')").click();
await c.waitForSelector("text=You rated this 4/5");
ok(true, "client confirmed and rated");

// staff dashboard shows the report
await s.goto("http://localhost:3000/dashboard");
await s.waitForSelector("text=Avg rating");
ok((await s.locator(".stat", { hasText: "Avg rating" }).innerText()).includes("4.0"), "dashboard avg rating 4.0");

// client cannot reach staff pages' data
await c.goto("http://localhost:3000/clients");
await c.waitForSelector("text=Your role cannot perform this action");
ok(true, "client blocked from clients list");

// public enquiry -> lead appears for sales; document issue -> public verify
await s.request.post("http://localhost:8080/v1/public/enquiries", { data: { name: "Visitor", email: "v@example.com", message: "Need a pentest" } });
await s.goto("http://localhost:3000/leads");
await s.waitForSelector("text=Need a pentest");
ok(true, "website enquiry reached CRM");
await s.goto("http://localhost:3000/documents");
await s.fill("#c", "CRT-A"); await s.fill("#t", "Certificate of Assessment");
await s.click("text=Issue reference");
const ref = await s.locator(".thread li strong").first().innerText();
const pub = await b.newPage();
await pub.goto("http://localhost:3000/verify");
await pub.fill("#ref", ref);
await pub.click("button:has-text('Verify')");
await pub.waitForSelector("text=Valid document");
await pub.fill("#ref", ref.slice(0, -1) + (ref.endsWith("A") ? "B" : "A"));
await pub.click("button:has-text('Verify')");
await pub.waitForSelector("text=Not a valid HEKATE document");
ok(true, "public verify accepts real ref and rejects tampered ref");

// mobile viewport: menu collapses, no horizontal page scroll
const m = await login(staff, 360);
await m.waitForSelector("button.menu");
ok(await m.locator("button.menu").isVisible(), "menu button visible at 360px");
ok(await m.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), "no horizontal page scroll at 360px");
await m.screenshot({ path: "mobile.png" });
await s.goto("http://localhost:3000/tickets"); await s.screenshot({ path: "desktop.png" });
await b.close();
