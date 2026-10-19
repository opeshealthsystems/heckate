// Document preview: the library lists the approved forms, a blank form can be previewed before it is filled in,
// a draft can be previewed as it will print (marked as a draft), and an issued invoice can be previewed and printed.
// Runs under the real Content-Security-Policy, so it also proves the in-page PDF viewer is allowed.
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
  const r = await fetch(API + path, { method, headers: { Authorization: `Bearer ${tok}`, "Content-Type": "application/json", "Idempotency-Key": `pv-${Date.now()}-${n++}` }, body: body ? JSON.stringify(body) : undefined });
  return r.json().catch(() => ({}));
};
const mgr = token("pv-mgr", ["manager", "finance", "sales", "hr"]);
const sfx = String(Date.now()).slice(-6);
const client = await call("POST", "/v1/clients", mgr, { legal_name: `Preview Co ${sfx}` });
const drafted = await call("POST", "/v1/invoices", mgr, { tenant_id: client.tenant_id, title: `Preview invoice ${sfx}`, lines: [{ description: "Work", quantity: 1, unit_price: 100000 }] });
await call("POST", `/v1/invoices/${drafted.id}/issue`, mgr);

const b = await chromium.launch({ executablePath: process.env.CHROMIUM ?? "/opt/pw-browsers/chromium", args: ["--no-sandbox"] });
const p = await (await b.newContext({ viewport: { width: 1280, height: 900 } })).newPage(); // no bypassCSP
const blocked = [];
p.on("console", (m) => { if (/content security policy|refused to/i.test(m.text())) blocked.push(m.text()); });
await p.goto(`${WEB}/login`); await p.fill("#tok", mgr); await p.click("text=Continue"); await p.waitForURL("**/dashboard");

// Library
await p.goto(`${WEB}/documents`);
await p.waitForSelector("table tbody tr");
const rows = await p.locator("table tbody tr").count();
ok(rows === 33, `the library lists all 33 approved forms and templates (${rows})`);
await p.fill("#doc-q", "fax");
ok((await p.locator("table tbody tr").count()) === 1, "searching by another name finds the form (fax cover)");
await p.fill("#doc-q", "PV");
ok((await p.locator("table tbody tr", { hasText: "HS-FRM-27" }).count()) === 1, "the acceptance form is found by its short name PV");
await p.fill("#doc-q", "");

// Blank form preview
await p.locator("table tbody tr", { hasText: "HS-FRM-23" }).getByRole("button", { name: /Preview/ }).click();
await p.waitForSelector(".pv-page[data-ready]", { timeout: 20000 });
ok(await p.locator('[role=dialog] .pv-title').innerText().then((t) => /HS-FRM-23/.test(t) && /SHA-256/.test(t)), "the blank form preview names the form, its version and its check value");
ok((await p.locator(".pv-page").count()) >= 1, "the blank form is drawn page by page in the page");
const px = await p.evaluate(() => { const c = document.querySelector(".pv-page"); const d = c.getContext("2d").getImageData(0, 0, c.width, c.height).data; let ink = 0; for (let i = 0; i < d.length; i += 40) if (d[i] < 200) ink++; return ink; });
ok(px > 200, `the page is not blank (${px} dark samples)`);
await p.keyboard.press("Escape");
ok((await p.locator("[role=dialog]").count()) === 0, "Escape closes the preview");

// Fill in, preview the draft
await p.locator("table tbody tr", { hasText: "HS-FRM-23" }).getByRole("link", { name: "Fill in" }).click();
await p.waitForURL("**/documents/new?code=HS-FRM-23");
await p.selectOption("#d-client", { label: `Preview Co ${sfx}` });
await p.fill("#d-title", `Change request ${sfx}`);
await p.fill('textarea[aria-label="Text 1"]', "Add two access points to the second floor.");
await p.getByRole("button", { name: /Preview as it will print/ }).click();
await p.waitForSelector(".pv-page[data-ready]", { timeout: 20000 });
ok(/Draft/.test(await p.locator(".pv-note, .pv-title").first().innerText()) && /not valid/.test(await p.locator(".pv-title").innerText()), "the draft preview says it is a draft and not valid");
await p.keyboard.press("Escape");
await p.getByRole("button", { name: /^Save (draft|changes)$/ }).click();
await p.waitForURL("**/documents");
await p.getByRole("tab", { name: "My documents" }).click();
ok((await p.locator("table tbody tr", { hasText: `Change request ${sfx}` }).count()) === 1, "the saved draft is listed under My documents");
await p.locator("table tbody tr", { hasText: `Change request ${sfx}` }).getByRole("button", { name: /Preview/ }).click();
await p.waitForSelector(".pv-page[data-ready]", { timeout: 20000 });
await p.keyboard.press("Escape");
await p.locator("table tbody tr", { hasText: `Change request ${sfx}` }).getByRole("button", { name: "Issue" }).click();
const gotRef = await p.locator("table tbody tr", { hasText: `Change request ${sfx}` }).filter({ hasText: "HS-FRM-23-" }).waitFor({ timeout: 10000 }).then(() => true, () => false);
ok(gotRef, "issuing gives the document its reference");
await p.locator("table tbody tr", { hasText: `Change request ${sfx}` }).getByRole("button", { name: /Preview/ }).click();
await p.waitForSelector(".pv-page[data-ready]", { timeout: 20000 });
ok(!/not valid/.test(await p.locator(".pv-title").innerText()), "an issued document previews without the draft marking");
await p.keyboard.press("Escape");

// Version history: every save is kept; a change after issue is a revision that replaces the issued document.
const row = () => p.locator("table tbody tr", { hasText: `Change request ${sfx}` });
await row().first().getByRole("button", { name: /History/ }).click();
await p.waitForSelector(".history li");
ok((await p.locator(".history li").count()) === 2, "the history lists the creation and the issue as two versions");
ok(/Issued as HS-FRM-23-/.test(await p.locator(".history li").first().innerText()), "the newest history entry records the issue");
await p.getByRole("button", { name: "Close" }).first().click();
await row().first().getByRole("button", { name: "Revise" }).click();
await p.locator("table tbody tr", { hasText: `Change request ${sfx}` }).filter({ hasText: "draft" }).first().waitFor({ timeout: 10000 });
await p.locator("table tbody tr", { hasText: `Change request ${sfx}` }).filter({ hasText: "draft" }).getByRole("link", { name: "Edit" }).click();
await p.waitForSelector("#d-note");
await p.fill('textarea[aria-label="Text 1"]', "Add three access points to the second floor.");
await p.fill("#d-note", "Client wants three, not two");
await p.getByRole("button", { name: "Save changes" }).click();
await p.waitForURL("**/documents");
await p.getByRole("tab", { name: "My documents" }).click();
await p.locator("table tbody tr", { hasText: `Change request ${sfx}` }).filter({ hasText: "draft" }).getByRole("button", { name: /History/ }).click();
await p.waitForSelector(".history li");
const hist = await p.locator(".history").innerText();
ok(/Client wants three, not two/.test(hist) && /Revision of HS-FRM-23-/.test(hist), "the revision's history keeps the reason for the change and names the document it replaces");
await p.locator(".history details summary").first().click();
ok((await p.locator(".diff .d-add").count()) >= 1 && (await p.locator(".diff .d-del").count()) >= 1, "the change is shown line by line, additions and removals marked");
await p.getByRole("button", { name: "Close" }).first().click();
await p.locator("table tbody tr", { hasText: `Change request ${sfx}` }).filter({ hasText: "draft" }).getByRole("button", { name: "Issue" }).click();
await p.locator("table tbody tr", { hasText: "Replaced by HS-FRM-23-" }).first().waitFor({ timeout: 10000 });
ok(true, "once the revision is issued the earlier document is shown as replaced");
const oldRef = (await p.locator("table tbody tr", { hasText: "Replaced by" }).first().locator("td").first().innerText()).trim();
await p.goto(`${WEB}/verify?ref=${oldRef}`);
await p.waitForSelector("text=Valid document");
ok(await p.getByText("replaced by a newer version").isVisible(), "verifying the old reference says it is genuine but replaced");

// Invoice preview and print
await p.goto(`${WEB}/invoices/${drafted.id}`);
await p.getByRole("button", { name: /Preview and print/ }).click();
await p.waitForSelector(".pv-page[data-ready]", { timeout: 20000 });
ok(await p.getByRole("button", { name: /Print/ }).isEnabled(), "an issued invoice previews and can be printed");
const printed = await p.evaluate(() => new Promise((res) => { window.print = () => res(document.body.classList.contains("printing-preview")); document.querySelectorAll("button").forEach((x) => /Print$/.test(x.textContent.trim()) && x.click()); }));
ok(printed === true, "Print hides the application and prints only the document pages");
await p.keyboard.press("Escape");

ok(blocked.length === 0, `no Content-Security-Policy violations${blocked.length ? ": " + blocked[0] : ""}`);
await b.close();
