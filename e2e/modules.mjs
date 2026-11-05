// Contracts, client online forms, project progress, QR labels, employee files and the bank-account register,
// driven through the real web app.
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
  const r = await fetch(API + path, { method, headers: { Authorization: `Bearer ${tok}`, "Content-Type": "application/json", "Idempotency-Key": `mo-${Date.now()}-${n++}` }, body: body ? JSON.stringify(body) : undefined });
  return r.json().catch(() => ({}));
};
const day = (o) => new Date(Date.now() + o * 86400000).toISOString().slice(0, 10);
const sfx = String(Date.now()).slice(-6);
const mgr = token("mo-mgr", ["manager", "sales", "support"]);
const hr = token("mo-hr", ["hr"]);
const fin = token("mo-fin", ["finance", "manager"]);
const log = token("mo-log", ["logistics"]);
const client = await call("POST", "/v1/clients", mgr, { legal_name: `Modules Co ${sfx}` });
const clientTok = token("mo-client", ["client_admin"], client.tenant_id);

const b = await chromium.launch({ executablePath: process.env.CHROMIUM ?? "/opt/pw-browsers/chromium", args: ["--no-sandbox"] });
const page = async (tok, w = 1280) => {
  const p = await (await b.newContext({ viewport: { width: w, height: 900 } })).newPage();
  const errors = [];
  p.on("pageerror", (e) => errors.push(e.message));
  await p.goto(`${WEB}/login`); await p.fill("#tok", tok); await p.click("text=Continue"); await p.waitForURL("**/dashboard");
  p.errors = errors;
  return p;
};

// ---- Contracts ----
const m = await page(mgr);
await m.goto(`${WEB}/contracts`);
await m.selectOption("#ct-client", { label: `Modules Co ${sfx}` });
await m.fill("#ct-title", `Five-year maintenance ${sfx}`);
await m.fill("#ct-start", day(0)); await m.fill("#ct-end", day(5 * 365));
await m.fill("#ct-value", "60000000"); await m.fill("#ct-per", "1000000");
await m.check("text=Renews automatically");
await m.click("text=Create draft");
await m.waitForURL("**/contracts/*");
ok(/CT-\d{4}-\d{4}/.test(await m.locator("h1").innerText()), "a draft contract is created and numbered");
await m.getByRole("button", { name: "Activate" }).click();
await m.locator(".error").first().waitFor();
ok(await m.locator(".error").count() > 0, "activating without clauses is refused with a reason");
await m.getByRole("button", { name: "Add clause" }).click();
await m.fill('[aria-label="Clause 1 title"]', "Scope");
await m.fill('[aria-label="Clause 1 text"]', "Preventive and corrective maintenance of the server room.");
await m.getByRole("button", { name: "Save clauses" }).click();
await m.waitForTimeout(700);
await m.fill("#sla-r-P1", "10"); await m.fill("#sla-v-P1", "60");
await m.getByRole("button", { name: "Save times" }).click();
await m.getByRole("button", { name: "Activate" }).click();
await m.getByRole("button", { name: /Renew by/ }).waitFor();
ok((await m.locator(".meta").innerText()).includes("active"), "the contract is activated once it has clauses");
ok((await m.inputValue("#sla-r-P1")) === "10", "the promised response time is kept on the contract");
await m.getByRole("button", { name: /Preview and print/ }).click();
await m.waitForSelector(".pv-page[data-ready]", { timeout: 20000 });
ok(/CT-/.test(await m.locator(".pv-title").innerText()), "the contract schedule previews before printing");
await m.keyboard.press("Escape");
await m.goto(`${WEB}/contracts`);
await m.getByRole("link", { name: /CT-\d{4}-\d{4}/ }).first().waitFor();
ok(await m.getByRole("link", { name: /CT-\d{4}-\d{4}/ }).first().isVisible(), "the contract appears in the register");

const c = await page(clientTok);
await c.goto(`${WEB}/contracts`);
await c.getByRole("link", { name: /CT-/ }).first().waitFor();
ok((await c.locator("body").innerText()).includes(`Five-year maintenance ${sfx}`), "the client sees its own signed contract");
ok(await c.getByText("New contract").count() === 0, "the client cannot create contracts");

// ---- Client online forms ----
await m.goto(`${WEB}/forms`);
await m.selectOption("#fr-client", { label: `Modules Co ${sfx}` });
await m.fill("#fr-msg", "Please complete before our visit.");
await m.click("text=Send to client");
await m.waitForURL("**/forms/*");
const formUrl = m.url();
ok(/Site survey/.test(await m.locator("h1").innerText()), "a form is sent to the client");
await c.goto(`${WEB}/forms`);
await c.getByText("form(s) waiting").waitFor();
ok((await c.locator("body").innerText()).includes("1 form(s) waiting"), "the client is told a form is waiting");
await c.goto(formUrl);
await c.fill("#q-site_name", "Douala DC");
await c.fill("#q-address", "Bonanjo, second floor");
await c.fill("#q-contact", "A. Contact +237600000000");
await c.selectOption("#q-power", "Mains and generator");
await c.getByRole("button", { name: "Save and finish later" }).click();
await c.waitForTimeout(500);
await c.reload();
ok((await c.inputValue("#q-site_name")) === "Douala DC", "a partly filled form is kept for later");
await c.fill("#q-site_name", "");
await c.getByRole("button", { name: "Submit", exact: true }).click();
ok(await c.locator("#q-site_name:invalid").count() === 1, "a required question must be answered");
await c.fill("#q-site_name", "Douala DC");
await c.getByRole("button", { name: "Submit", exact: true }).click();
await c.getByRole("heading", { name: "Answers" }).waitFor();
ok(/submitted/.test(await c.locator(".meta").innerText()), "the client submits the form");
await m.goto(formUrl);
await m.fill("#rv-note", "Please confirm the rack count.");
await m.getByRole("button", { name: "Return for changes" }).click();
await m.getByText("returned").first().waitFor();
await c.goto(formUrl);
await c.getByText("Returned for changes").waitFor();
ok((await c.locator("body").innerText()).includes("Please confirm the rack count."), "the client sees why the form came back");
await c.getByRole("button", { name: "Submit", exact: true }).click();
await m.goto(formUrl);
await m.getByRole("button", { name: "Accept" }).click();
await m.getByText("accepted").first().waitFor();
ok(true, "staff review and accept the form");

// ---- Projects: from a quotation, milestones, budget against cost ----
const quote = await call("POST", "/v1/quotes", mgr, { tenant_id: client.tenant_id, title: `Rack refresh ${sfx}`, lines: [{ description: "Supply racks", quantity: 2, unit_price: 500000 }, { description: "Install", quantity: 1, unit_price: 250000 }] });
await call("POST", `/v1/quotes/${quote.id}/send`, mgr);
await call("POST", `/v1/quotes/${quote.id}/approve`, clientTok);
await m.goto(`${WEB}/projects`);
await m.selectOption("#fq-quote", { index: 1 });
await m.fill("#fq-start", day(-5)); await m.fill("#fq-end", day(40));
await m.getByRole("button", { name: "Start project" }).click();
await m.waitForURL("**/projects/*");
const projUrl = m.url();
await m.getByText("Progress").first().waitFor();
ok((await m.locator("body").innerText()).includes("0 of 2 tasks"), "each quoted line became a task");
await m.fill("#ms-title", "Racks delivered"); await m.fill("#ms-due", day(-1));
await m.locator("form", { hasText: "Add a milestone" }).getByRole("button", { name: "Add" }).click();
await m.getByText("late").first().waitFor();
ok((await m.locator("body").innerText()).includes("behind schedule"), "a late milestone marks the project behind schedule");
await m.getByRole("cell", { name: "Mark done" }).getByRole("button").click();
await m.getByText("1 of 1 milestones").waitFor();
ok((await m.locator("body").innerText()).includes("33% done") || (await m.locator("body").innerText()).includes("34% done"), "progress follows tasks and milestones");
const f = await page(fin);
await f.goto(projUrl);
await f.fill("#co-desc", "Racks"); await f.fill("#co-amt", "1500000");
await f.getByRole("button", { name: "Record cost" }).click();
await f.getByText("over budget").first().waitFor();
ok(true, "finance sees spending against the budget and the overrun");
await c.goto(projUrl);
await c.getByText("Progress").first().waitFor();
ok(!(await c.locator("body").innerText()).includes("Record cost") && !(await c.locator("body").innerText()).includes("over budget"), "the client sees progress but no costs");
await m.goto(`${WEB}/projects`);
await m.locator('[role="progressbar"]').first().waitFor();
ok(await m.locator('[role="progressbar"]').count() > 0, "the project list shows a progress bar");

// ---- QR labels ----
const item = await call("POST", "/v1/items", log, { sku: `LB-${sfx}`, name: `Label router ${sfx}`, serialised: true });
const wh = await call("POST", "/v1/warehouses", log, { name: `Label store ${sfx}` });
await call("POST", "/v1/stock/receipts", log, { item_id: item.id, warehouse_id: wh.id, qty: 1, serials: [`SN-${sfx}`] });
const l = await page(log);
await l.goto(`${WEB}/inventory`);
await l.getByRole("tab", { name: "Equipment" }).click();
await l.getByLabel(`Print a label for SN-${sfx}`).check();
await l.getByRole("button", { name: /Preview and print/ }).click();
await l.waitForSelector(".pv-page[data-ready]", { timeout: 20000 });
ok(true, "a QR sticker sheet previews before printing");
await l.keyboard.press("Escape");
await l.getByRole("link", { name: `Label router ${sfx}` }).click();
await l.waitForURL("**/asset?id=*");
await l.getByText(`SN-${sfx}`).first().waitFor();
ok((await l.locator("body").innerText()).includes(`SN-${sfx}`), "scanning a sticker opens the asset");

// ---- Employee file ----
const actor = `mo-emp-${sfx}`;
const emp = await call("POST", "/v1/employees", hr, { actor, full_name: `File Person ${sfx}`, position: "Technician", hire_date: "2024-02-01" });
const h = await page(hr);
await h.goto(`${WEB}/hr`);
await h.getByRole("tab", { name: "Staff" }).click();
await h.getByRole("link", { name: `File Person ${sfx}` }).click();
await h.waitForURL("**/hr/employee/*");
await h.fill("#pv-national_id", "ID-777"); await h.fill("#pv-bank_account", "0001-2222");
await h.getByRole("button", { name: "Save details" }).click();
await h.waitForTimeout(600);
await h.reload();
ok((await h.inputValue("#pv-national_id")) === "ID-777", "personal details are saved on the employee file");
await h.selectOption("#ec-type", "CDI"); await h.fill("#ec-pos", "Senior Technician"); await h.fill("#ec-start", "2024-02-01"); await h.fill("#ec-sign", "2024-01-25");
await h.getByRole("button", { name: "Draft contract" }).click();
await h.getByRole("button", { name: "Activate" }).click();
await h.locator(".meta", { hasText: "Senior Technician" }).waitFor();
ok((await h.locator(".meta").innerText()).includes("Senior Technician"), "activating the contract updates the position");
const mg = await page(token("mo-mgr2", ["manager"]));
await mg.goto(`${WEB}/hr/employee/${emp.id}`);
ok(await mg.locator("#pv-national_id").count() === 0, "a manager sees the file without personal details");

// ---- Bank accounts ----
const acct = `5${String(Date.now()).slice(-6)}`;
await call("POST", "/v1/accounting/accounts", fin, { code: acct, name: `Register bank ${sfx}` });
await f.goto(`${WEB}/accounting`);
await f.getByRole("tab", { name: "Bank accounts" }).click();
await f.selectOption("#ba-ledger", acct);
await f.fill("#ba-bank", "Test Bank"); await f.fill("#ba-num", `ACC-${sfx}`);
await f.getByRole("button", { name: "Register", exact: true }).click();
await f.getByText(`ACC-${sfx}`).waitFor();
ok(true, "a company bank account is registered against its ledger account");

for (const [name, p] of [["manager", m], ["client", c], ["finance", f], ["logistics", l], ["hr", h]]) ok(p.errors.length === 0, `no script errors for the ${name} (${p.errors.join("; ")})`);
await b.close();
