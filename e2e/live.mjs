// A client has a ticket open; staff reply through the API; the reply must appear without a reload,
// while a client of another tenant sees nothing.
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
  const ctx = await b.newContext({ viewport: { width: 1280, height: 900 } });
  const p = await ctx.newPage();
  await p.goto(`${WEB}/login`);
  await p.fill("#tok", tok);
  await p.click("text=Continue");
  await p.waitForURL("**/dashboard");
  return p;
}
const sfx = Date.now();
const staff = token("live-staff", ["support"]);
const c1 = (await api("POST", "/v1/clients", staff, { legal_name: `Live A ${sfx}` })).body.tenant_id;
const alice = token("live-alice", ["client_user"], c1);
const t = (await api("POST", "/v1/tickets", alice, { subject: `Live ticket ${sfx}`, severity: "P3", channel: "app" })).body;

const a = await session(alice);
await a.goto(`${WEB}/tickets/${t.id}`);
await a.waitForLoadState("networkidle");
await a.waitForSelector("text=No replies yet");

// Count page loads: the update must arrive over the socket, not through a navigation.
let navigations = 0;
a.on("framenavigated", () => navigations++);

await api("POST", `/v1/tickets/${t.id}/comments`, staff, { body: "We are looking at it now" });
await a.waitForSelector("text=We are looking at it now", { timeout: 15000 });
ok(navigations === 0, "the staff reply appeared on the client's open page without a reload");

await api("PATCH", `/v1/tickets/${t.id}`, staff, { status: "in_progress" });
await a.waitForSelector("text=in progress", { timeout: 15000 });
ok(true, "the status change also appeared live");

// A client of another tenant with their own ticket open hears nothing about this one.
const c2 = (await api("POST", "/v1/clients", staff, { legal_name: `Live B ${sfx}` })).body.tenant_id;
const bob = token("live-bob", ["client_user"], c2);
const bobTicket = (await api("POST", "/v1/tickets", bob, { subject: `Bob ticket ${sfx}`, severity: "P4", channel: "app" })).body;
const o = await session(bob);
await o.goto(`${WEB}/tickets/${bobTicket.id}`);
await o.waitForLoadState("networkidle");
await api("POST", `/v1/tickets/${t.id}/comments`, staff, { body: "secret for alice only" });
await o.waitForTimeout(3000);
ok((await o.locator("text=secret for alice only").count()) === 0, "another tenant's page never shows it");
await b.close();
