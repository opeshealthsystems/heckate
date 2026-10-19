// Drives attachments in the browser: a client attaches a photo to a ticket, staff download it,
// a disguised HTML file is refused, and another client cannot reach it.
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
  const ctx = await b.newContext({ bypassCSP: true, viewport: { width: 1280, height: 900 }, acceptDownloads: true });
  const p = await ctx.newPage();
  await p.goto(`${WEB}/login`);
  await p.fill("#tok", tok);
  await p.click("text=Continue");
  await p.waitForURL("**/dashboard");
  return p;
}
const go = async (p, path) => { await p.goto(WEB + path); await p.waitForLoadState("networkidle"); await p.waitForSelector("h1"); };

const sfx = Date.now();
const staffTok = token("att-staff", ["support"]);
const c1 = (await api("POST", "/v1/clients", staffTok, { legal_name: `Att A ${sfx}` })).body.tenant_id;
const c2 = (await api("POST", "/v1/clients", staffTok, { legal_name: `Att B ${sfx}` })).body.tenant_id;
const alice = token("att-alice", ["client_user"], c1);
const bob = token("att-bob", ["client_user"], c2);
const t = (await api("POST", "/v1/tickets", alice, { subject: `Broken rack ${sfx}`, severity: "P3", channel: "app" })).body;

const png = Buffer.concat([Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]), Buffer.alloc(300, 9)]);

const a = await session(alice);
await go(a, `/tickets/${t.id}`);
await a.setInputFiles(`#att-${t.id}`, { name: "rack photo.png", mimeType: "image/png", buffer: png });
await a.waitForSelector("text=rack photo.png");
ok(true, "client attached a photo to their ticket");

// A file that only pretends to be an image is refused with a clear message.
await a.setInputFiles(`#att-${t.id}`, { name: "innocent.png", mimeType: "image/png", buffer: Buffer.from("<html><script>alert(1)</script></html>") });
await a.waitForSelector("text=File type not allowed");
ok((await a.locator("text=innocent.png").count()) === 0, "HTML disguised as a PNG was refused and not stored");

// Staff can download exactly those bytes.
const s = await session(staffTok);
await go(s, `/tickets/${t.id}`);
const [dl] = await Promise.all([s.waitForEvent("download"), s.click("text=rack photo.png")]);
const stream = await dl.createReadStream();
const chunks = [];
for await (const c of stream) chunks.push(c);
ok(dl.suggestedFilename() === "rack photo.png" && Buffer.concat(chunks).equals(png), "staff downloaded the identical file");

// Another client cannot open the ticket, so they cannot see its files either.
const o = await session(bob);
await o.goto(`${WEB}/tickets/${t.id}`);
await o.waitForSelector("text=not found");
ok((await o.locator("text=rack photo.png").count()) === 0, "another client sees neither the ticket nor its attachment");
const direct = await api("GET", `/v1/attachments?entity_type=ticket&entity_id=${t.id}`, bob);
ok(direct.status === 404, "the API also refuses another client's attachment list");
await b.close();
