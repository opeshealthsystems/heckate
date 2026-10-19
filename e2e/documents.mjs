// PDF download and verification in the browser, plus company API keys.
// A client downloads an issued invoice as a PDF, checks that exact file on /verify (it matches),
// and a tampered copy is flagged. A client admin then creates an API key in the Developer screen
// and uses it against the API, where it can read but not write.
import { chromium } from "playwright-core";
import crypto from "node:crypto";
import fs from "node:fs";

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
  const ctx = await b.newContext({ viewport: { width: 1280, height: 900 }, acceptDownloads: true });
  const p = await ctx.newPage();
  await p.goto(`${WEB}/login`);
  await p.fill("#tok", tok);
  await p.click("text=Continue");
  await p.waitForURL("**/dashboard");
  return p;
}
const go = async (p, path) => { await p.goto(WEB + path); await p.waitForLoadState("networkidle"); await p.waitForSelector("h1"); };

const sfx = Date.now();
const fin = token("doc-fin", ["finance", "sales"]);
const tid = (await api("POST", "/v1/clients", fin, { legal_name: `Doc Client ${sfx}` })).body.tenant_id;
const admin = token("doc-admin", ["client_admin"], tid);
const inv = (await api("POST", "/v1/invoices", fin, { tenant_id: tid, title: `Audit ${sfx}`, lines: [{ description: "Audit", quantity: 1, unit_price: 100000 }] })).body;
const issued = (await api("POST", `/v1/invoices/${inv.id}/issue`, fin)).body;
ok(!!issued.ref, "invoice issued");
await api("POST", "/v1/tickets", admin, { subject: `Key test ${sfx}`, description: "d", severity: "P3", channel: "app" });

// 1. The client downloads the PDF from the invoice page.
const c = await session(admin);
await go(c, "/invoices");
await c.click(`text=Audit ${sfx}`);
const [dl] = await Promise.all([c.waitForEvent("download"), c.click("button:has-text('Download PDF')")]);
const path = await dl.path();
const pdf = fs.readFileSync(path);
ok(pdf.subarray(0, 4).toString() === "%PDF", "downloaded file is a PDF");

// 2. The public verify page confirms that exact file, and flags an altered copy.
const pub = await b.newPage();
await pub.goto(`${WEB}/verify?ref=${encodeURIComponent(issued.ref)}`);
await pub.waitForSelector("text=Valid document");
await pub.setInputFiles("#file", path);
await pub.waitForSelector("text=identical to the original");
ok(true, "verify page confirms the downloaded PDF is the original");
const tampered = Buffer.concat([pdf, Buffer.from("\n% edited\n")]);
await pub.setInputFiles("#file", { name: "edited.pdf", mimeType: "application/pdf", buffer: tampered });
await pub.waitForSelector("text=does NOT match");
ok(true, "verify page flags an altered copy");

// 3. The client admin creates an API key in the Developer screen.
await go(c, "/developer");
await c.fill("#key-name", "e2e sync");
await c.click("button:has-text('Create key')");
await c.waitForSelector("[data-testid=new-key]");
const key = (await c.locator("[data-testid=new-key]").innerText()).trim();
ok(key.startsWith("hk_"), "key shown once after creation");
await c.click("text=I have saved it");
ok((await c.locator("[data-testid=new-key]").count()) === 0, "secret is gone after dismissing");
const read = await fetch(`${API}/v1/tickets`, { headers: { Authorization: `Bearer ${key}` } });
const list = await read.json();
ok(read.status === 200 && list.data.length >= 1, "key reads the company's tickets");
const write = await fetch(`${API}/v1/tickets`, { method: "POST", headers: { Authorization: `Bearer ${key}`, "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: "{}" });
ok(write.status === 403, "key cannot write");
await go(c, "/developer");
await c.waitForSelector("text=e2e sync");
c.once("dialog", (d) => d.accept());
await c.click("button:has-text('Revoke')");
await c.waitForSelector("text=revoked");
const after = await fetch(`${API}/v1/tickets`, { headers: { Authorization: `Bearer ${key}` } });
ok(after.status === 401, "revoked key stops working");

await b.close();
