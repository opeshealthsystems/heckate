# HEKATE Platform

ERP, client and supplier portals, public website and mobile app on one API. Requirements: [docs/HEKATE_Platform_Specification.md](docs/HEKATE_Platform_Specification.md). Owner action items: [docs/BLOCKERS.md](docs/BLOCKERS.md).

## Layout

| Path | What it is |
| --- | --- |
| `api/` | Go modular monolith (stdlib HTTP + pgx). One package per bounded context in `internal/modules/`, shared concerns in `internal/platform/`, schema in `migrations/` (embedded, applied on start) |
| `api/openapi/openapi.yaml` | OpenAPI 3.1 contract, linted in CI |
| `web/` | Next.js client portal and staff console |
| `site/` | Next.js static public website (EN/FR, SEO/GEO/AEO), separate host |
| `mobile/` | Expo (React Native) app for clients and staff |
| `deploy/` | Docker compose (Postgres, Keycloak, MinIO, NATS, plus api/web/site), Keycloak realm |
| `e2e/` | Playwright smoke test of the whole flow |

## Status

| Module | State |
| --- | --- |
| Identity (JWT/OIDC, roles, tenant claim) | Built. Keycloak realm provided, **untested** against a real Keycloak |
| Clients, CRM (leads from the website form) | Built |
| Support & Incidents (tickets, SLA, comments, internal notes, rating, report) | Built: web and mobile |
| Sales & Invoicing (quotes, client approval, invoices, payments, credit notes, TVA+CAC, dual control above a threshold) | Built: web |
| Accounting (append-only hash-chained double-entry ledger, auto-posting from sales and purchases, trial balance, chain verification) | Built: web. Starter chart only; accountant must validate |
| Projects & Field Service (projects, tasks, interventions with GPS and signature, timesheets) | Built: web; field check-in/out on mobile |
| Inventory & Assets (stock ledger, serialised equipment, issue to staff, client assets, parts used on interventions) | Built: web |
| Purchasing & Suppliers (supplier register with call-back-verified bank details, RFQs, POs, receipts, supplier invoices with three-way match, supplier portal) | Built: web |
| HR (employee records, leave with balances, expense claims) | Built: web |
| Payroll (salaries, validated rules, monthly runs, payslips, ledger posting) | Built: web. Dual control at every step. **Rates are unvalidated placeholders; no run is possible until a second person validates them** |
| Documents & Trust (register, references, public verification) | Built: register, public verification, PDFs (invoice, quote, site-visit acceptance) with QR and SHA-256 check on /verify. **PAdES signing not built** |
| Push notifications | Built (Expo push); real delivery needs credentials |
| Public website (EN/FR) | Built |
| Reporting dashboards | Per-module reports and the dashboard; Metabase not set up |
| API as a product (keys, plans, metering, developer screen) | Built: read-only company keys, plans with rate limit and monthly quota, usage. Billing for paid plans and SDK generation not built |
| Live updates (WebSocket `/v1/stream`, tickets and interventions) | Built: web and mobile. Chat not built |
| Mobile payments (MTN MoMo, Orange Money) | Not built: payments are recorded manually |
| File attachments | Built, stored in Postgres (max 10 MB; MinIO not used) |

Every endpoint is in `api/openapi/openapi.yaml`, and a test fails if the code and the contract disagree.

## Run it

```
# database (or: make up for the full dev infrastructure)
createdb hekate
export HEKATE_DATABASE_URL='postgres://USER@localhost:5432/hekate?sslmode=disable'
export HEKATE_DEV_JWT_SECRET=dev-secret HEKATE_CORS_ORIGINS=http://localhost:3000,http://localhost:3001

go run ./api/cmd/api                                    # API on :8080, migrates itself
go run ./api/cmd/api devtoken manager,support > staff.tok
(cd web && cp .env.example .env.local && npm install && npm run dev)     # :3000, paste a token to sign in
(cd site && npm install && npm run dev)                                  # :3001
(cd mobile && npm install && npx expo start)
```

`HEKATE_DEV_JWT_SECRET` enables HS256 dev tokens; never set it in production. With it unset the API accepts only Keycloak RS256 tokens (`HEKATE_JWKS_URL`).

## Test

```
make test      # Go unit tests; integration tests run when HEKATE_TEST_DB points at a Postgres
cd web && npm test && npm run typecheck
cd mobile && npm test && npm run typecheck
cd site && npm test && npm run typecheck
```

## Security model in one paragraph

Every request carries a bearer token. The API opens one transaction per request and sets `app.tenant_id` and `app.role` from the token; PostgreSQL row-level security then decides which rows exist for that request, and the app connects as a role that cannot bypass it. A handler bug cannot leak another client's data because the database does not return it. Integration tests prove this with two tenants.
