# Where the platform stands

Last updated with the commit that introduced this file. Numbers are counted from the repository, not estimated.

## The honest verdict

This is a **hardened pre-production build**: well beyond a demo, past a typical MVP in data integrity and access control, but **not production-certified, and no module is "100% production ready"**. Nobody can honestly stamp a system that way without evidence that only time, people and real infrastructure can produce. This file lists exactly what is done, what is missing, and which gates remain, so that nothing is left to a feeling.

What exists today (counted):

| | |
| --- | --- |
| API operations in the OpenAPI contract | 183 |
| Database migrations | 31 |
| Go tests (integration tests run against a real PostgreSQL, as a superuser **and** as an ordinary database owner) | ~110 |
| Browser end-to-end scripts (Playwright) | 8 |
| Languages / apps | Go API, Next.js web ERP and portals, Expo mobile app, static bilingual site, TypeScript SDK |

## What "production ready" has to mean here

A module counts as production ready only when **all** of these are true. Today the right-hand column is empty for every module, because gates 7 to 10 are outside the code.

| # | Gate | Who can close it |
| --- | --- | --- |
| 1 | Functionally complete against `HEKATE_Platform_Specification.md` | Build (me) |
| 2 | Data integrity enforced in the database, not only the app | Build (done for most modules) |
| 3 | Access control tested, including cross-tenant and wrong-role cases | Build (done for most modules) |
| 4 | Automated tests, run in CI on every change | Build (done) |
| 5 | Printable documents the business needs, with verification | Build (35 of ~36, see below) |
| 6 | Dependency, secret and code scanning gating every change | Build (added with this file) |
| 7 | Domain sign-off: the accountant (tax, ledger, payroll), a lawyer (contracts, retention) | **People** |
| 8 | Independent penetration test and fixes | **External firm** |
| 9 | Operated for real: backups restored in a drill, monitoring and alerts, load tested, runbooks, DR | **Infrastructure + people** |
| 10 | Real identity provider, real payment providers, real devices exercised | **Accounts + hardware** |

## Modules in the specification (10) and where each stands

"Built" means working end to end through the API, the database rules and a screen, with tests. "Gap" lists what the specification asks for that is **not** there.

| Module | Built | Gaps |
| --- | --- | --- |
| **CRM** | Client accounts, contacts, leads from the website form, KYC status field | Contracts, KYC / onboarding forms (HS-FRM-07, 13), pipeline reporting, consent records |
| **Support & Incidents** | Tickets, SLA clocks, comments, private staff notes, rating, support report, live updates, push, attachments; web and mobile | Incident report form (HS-FRM-14), on-call roster, SLA calendars and pauses, email intake, knowledge base. **SLA times are placeholders** |
| **Projects & Field** | Projects, tasks, site visits with GPS, signature, parts used; offline queue on mobile; timesheets with approval; acceptance PDF | Kick-off, change request, closure forms (HS-FRM-23, 26, 31), statements of work, milestone billing |
| **Sales & Invoicing** | Quotes, client approval, invoices, dual control above a threshold, payments, credit notes, TVA + CAC, PDFs for invoice, quote, receipt and credit note, proposals, order confirmations and statements of work (text-document engine), online mobile-money payment framework, receivables ageing report | Payment reminders, a designed rate card (the form exists as a generic text document). (Recurring invoice templates exist: a run makes drafts, finance still issues them; nothing is sent automatically. The API runs due templates itself, every hour by default, and the audit log names the scheduler as the actor.) **Tax figures need the accountant** |
| **Accounting** | Append-only hash-chained ledger, balanced entries enforced in the database, posting from sales, purchasing and payroll, trial balance, permanent month close that locks the period in the database, ledger integrity check, bank reconciliation | DSF filing, SYSCOHADA presentation of the statements (a class-based income statement and balance sheet exist), the filed VAT return (a working VAT summary from the ledger exists), fixed assets, a receivables ageing report exists. **Chart of accounts is a starter set** |
| **Purchasing & Suppliers** | Supplier register with call-back-verified bank details, RFQs with sealed bids, purchase orders with dual control, goods receipt, supplier invoices with three-way match, supplier portal | supplier registration and evaluation forms (HS-FRM-39, 41), contract management |
| **Inventory & Assets** | Append-only stock ledger, no negative stock, serialised equipment, issue to staff, client-held assets, parts on interventions, stock counts approved by a second person, reorder report from minimum levels | Counting serialised equipment, barcode scanning, depreciation |
| **HR & Payroll** | Employee records, leave with live balances, expense claims with receipts, payroll engine (rates are data, a second person must validate them, dual control to approve and pay, posts to the ledger), payslips for employees | Recruitment and offers (HS-FRM-32, 33), appraisals, CNPS / tax declaration files, overtime, bonuses, contracts. **Payroll rates are unvalidated** |
| **Documents & Trust** | Document register, references with check characters, public verification page, PDF hash recorded and checkable in the browser | **Digital signatures (PAdES) need a certificate**; letter and certificate templates; retention rules |
| **Reporting** | Per-module reports and a dashboard | BI tool (Metabase), exports to CSV / Excel, scheduled reports, cross-module KPIs |

Also built, outside the ten: identity and roles, an API-as-a-product layer (keys, plans, quotas, usage, TypeScript SDK), tamper-evident audit trail, live updates over WebSockets, push notifications, mobile-money payment framework, a bilingual public website.

## The printable documents

The specification lists about **36** document templates (quotation, proposal, order confirmation, statement of work, invoice, receipt, credit note, 24 numbered forms, penetration test report, purchase order, RFQ, letter, certificate).

- **Rendered as real PDFs today: 35**: invoice (HS-INV), quotation (HS-QT), site-visit acceptance (HS-FRM-27), payment receipt (HS-RCT), credit note (HS-CN), purchase order (HS-PO), request for quotation (HS-RFQ), asset issue and return form (HS-FRM-38), payslip (HS-PAY, confidential; reference issued when the run is approved), and through one text-document engine: proposal (HS-PR), order confirmation (HS-OC), statement of work (HS-SOW), letter (HS-LT), certificate (HS-CRT-A), guarantee policy (HS-GP), contract, penetration test report and 19 numbered HS-FRM forms, each a draft that is frozen and numbered when issued. **Caveat:** these use one generic layout of titled fields; the exact field list and design of each form still has to be agreed with the business, and the certificates lack the security-print features (guilloche, hologram, microtext) and PAdES signing. Each has a QR code to the public verification page and its SHA-256 recorded; the first rendering is stored so one reference is always one file.
- **Not started: the dedicated layouts for the 3 certificate templates and the legal contract wording.**

The company letterhead lines are configuration (`HEKATE_LETTERHEAD`), and PDFs use built-in Latin-1 fonts. Cryptographic signing (PAdES) is not done because it needs a signing certificate.

## What has actually been verified, and what has not

Verified by running it: every API rule above through integration tests; row-level security as a non-superuser; database triggers against raw SQL; the web app in a real browser (8 scripts); the mobile app typechecks, passes `expo-doctor` and bundles for Android.

**Not** verified, because it needs something I do not have:

- The Keycloak realm file and real token verification against it
- The Docker images and the full compose stack
- The mobile app on a physical phone
- MTN MoMo against the real service (written against the public API and tested only with a fake server); the Orange Money client is written the same way (off until credentials are set) and equally unverified
- Backups and restore, failover, load and soak tests
- Any penetration test

## What happens next

Order of work, highest value first. Items marked (people) cannot be finished by code alone.

1. **Security evidence** (in progress; see `SECURITY.md`): hardening, scanning in CI, audit trail, an auditor-facing evidence pack.
2. **Operational readiness:** health and readiness probes, metrics, structured logs, backup and restore scripts with a drill, runbooks, load tests.
3. **The remaining ~29 document templates**, in the order the business uses them.
4. **Accounting completeness:** statements, period close, VAT, fixed assets (the accountant must validate: people).
5. **Remaining module gaps** in the table above.
6. **Integrations when credentials exist:** MTN MoMo, Orange Money, Keycloak, push, signing certificate (people).
7. **Independent penetration test and fixes** (people).
