// Visits the main screens with the real Content-Security-Policy in force (no bypass) and fails if the
// browser blocks anything the app needs, or if an inline-script injection would run.
import { chromium } from "playwright-core";
import crypto from "node:crypto";

const WEB = process.env.WEB ?? "http://localhost:3000";
const API = process.env.API ?? "http://localhost:8080";
const SECRET = process.env.HEKATE_DEV_JWT_SECRET ?? "e2e-secret";
const b64 = (o) => Buffer.from(JSON.stringify(o)).toString("base64url");
function token(sub, roles, tenant = "") {
  const head = b64({ alg: "HS256", typ: "JWT" });
  const body = b64({ sub, roles, tenant_id: tenant, exp: Math.floor(Date.now() / 1000) + 3600 });
  return `${head}.${body}.${crypto.createHmac("sha256", SECRET).update(`${head}.${body}`).digest("base64url")}`;
}
const ok = (c, m) => { console.log(c ? "PASS" : "FAIL", m); if (!c) process.exitCode = 1; };

const b = await chromium.launch({ executablePath: process.env.CHROMIUM ?? "/opt/pw-browsers/chromium", args: ["--no-sandbox"] });
const ctx = await b.newContext({ viewport: { width: 1280, height: 900 } }); // no bypassCSP
const p = await ctx.newPage();
const violations = [];
p.on("console", (m) => { if (/content security policy|refused to (load|connect|execute)/i.test(m.text())) violations.push(m.text()); });
p.on("pageerror", (e) => { if (/content security policy/i.test(e.message)) violations.push(e.message); });

const mgr = token("csp-mgr", ["manager", "finance", "hr", "sales", "support"]);
await p.goto(`${WEB}/login`);
await p.fill("#tok", mgr);
await p.click("text=Continue");
await p.waitForURL("**/dashboard");
for (const path of ["/dashboard", "/tickets", "/clients", "/quotes", "/invoices", "/accounting", "/payroll", "/hr", "/inventory", "/procurement", "/documents", "/developer"]) {
  await p.goto(WEB + path);
  await p.waitForLoadState("networkidle");
  await p.waitForSelector("h1");
}
ok(violations.length === 0, `no CSP violations across 12 screens${violations.length ? ": " + violations[0] : ""}`);
const hdr = (await p.goto(`${WEB}/login`)).headers()["content-security-policy"] ?? "";
ok(/script-src [^;]*/.test(hdr) && !/unsafe-eval/.test(hdr) && /frame-ancestors 'none'/.test(hdr) && /object-src 'none'/.test(hdr), "the page is served with a CSP that forbids eval, plugins and framing");

// The API lets the app's origin in and nobody else.
const evil = await fetch(`${API}/v1/me`, { headers: { Origin: "https://evil.example", Authorization: `Bearer ${mgr}` } });
ok(!evil.headers.get("access-control-allow-origin"), "a foreign origin gets no CORS permission");
const good = await fetch(`${API}/v1/me`, { headers: { Origin: WEB, Authorization: `Bearer ${mgr}` } });
ok(good.headers.get("access-control-allow-origin") === WEB, "the app's own origin is allowed");
await b.close();
