# Platform audit

A running record of what was checked, what was found, what was fixed and what remains. Each finding has an ID, a severity (our own judgement, assuming real client data), and a status. Fixes name the commit's test, so each one can be re-checked.

Status key: **Fixed** (change plus a test), **By design** (explained), **Open** (needs a decision or a person).

## 1. Data layer (69 tables, 31 migrations)

Checked: row-level security (RLS) and FORCE on every table, policies per table, privileges of the application role, definer functions, primary keys, foreign-key indexes.

| ID | Finding | Severity | Status |
| --- | --- | --- | --- |
| D-01 | `platform.outbox` had no RLS: any request context could read every tenant's event payloads by SQL (no endpoint did) | Medium | **Fixed** (0028): requests may append; only the relay, which has no request role, may read or mark. `TestEveryTableHasRowLevelSecurity` |
| D-02 | `platform.idempotency_keys` had no RLS: a caller could read other callers' stored responses by SQL | Medium | **Fixed** (0028): a caller sees and stores only their own |
| D-03 | `documents.counters` had no RLS | Low | **Fixed** (0028): staff only |
| D-04 | `procurement.settings` (approval threshold) readable by every role | Low | **Fixed** (0028): staff only |
| D-05 | `sales.settings` (tax rates and approval threshold) had no RLS. Clients legitimately read the rates to see their own quote totals, so it stays readable by any signed-in caller | Low | **Fixed in part** (0028): not readable anonymously. The threshold is still visible to a signed-in client who can run SQL; split it from the rates to close that |
| D-06 | `support.sla_policies`, `platform.audit_log` had no RLS | Low | **Fixed** (0028): SLA times readable (they are promised to clients); the audit log is append-only for requests |
| D-07 | 8 tables have RLS but not FORCE | Info | **By design**: SECURITY DEFINER functions owned by the schema owner must reach them. The application role is neither owner nor superuser nor BYPASSRLS, so RLS applies to it. Now listed with reasons in `schema_test.go`; a new unlisted one fails the test |
| D-08 | 31 foreign keys had no index (joins and per-client row filters would degrade into table scans as data grows, for example invoice and quote lines, credit notes, every `tenant_id`) | Medium | **Fixed** (0029). `TestEveryForeignKeyIsIndexed` fails for any new one |
| D-09 | Application role privileges | Info | Checked, no change: no TRUNCATE anywhere; DELETE only on draft-state tables guarded by triggers and policies; no PUBLIC grants; every definer function pins `search_path` and is not executable by PUBLIC. `TestApplicationRoleCannotRewriteHistory` keeps the append-only tables that way |
| D-10 | Every table has a primary key | Info | Checked |

New regression tests: `TestEveryTableHasRowLevelSecurity`, `TestApplicationRoleCannotRewriteHistory`, `TestEveryForeignKeyIsIndexed` (the first was shown to fail on a deliberately unprotected table).

## 2. Access control (RBAC): 179 API routes by 15 identities

Method: `internal/server/rbac_matrix_test.go` calls every route as every role (14 roles plus anonymous) and records whether the role gate lets the call through; `rbac_visibility_test.go` then creates real invoices, quotes, tickets, projects and documents for one client and asks each role to list or open them, because a role that passes the gate can still be shown too much by row-level security. Both are standing tests now.

| ID | Finding | Severity | Status |
| --- | --- | --- | --- |
| R-01 | **Every staff role could read every client's invoices, quotes, payments, credit notes and recurring templates.** A plain employee, or someone in HR, logistics or procurement, saw all of them (the spec gives an employee self-service only). Row-level security treated "staff" as one role | High | **Fixed** (0030): only management, finance and sales; the same on the API route lists. `TestLeastPrivilegeVisibility` |
| R-02 | **Every staff role could read every text document, including restricted forms** (background screening, KYC, penetration test reports), proposals and contracts (introduced with the document engine) | High | **Fixed** (0030): readable and writable by kind and role (restricted forms: management only; HR the hiring forms; support and engineers the incident forms; and so on). `TestNarrativeKindsFollowRoles` |
| R-03 | **Every staff role could see every client's API-key metadata and usage** | Medium | **Fixed** (0030): management and support; clients their own |
| R-04 | **The external auditor could read individual payslips** (the spec says auditors view ledgers and documents) | High | **Fixed** (0030): the auditor sees run totals and the ledger, not individual pay. Asserted in `payroll_test.go` |
| R-05 | **Every staff role could read every client's tickets.** HR, procurement, logistics, finance, sales and plain employees saw all of them | Medium | **Fixed** (0030): management, support and engineers work all tickets; everyone else sees only tickets they raised. Comments follow the ticket |
| R-06 | **The live stream sent every ticket and site-visit event to every staff connection**, so protected records could still be learned from events | Medium | **Fixed**: the stream applies the same rule; ticket events now carry the reporter. `stream_test.go` |
| R-07 | 40 routes had no role list at all (open to any signed-in caller, including suppliers and the auditor); suppliers and the auditor could pass the gate on client-facing routes such as invoices, quotes, tickets and projects, and the auditor could call ticket write routes (row-level security returned nothing, so no data moved, but the gate was the only defence missing) | Medium | **Fixed**: role lists on all of them (`platform.Portal` is all staff and clients; money routes are narrower). `TestRouteGatesStayNarrow` fails if a route opens to anonymous callers, the auditor writes, a supplier writes outside purchasing, or a plain employee writes outside self-service |
| R-08 | API keys were limited to routes with no role list; giving those routes lists would have silently disabled all key reads | Info | **Fixed**: a key acts as a `client_user`, so it may call a route only if that role may (still GET only, still scoped, still never keys, plans, devices or the stream) |
| R-09 | The web navigation showed Quotes and Invoices to every staff role | Low | **Fixed**: shown only to roles that can use them |
| R-10 | Plain `employee` and the other staff roles still see **all projects, site interventions and stock** (timesheets and field work need the project list) | Low | **By design, recorded**: internal projects are shared with staff. Revisit if project names ever carry client secrets |
| R-11 | `sales.settings` approval threshold is readable by a signed-in client who can run SQL | Low | **Open**: split the threshold from the rates (see D-05) |

Checked and fine: anonymous callers reach only the three health probes, public verification and the website enquiry; the auditor writes nothing; suppliers write only through purchasing; the payment webhook rejects unsigned calls; the matrix shows no mutating route open to every role.

Re-verified after the changes: the whole Go suite (including the non-superuser run in CI) and all nine browser scripts (smoke, erp, documents, payroll, bank, payments, attachments, live, csp).

## 3. Screens, tabs, forms and buttons, per role

A Playwright crawl signs in as each of 14 identities (ceo, manager, sales, support, engineer, finance, hr, procurement, logistics, employee, auditor, client admin, client user, supplier), opens every menu entry and every tab, and records page errors, console errors, failed API calls, horizontal overflow at 390 px and axe-core violations (WCAG 2 A/AA).

| Finding | Fix |
| --- | --- |
| Auditor and supplier were sent to a ticket queue their role may not read (403 on dashboard and Tickets) | Menu entry removed for them; the dashboard shows a plain notice instead |
| Procurement and logistics saw Time and Inventory links they cannot use (403) | Links shown only to roles the API allows |
| New-ticket form asked for the client list for roles that cannot list clients (403) | Fetched only for ceo, manager, sales, support, finance, engineer |
| Procurement page asked for RFQs (finance) and warehouses (procurement) it may not read | Fetched only for roles the API allows |
| Empty table header cells (action columns) failed axe `empty-table-header` | Header renders a screen-reader "Actions" label |
| Supplier portal scrolled sideways at 390 px: the screen-reader-only label in a table header was positioned against the page, not the scrolling wrapper | Wrapper is now a positioned box |

Result after fixes: 0 page errors, 0 console errors, 0 axe violations, 0 horizontal overflow on all 14 identities. The only remaining 4xx is `GET /v1/employees/me` returning 404 for identities with no employee record; the People page shows a notice for it by design.

End-to-end scripts (erp, documents, payroll, bank, attachments, csp, live) pass with 0 failures. `smoke` and `payments` need extra set-up (pre-minted tokens; sandbox payment provider) and were not re-run in this pass.

## 4. Colours, status and styling

* All colours come from the brand tokens in `web/app/globals.css`, `site/app/globals.css` and `mobile/src/lib/theme.ts`. Three deliberate, documented deviations keep text legible: gold text `#8B6B2B`, input border `#788EAC`, PDF "CONFIDENTIAL" banner `#A96500` (see `BRAND_PACK.md`).
* Status chips were inconsistent: only "ok" and "bad" tones existed and each screen chose them by hand. `Pill` now derives the tone from the status word (green: approved, paid, posted, reconciled, received, awarded, closed, resolved; red: rejected, cancelled, overdue, failed, void; amber: pending, submitted, draft, open, partial), so one state has one colour everywhere. Colour is never the only signal; every chip carries its text.
* Severity (P1 red, P2 amber) reuses the same red and amber as status, so a "danger" colour means the same thing on every screen. No two meanings share a colour.
* Contrast: axe colour-contrast reports no violations on any crawled screen.

## 5. Known gaps (not fixed in this pass)

* No icon set in the ERP: controls use text labels (✕ and ☰ are the only glyphs). This is consistent and accessible; a brand icon set would be a design task.
* Mobile screens were checked for theme tokens, not pixel-compared with the phone designs. The web dashboards now follow the approved staff and client layouts.
* Pack form layouts (exact positions), certificate security print and PAdES signing remain open (see `BLOCKERS.md`).
* Payment reminders are built (push to the client's admins at 1, 7 and 30 days late). The Orange Money client is built but off and unverified against the real service; the redirect to its payment page is not wired into the UI.
* Independent penetration test, Keycloak in production, off-site backups and monitoring remain pre-requisites for regulated clients (see `READINESS.md`).
