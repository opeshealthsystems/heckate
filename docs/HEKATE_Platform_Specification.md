# HEKATE Platform — ERP & Mobile App Specification

Version 1.0 · 19 October 2026 · HEKATE SYSTEMS SARL

One platform runs HEKATE: a web ERP hosted online and a mobile app on the same API, used by employees, clients and suppliers, with client support and incident handling at its centre.

## Architecture

| Layer | Components | Access |
| --- | --- | --- |
| Users | Clients and employees (mobile app, browser), suppliers (portal) | Internet, HTTPS only |
| Perimeter | WAF + reverse proxy; VPN for admin access | Public 443; admin via VPN only |
| Identity | Keycloak SSO, two-factor login, roles | All apps authenticate here |
| Application | Go API (single API for web and mobile); Next.js web ERP and portals; WebSockets + FCM push | Behind WAF |
| Data | PostgreSQL with row-level security (primary + replica); MinIO file storage; Metabase reporting | Internal network only |
| Operations | HCDP monitoring, logging, alerts; encrypted off-site backup | Internal + off-site |
| Public website | Static site on a separate host | No link to ERP data |

Flow: every request passes the WAF, is authenticated by Keycloak, served by the Go API, and reaches data only through row-level security; nothing in the data layer is reachable from the internet.

## Architecture style

Modular monolith, API-first, domain-driven and event-driven. One Go service split into strict domain modules; any module can be extracted into its own service without a rewrite when load justifies it.

| Pattern | Decision | Rule |
| --- | --- | --- |
| Microservices | Not at launch | Modules are deployable units in waiting; extraction order: Support, Identity, Finance |
| API-first | Yes | Every endpoint defined in OpenAPI 3.1 before code; SDKs for web (TypeScript) and mobile (Dart) generated from the contract; contract tests in CI |
| API as a product | Yes | Developer portal, API keys, plans, rate limits, `/v1` versioning, 12-month deprecation notice, usage metering for billing |
| Domain-Driven Design | Yes | Bounded contexts: CRM, Sales, Support & Incidents, Projects, Finance, Inventory & Assets, Procurement, HR, Documents, Identity, Platform. Each owns its schema; no cross-module table reads |
| CQRS | Selective | Support & Incidents, Finance, Identity, dashboards: commands write to PostgreSQL, queries read from projections |
| Event-driven | Yes | Domain events (`ticket.created`, `invoice.paid`, `id.enrolled`) via transactional outbox to NATS JetStream; webhooks for external systems; idempotent consumers |
| Event sourcing | Ledger and identity only | Append-only, hash-chained records; no updates or deletes |
| Authentication | OAuth 2.1 + OpenID Connect (Keycloak) | Authorization Code + PKCE for web and mobile; Client Credentials for machine-to-machine; mTLS for government links; WebAuthn/TOTP two-factor |
| Authorization | RBAC + ABAC | Roles and attributes in the token; tenant and row-level security enforced in PostgreSQL |
| Real-time | WebSockets + push | Live chat and incident updates; FCM/APNs notifications |

## API conventions

- REST over HTTPS, JSON, resource names in plural (`/v1/tickets`, `/v1/invoices`).
- Cursor pagination, `Idempotency-Key` on every POST, ETags for concurrency.
- Errors in RFC 9457 Problem Details format.
- Every request carries a trace ID; every write lands in the audit log.
- Rate limits per key and per tenant, returned in `RateLimit-*` headers.
- Webhooks signed with HMAC-SHA256, retried with exponential backoff.

## Public website and landing page

The public site (hekatesystems.com) is static and hosted separately from the ERP. Sections: hero with the client-portal sign-in, services, platform (ERP, support, identity), trust and security, sectors served, client support entry, contact, footer with verification link for HEKATE documents.

## Platform screens

| Screen | Users | Purpose |
| --- | --- | --- |
| Home (landing) | Public | Services, platform, identity, trust, support entry, verification |
| About | Public | Company, founders, legal identity, values, locations |
| Contact | Public | Enquiry form, emergency incident line, office map, departments |
| Sign in | Clients, staff, suppliers | Email + password, two-factor, passkey, SSO |
| Client dashboard | Clients | Tickets and incidents, quotes, invoices, contracts, reports, certificates, assets |
| Staff dashboard | Employees | Ticket queue, SLA timers, projects, timesheets, approvals, documents |
| Document verification | Public | Check any HEKATE document by QR or reference |
| Mobile app | Clients and staff | Same functions, push notifications, offline drafts |

## Documents, forms and certificates in the platform

Every printable document built for HEKATE is a platform template. The platform fills it from live data, assigns the reference, signs it (PAdES), adds the QR verification code and logs it in the document register. Paper versions remain available for offline use.

| Code | Name | Module | Classification |
| --- | --- | --- | --- |
| HS-QT | Quotation | Sales | Confidential |
| HS-PR | Commercial and Technical Proposal | Sales | Confidential |
| HS-OC | Order Confirmation | Sales | Confidential |
| HS-SOW | Statement of Work | Sales / Projects | Confidential |
| HS-INV | Invoice | Finance | Confidential |
| HS-RCT | Receipt | Finance | Confidential |
| HS-CN | Credit Note | Finance | Confidential |
| HS-FRM-23 | Change Request Form | Projects | Confidential |
| HS-FRM-24 | Rate Card | Sales | Confidential |
| HS-FRM-06 | Transmittal and Fax Cover Sheet | Documents | Confidential |
| HS-FRM-07 | Client Information and Onboarding Form | CRM | Confidential |
| HS-FRM-08 | Service Request and Requirements Questionnaire | CRM / Support | Confidential |
| HS-FRM-09 | Penetration Test Scoping Form | Projects | Restricted |
| HS-FRM-10 | Security Assessment Pre-Engagement Questionnaire | Projects | Confidential |
| HS-FRM-11 | hecID e-KYC Integration Request | Identity | Confidential |
| HS-FRM-12 | Background Screening Request | Identity | Restricted |
| HS-FRM-13 | Client KYC and Due-Diligence Form | CRM / Compliance | Restricted |
| HS-FRM-14 | Incident Report and Emergency Response Request | Support & Incidents | Restricted |
| HS-FRM-15 | Customer Feedback and Satisfaction Form | Support | Internal |
| HS-FRM-26 | Project Kick-off Record | Projects | Confidential |
| HS-FRM-27 | Delivery Note and Acceptance Certificate | Projects | Confidential |
| HS-FRM-30 | Timesheet and Service Log | Projects / HR | Internal |
| HS-FRM-31 | Project Closure and Sign-off | Projects | Confidential |
| HS-RPT | Penetration Test Report | Projects | Restricted |
| HS-FRM-32 | Job Application Form | HR | Confidential |
| HS-FRM-33 | Offer of Employment | HR | Confidential |
| HS-FRM-35 | Leave Request | HR | Internal |
| HS-FRM-36 | Expense Claim | HR / Finance | Internal |
| HS-FRM-37 | Visitor and Laboratory Access Log | Facilities | Internal |
| HS-FRM-38 | Asset Issue and Return Form | Inventory & Assets | Internal |
| HS-FRM-39 | Supplier Registration Form | Procurement | Confidential |
| HS-PO | Purchase Order | Procurement | Confidential |
| HS-FRM-41 | Supplier Evaluation Form | Procurement | Internal |
| HS-RFQ | Request for Quotation | Procurement | Confidential |
| HS-LT | Letter on letterhead | Documents | Per content |
| HS-CRT-A | Certificate of Assessment (template A) | Certificates | Public, verifiable |
| HS-ASM | Assessment Certificate (template B) | Certificates | Public, verifiable |
| HS-CRT-C | Certificate (template C) | Certificates | Public, verifiable |
| HS-GP | Guarantee Policies | Legal / Support | Public |

Contract templates (NDA, Master Services Agreement and SLA, Data Processing Addendum, Pentest Authorisation and Rules of Engagement, Employment Contract, Screening Consent) are generated the same way from the Legal module.

Reference format: `HS-<CODE>-<YYYY>-<NNNNNN>`, with a `-<check>` character on certificates. Certificates carry the security features already built: guilloche, UV plate, hologram, microtext, Code128 barcode and signed QR.

## Guarantee policies

The full policy is in the separate document "HEKATE Guarantee Policies" (HS-GP). The platform enforces it: SLA timers on every ticket, automatic service credits on breach, warranty periods recorded against each delivery, and re-test entitlements tracked per engagement.

## Responsive design

Every screen of the website, web ERP and portals is built mobile-first and desktop-complete: one fluid layout that adapts to any width from 320 px phones to 2560 px monitors, with no separate mobile site.

- Fluid grids (CSS grid auto-fit, flex-wrap) and fluid type (clamp); breakpoints at 480, 768, 1024, 1280 and 1600 px.
- Navigation collapses to a menu on phones; side menus stack above content; wide tables scroll inside their box.
- Touch targets at least 44 px; no hover-only actions.
- Tested on Android and iOS phones, tablets, laptops and large monitors in portrait and landscape.
- The Flutter mobile app uses the same layouts and adapts to phone and tablet sizes.

## Search and discovery optimisation

SEO, GEO (generative-engine optimisation) and AEO (answer-engine optimisation) are built into the public website from day one, not added later.

- **SEO:** server-rendered pages, clean URLs, one H1 per page, meta titles and descriptions, XML sitemap, robots.txt, canonical tags, hreflang for English and French, Core Web Vitals targets (LCP under 2.5 s, CLS under 0.1, INP under 200 ms), image alt text, Google Business Profile for the Douala office.
- **AEO:** FAQ sections on service pages with FAQPage schema, direct question-and-answer copy ("What does a penetration test cost in Cameroon?"), HowTo schema for support steps.
- **GEO:** Organization, LocalBusiness and Service schema (JSON-LD) with legal name, RCCM, address and founders; consistent facts on every page; an llms.txt file; citable service and policy pages so AI assistants can quote HEKATE accurately.
- Client portal, dashboards and ERP pages are excluded from indexing (noindex, behind sign-in).

## Modules

Twelve modules share one database and one API; the website and client portal are views on the same data.

| Module | What it does | Main users |
| --- | --- | --- |
| Website & CMS | Public site, service pages, blog, contact and quote forms feeding the CRM | Public, marketing |
| CRM | Leads, opportunities, pipeline, quotations, follow-ups, client history | Sales, management |
| Clients | Client accounts, contacts, contracts, KYC status (HS-FRM-07, HS-FRM-13), documents | All staff, clients (own data) |
| Support & Incidents | Tickets, incident reports, SLA timers, escalation, on-call, chat, attachments, satisfaction rating | Support, engineers, clients |
| Projects & Field Service | Projects, tasks, interventions, site visits, timesheets, delivery and acceptance PV | Engineers, technicians, PMs |
| Sales & Invoicing | Quotations, orders, invoices, credit notes, receipts, TVA + CAC, payment tracking (MoMo, bank) | Sales, finance, clients |
| Accounting | SYSCOHADA chart of accounts, journals, bank reconciliation, ledgers, trial balance, DSF tax statements | Finance |
| Purchasing & Suppliers | Supplier register and evaluation, RFQs, purchase orders, receipts | Procurement, suppliers (portal) |
| Inventory & Assets | Stock, warehouses, serial numbers, equipment issued to staff, client assets under contract | Logistics, engineers |
| HR & Payroll | Staff records, contracts, leave, expenses, CNPS and IRPP payroll | HR, all staff (self-service) |
| Documents & Trust | Document register, signed PDFs, QR verification, certificates (HS-CERT) | All staff, public verify page |
| Reporting | Dashboards: revenue, pipeline, SLA performance, incidents, cash, stock | Management |

## Users and roles

Every user sees only what their role allows, and clients and suppliers see only their own records.

| Role | Web ERP | Mobile app | Can do |
| --- | --- | --- | --- |
| CEO / Manager | Yes | Yes | Everything, approvals above thresholds, all reports |
| Finance | Yes | Read-only | Accounting, invoicing, payments, payroll |
| Sales | Yes | Yes | CRM, quotations, client accounts |
| Support agent | Yes | Yes | Tickets, chat, first response, escalation |
| Engineer / technician | Yes | Yes | Assigned tickets, interventions, timesheets, photos, client sign-off |
| HR | Yes | Read-only | Staff, leave, payroll |
| Employee (all) | Self-service | Yes | Own leave, expenses, payslips, assets, notifications |
| Client user | Portal | Yes | Open tickets and incidents, chat, quotes, invoices, contracts, documents, verify certificates |
| Client admin | Portal | Yes | Manage own company users, approve quotes |
| Supplier | Portal | No | Answer RFQs, see POs, upload invoices, update bank details (with call-back check) |
| Auditor (external) | Read-only, time-limited | No | View ledgers and documents |

## Mobile app

One app (Android and iOS) with two experiences chosen at login: staff and client. It talks only to the platform API, never to the database.

**Client side**

- Report an incident in three taps: category, severity, description, photos or screenshots; urgent ones trigger the on-call engineer immediately
- Open and follow support tickets with live status, SLA countdown and engineer assigned
- Chat with support (text, files, voice notes); call-back request
- Push notifications at each status change; rate the service when closed
- Quotes to approve, invoices to pay (MTN MoMo, Orange Money, card), contracts and documents
- Scan a HEKATE QR code to verify a certificate or document

**Staff side**

- Ticket queue, accept and update tickets, internal notes, escalation
- Field interventions: check-in with GPS at client site, photos, parts used from stock, client signature on screen, auto-generated intervention report
- Timesheets, leave and expense requests with receipt photos, payslips
- On-call schedule and incident alerts
- Works offline in the field and syncs when back online

**Support and incident flow**

1. Client reports by app, portal, e-mail (incident@hekatesystems.com) or phone; every channel becomes one ticket.
2. Ticket is classified by severity (P1 critical to P4 low); the SLA clock starts.
3. Auto-assigned to the right queue; P1 pages the on-call engineer and the Manager.
4. Engineer works remotely or on site; every action is logged.
5. Client confirms resolution in the app and rates it; a P1/P2 gets a written incident report.
6. SLA breaches and repeated incidents show on the management dashboard.

## Technology stack and hosting

A custom build, generated with AI and reviewed module by module, on proven open-source parts and hosted on HEKATE's own servers.

| Layer | Choice | Why |
| --- | --- | --- |
| Backend API | Go | Memory-safe, fast, small attack surface, one binary to deploy |
| Web ERP and portals | TypeScript + Next.js (React) | One codebase for staff ERP, client and supplier portals |
| Public website | Next.js static export (separate host) | Nothing on the public site can reach the ERP |
| Mobile app | Flutter (Android + iOS) | One codebase, offline support, good camera/GPS/signature support |
| Database | PostgreSQL 16 + row-level security | Clients and suppliers can only read their own rows, enforced in the database |
| Files | MinIO (S3-compatible), encrypted | Attachments, photos, signed PDFs |
| Login | Keycloak | Single sign-on, two-factor login, roles in one place |
| Real-time chat and notifications | WebSockets + Firebase Cloud Messaging | Live ticket chat and push alerts |
| Search and reporting | PostgreSQL + Metabase | Dashboards without exposing raw data |
| Hosting | 2 × DL380 Gen11 (app + API, active/standby), 1 × DL380 Gen11 (database primary), 1 × DL380 Gen10 (database replica + backups), 2 × DL380 Gen10 (monitoring/HCDP, test environment) | Uses the six servers in the RFQ; Proxmox virtualisation |
| Off-site | Encrypted nightly backup to a second location or cloud | Survives a fire or theft at the office |

## Security requirements

The platform must pass an internal and an external penetration test before go-live.

- **Perimeter:** only the reverse proxy and web application firewall face the internet; the database and admin tools are reachable only through VPN.
- **Login:** two-factor login for every staff account; passwordless or OTP for clients; account lockout and alerts on suspicious logins.
- **Access:** role-based permissions plus database row-level security; every approval above the thresholds needs two people.
- **Data:** TLS 1.3 everywhere; database and file storage encrypted at rest; personal data handled under Law 2024-017 and the Data Processing Addendum (HS-DOC-34).
- **Mobile:** certificate pinning, no data stored unencrypted on the phone, remote logout of lost devices.
- **Audit:** every create, change, delete and login logged and sent to HCDP; logs kept 12 months.
- **Code:** automated tests, Semgrep code scanning and dependency scanning on every change; no code reaches production without review.
- **Recovery:** daily encrypted backups, monthly restore test, target recovery within 4 hours.

## Build plan

Support and incidents come first because clients feel them first; accounting comes last because it is the riskiest.

| Phase | Delivers | Gate before next phase |
| --- | --- | --- |
| 0 | Specification, data model, login (Keycloak), servers and network | Architecture approved |
| 1 | Clients, CRM, Support & Incidents (web + mobile app), public website | Pilot with 2–3 friendly clients |
| 2 | Projects & field service, quotations, invoicing, mobile payments | First real invoices issued |
| 3 | Purchasing & suppliers portal, inventory & assets | Stock count matches |
| 4 | HR & payroll, employee self-service | One payroll run checked by an accountant |
| 5 | Accounting (SYSCOHADA), reporting dashboards | Two quarters run in parallel with the accountant's books |
| Go-live | Full platform | Internal + external penetration test passed |

## Open decisions

- [ ] Platform name (internal and for clients)
- [ ] Host on own servers only, or own servers plus a cloud copy for the public site and app API
- [ ] Mobile payment providers to integrate first (MTN MoMo, Orange Money, bank card)
- [ ] Languages in the app: English and French at launch?
- [ ] SLA levels and response times to promise clients (P1–P4)
- [ ] Who approves code going to production (two named people)
