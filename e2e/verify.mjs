// The public verification page: a genuine document shows as authentic with its particulars; a replaced one says so;
// an unknown reference and a forged code are refused; someone who verified a document can ask to see it, a manager
// approves, and the granted link downloads the original. Runs under the real Content-Security-Policy.
import { chromium } from "playwright-core";
import crypto from "node:crypto";

const WEB = process.env.WEB ?? "http://localhost:3000";
const API = process.env.API ?? "http://localhost:8080";
const SECRET = process.env.HEKATE_DEV_JWT_SECRET ?? "e2e-secret";
const b64 = (o) => Buffer.from(JSON.stringify(o)).toString("base64url");
const token = (sub, roles, tenant = "") => {
  const h = b64({ alg: "HS256", typ: "JWT" }), bd = b64({ sub, roles, tenant_id: tenant, exp: Math.floor(Date.now() / 1000) + 3600 });
  return `${h}.${bd}.${crypto.createHmac("sha256", SECRET).update(`${h}.${bd}`).digest("base64url")}`;
};
const ok = (c, m) => { console.log(c ? "PASS" : "FAIL", m); if (!c) process.exitCode = 1; };
let n = 0;
const call = async (method, path, tok, body) => {
  const r = await fetch(API + path, { method, headers: { Authorization: `Bearer ${tok}`, "Content-Type": "application/json", "Idempotency-Key": `vf-${Date.now()}-${n++}` }, body: body ? JSON.stringify(body) : undefined });
  return r.json().catch(() => ({}));
};
const sales = token("vf-sales", ["sales"]), mgr = token("vf-mgr", ["manager", "sales"]);
const sfx = String(Date.now()).slice(-6);
const client = await call("POST", "/v1/clients", mgr, { legal_name: `Verify Co ${sfx}` });
const nar = await call("POST", "/v1/narratives", sales, { kind: "proposal", tenant_id: client.tenant_id, title: `Verification proposal ${sfx}`, sections: [{ heading: "Scope", body: "Secret scope " + sfx }] });
const issued = await call("POST", `/v1/narratives/${nar.id}/issue`, sales);
const ref = issued.ref;
await fetch(`${API}/v1/narratives/${nar.id}/pdf`, { headers: { Authorization: `Bearer ${sales}` } }); // produce the stored original

const b = await chromium.launch({ executablePath: process.env.CHROMIUM ?? "/opt/pw-browsers/chromium", args: ["--no-sandbox"] });
const ctx = await b.newContext({ viewport: { width: 1100, height: 900 }, acceptDownloads: true });
const p = await ctx.newPage(); // no bypassCSP
const blocked = [];
p.on("console", (m) => { if (/content security policy|refused to/i.test(m.text())) blocked.push(m.text()); });

// Genuine document
await p.goto(`${WEB}/verify?ref=${ref}`);
await p.waitForSelector(".vr");
const text = await p.locator(".vr").innerText();
ok(/Authentic HEKATE document/.test(text), "a genuine document shows as authentic");
ok(/Commercial and Technical Proposal|Proposal/.test(text) && text.includes(ref) && /1\.0/.test(text) && /Confidential/.test(text) && /HEKATE SYSTEMS SARL/.test(text), "it shows the document type, reference, version, classification and issuer");
ok(/Verified once/.test(text), "the first check is counted");
ok(!text.includes("Secret scope") && !text.includes(`Verify Co ${sfx}`), "verification never reveals the content or the client");
ok(await p.locator('img[src*="HEKATE_Seal_Navy"]').first().isVisible(), "the HEKATE seal is shown on the result");
await p.reload();
await p.waitForSelector(".vr");
ok(/Verified 2 times/.test(await p.locator(".vr").innerText()), "later checks are counted");

// Unknown reference and a forged signed code
await p.goto(`${WEB}/verify?ref=HS-PR-2026-999999`);
await p.waitForSelector(".vr");
ok(/Not a valid HEKATE document/.test(await p.locator(".vr").innerText()), "an unknown reference is refused");
await p.goto(`${WEB}/v/${ref}.1.0.20260101.C.${"A".repeat(86)}`);
await p.waitForSelector(".vr");
ok(/Not a valid HEKATE document/.test(await p.locator(".vr").innerText()), "a forged signed code is refused");

// Request access, with a reason
await p.goto(`${WEB}/verify?ref=${ref}`);
await p.waitForSelector(".vr");
await p.getByRole("button", { name: "Request access" }).click();
await p.fill("#ra-name", "Auditor One");
await p.fill("#ra-email", "auditor@bank.example");
await p.fill("#ra-org", "A Bank");
await p.fill("#ra-reason", "Due-diligence file for the 2026 supplier review needs a copy.");
await p.getByRole("button", { name: "Send request" }).click();
await p.waitForSelector("text=Request received");
ok(true, "a verifier can ask to see the document and say why");

// A manager approves; the link downloads the original
const m = await (await b.newContext({ viewport: { width: 1280, height: 900 }, acceptDownloads: true })).newPage();
await m.goto(`${WEB}/login`); await m.fill("#tok", mgr); await m.click("text=Continue"); await m.waitForURL("**/dashboard");
await m.goto(`${WEB}/documents`);
await m.getByRole("tab", { name: "Access requests" }).click();
const card = m.locator(".requests li", { hasText: "Auditor One" }).first();
await card.waitFor({ timeout: 10000 });
ok((await card.innerText()).includes("Due-diligence file"), "the manager sees who asked, for which document, and why");
await card.getByRole("button", { name: /Approve and create link/ }).click();
await m.waitForSelector(".notice.ok code");
const link = await m.locator(".notice.ok code").innerText();
ok(/\/access\/[A-Za-z0-9_-]{40,}/.test(link), "approving creates a long one-time-shown link");
const dl = await ctx.newPage();
await dl.goto(link.replace(/^https?:\/\/[^/]+/, WEB));
const [download] = await Promise.all([dl.waitForEvent("download"), dl.getByRole("button", { name: /Download the PDF/ }).click()]);
const path = await download.path();
const head = (await import("node:fs")).readFileSync(path).subarray(0, 4).toString();
ok(head === "%PDF" && download.suggestedFilename() === `${ref}.pdf`, "the granted link downloads the original PDF");

// Staff see how often it was checked
await m.goto(`${WEB}/documents`);
await m.getByRole("tab", { name: "My documents" }).click();
await m.locator("table tbody tr", { hasText: ref }).getByRole("button", { name: /History/ }).click();
await m.waitForSelector("text=Verified");
ok(/Verified \d+ times? by the public/.test(await m.locator(".notice").first().innerText()), "the document's history shows how often the public verified it");

// A revision replaces it: the old reference stays genuine but says it was replaced
const rev = await call("POST", `/v1/narratives/${nar.id}/revise`, sales, {});
await call("PUT", `/v1/narratives/${rev.id}`, sales, { title: rev.title, sections: [{ heading: "Scope", body: "Changed scope " + sfx }], note: "Scope changed" });
const revIssued = await call("POST", `/v1/narratives/${rev.id}/issue`, sales);
await p.goto(`${WEB}/verify?ref=${ref}`);
await p.waitForSelector(".vr");
const rt = await p.locator(".vr").innerText();
ok(/Genuine, but replaced/.test(rt) && rt.includes(revIssued.ref), "a replaced document says so and names the newer version");
await p.getByRole("link", { name: /Verify the newer version/ }).click();
await p.waitForSelector("text=Authentic HEKATE document");
ok(/2\.0/.test(await p.locator(".vr").innerText()), "the newer version is authentic and is version 2.0");

ok(blocked.length === 0, `no Content-Security-Policy violations${blocked.length ? ": " + blocked[0] : ""}`);
await b.close();
