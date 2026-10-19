// Paying an invoice by mobile money in the browser, against the API's sandbox provider
// (start the API with HEKATE_PAYMENTS_SANDBOX=1 HEKATE_PAYMENTS_WEBHOOK_SECRET=e2e-pay).
// The client asks for a payment; nothing is booked until a correctly signed callback arrives.
import { chromium } from "playwright-core";
import crypto from "node:crypto";

const API = process.env.API ?? "http://localhost:8080";
const WEB = process.env.WEB ?? "http://localhost:3000";
const SECRET = process.env.HEKATE_DEV_JWT_SECRET ?? "e2e-secret";
const PAY = process.env.HEKATE_PAYMENTS_WEBHOOK_SECRET ?? "e2e-pay";
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
async function callback(event, key = PAY) {
  const body = JSON.stringify(event);
  const sig = "sha256=" + crypto.createHmac("sha256", key).update(body).digest("hex");
  return (await fetch(`${API}/v1/webhooks/payments/momo`, { method: "POST", headers: { "Content-Type": "application/json", "X-Hekate-Signature": sig }, body })).status;
}
const ok = (c, m) => { console.log(c ? "PASS" : "FAIL", m); if (!c) process.exitCode = 1; };

const n = Date.now();
const fin = token("e2e-pay-fin", ["finance", "sales"]);
const tid = (await api("POST", "/v1/clients", fin, { legal_name: `Pay Co ${n}` })).body.tenant_id;
const admin = token("e2e-pay-admin", ["client_admin"], tid);
const inv = (await api("POST", "/v1/invoices", fin, { tenant_id: tid, title: `Support plan ${n}`, lines: [{ description: "Support", quantity: 1, unit_price: 100000 }] })).body;
await api("POST", `/v1/invoices/${inv.id}/issue`, fin);

const b = await chromium.launch({ executablePath: process.env.CHROMIUM ?? "/opt/pw-browsers/chromium", args: ["--no-sandbox"] });
const p = await (await b.newContext({ viewport: { width: 1280, height: 900 } })).newPage();
await p.goto(`${WEB}/login`);
await p.fill("#tok", admin);
await p.click("text=Continue");
await p.waitForURL("**/dashboard");
await p.goto(`${WEB}/invoices/${inv.id}`);
await p.waitForSelector("section[aria-label='Pay online']");
await p.fill("#po-num", "237670000000");
await p.click("button:has-text('Pay')");
await p.waitForSelector("text=Approve it there");
ok(true, "request sent; the page asks the client to approve on the phone");

const intent = (await api("GET", `/v1/invoices/${inv.id}/online-payments`, admin)).body.data[0];
ok(await callback({ event_id: `e-${n}-a`, reference: intent.id, status: "succeeded", amount: 119250 }, "not-the-secret") === 401, "a callback with the wrong signature is refused");
ok((await api("GET", `/v1/invoices/${inv.id}`, admin)).body.status === "issued", "the invoice is still unpaid");
ok(await callback({ event_id: `e-${n}-b`, reference: intent.id, status: "succeeded", amount: 119250 }) === 200, "the signed callback is accepted");
await p.waitForFunction(() => document.body.innerText.replace(/\s/g, "").includes("balance0XAF"), null, { timeout: 20000 });
ok(true, "the page notices the payment without a reload");
ok((await api("GET", `/v1/invoices/${inv.id}`, admin)).body.status === "paid", "the invoice is now paid");
await b.close();
