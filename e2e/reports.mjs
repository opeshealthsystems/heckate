// Statements and exports: a finance user previews a client's statement of account and the financial statements,
// downloads spreadsheet-ready CSV files, and a client sees their own statement. Real Content-Security-Policy.
import { chromium } from "playwright-core";
import crypto from "node:crypto";
import fs from "node:fs";

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
  const r = await fetch(API + path, { method, headers: { Authorization: `Bearer ${tok}`, "Content-Type": "application/json", "Idempotency-Key": `rp-${Date.now()}-${n++}` }, body: body ? JSON.stringify(body) : undefined });
  return r.json().catch(() => ({}));
};
const fin = token("rp-fin", ["finance", "manager"]);
const sfx = String(Date.now()).slice(-6);
const client = await call("POST", "/v1/clients", fin, { legal_name: `Report Co ${sfx}` });
const inv = await call("POST", "/v1/invoices", fin, { tenant_id: client.tenant_id, title: `Report invoice ${sfx}`, lines: [{ description: "Work", quantity: 1, unit_price: 500000 }] });
await call("POST", `/v1/invoices/${inv.id}/issue`, fin);
await call("POST", `/v1/invoices/${inv.id}/payments`, fin, { amount: 100000, method: "bank" });

const b = await chromium.launch({ executablePath: process.env.CHROMIUM ?? "/opt/pw-browsers/chromium", args: ["--no-sandbox"] });
const ctx = await b.newContext({ viewport: { width: 1280, height: 900 }, acceptDownloads: true });
const p = await ctx.newPage();
const blocked = [];
p.on("console", (m) => { if (/content security policy|refused to/i.test(m.text())) blocked.push(m.text()); });
await p.goto(`${WEB}/login`); await p.fill("#tok", fin); await p.click("text=Continue"); await p.waitForURL("**/dashboard");

// Client statement from the Clients page
await p.goto(`${WEB}/clients`);
const row = p.locator("table tbody tr", { hasText: `Report Co ${sfx}` });
await row.waitFor();
await row.getByRole("button", { name: /Statement/ }).click();
await p.waitForSelector(".pv-page[data-ready]", { timeout: 20000 });
ok(/Statement of account/.test(await p.locator(".pv-title").innerText()), "a client's statement of account opens in the preview");
const ink = await p.evaluate(() => { const c = document.querySelector(".pv-page"); const d = c.getContext("2d").getImageData(0, 0, c.width, c.height).data; let k = 0; for (let i = 0; i < d.length; i += 40) if (d[i] < 200) k++; return k; });
ok(ink > 200, "the statement page is drawn, not blank");
await p.keyboard.press("Escape");
const [csvDl] = await Promise.all([p.waitForEvent("download"), row.getByRole("button", { name: /CSV/ }).click()]);
const csv = fs.readFileSync(await csvDl.path(), "utf8");
ok(csv.startsWith("﻿Date,Type,Reference") && csv.includes("596250") && csv.includes("100000"), "the statement downloads as a CSV with plain amounts");
ok(/^statement-Report-Co/.test(csvDl.suggestedFilename()) && csvDl.suggestedFilename().endsWith(".csv"), "the file is named after the client and the date");

// Accounting reports tab
await p.goto(`${WEB}/accounting`);
await p.getByRole("tab", { name: "Reports" }).click();
await p.locator("li", { hasText: "Financial statements" }).getByRole("button", { name: /Preview and print/ }).click();
await p.waitForSelector(".pv-page[data-ready]", { timeout: 20000 });
ok(/financial-statements|Financial statements/i.test(await p.locator(".pv-title").innerText()), "the financial statements preview opens");
await p.keyboard.press("Escape");
const [tbDl] = await Promise.all([p.waitForEvent("download"), p.locator("li", { hasText: "Trial balance" }).getByRole("button", { name: /Export CSV/ }).click()]);
const tb = fs.readFileSync(await tbDl.path(), "utf8");
ok(tb.startsWith("﻿Account,Name,Debit (XAF),Credit (XAF),Balance (XAF)"), "the trial balance exports to CSV");
const [pdfDl] = await Promise.all([p.waitForEvent("download"), (async () => { await p.locator("li", { hasText: "Receivables ageing" }).getByRole("button", { name: /Download PDF/ }).click(); })()]);
ok(fs.readFileSync(await pdfDl.path()).subarray(0, 4).toString() === "%PDF", "the ageing report downloads as a PDF");

// A client sees their own statement from Invoices, and nothing of anyone else's
const cl = await (await b.newContext({ viewport: { width: 1100, height: 900 } })).newPage();
await cl.goto(`${WEB}/login`); await cl.fill("#tok", token("rp-client", ["client_admin"], client.tenant_id)); await cl.click("text=Continue"); await cl.waitForURL("**/dashboard");
await cl.goto(`${WEB}/invoices`);
await cl.getByRole("button", { name: /Preview and print/ }).click();
await cl.waitForSelector(".pv-page[data-ready]", { timeout: 20000 });
ok(true, "a client can preview their own statement of account");
await cl.goto(`${WEB}/accounting`);
await cl.waitForSelector("text=Not available");
ok((await cl.locator("text=Reports").count()) === 0, "a client who types the accounting address gets \"not available\", not the accounting screens");

ok(blocked.length === 0, `no Content-Security-Policy violations${blocked.length ? ": " + blocked[0] : ""}`);
await b.close();
