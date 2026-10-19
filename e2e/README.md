# End-to-end tests

Drive the real web app against a real API and Postgres with Playwright (Chromium). Scripts:

| Script | Covers |
| --- | --- |
| `smoke.mjs` | Support: client reports a P1, staff works it, internal notes stay hidden, client confirms and rates, dashboard report, role blocking, website enquiry to CRM, document issue and public verification |
| `erp.mjs` | ERP: quote, client approval, invoice, mobile-money payment, ledger and hash-chain check, leave request and approval, and no sideways scroll at 360 px on every dense screen. It also asserts the session is not dropped |
| `documents.mjs` | Downloading an invoice PDF, checking that exact file (and a tampered copy) on `/verify`, and a client admin creating, using and revoking an API key |

## Run

```
# 1. Database and API (HEKATE_DEV_JWT_SECRET enables dev tokens; never set it in production)
createdb hekate_e2e
HEKATE_DATABASE_URL='postgres://USER@localhost:5432/hekate_e2e?sslmode=disable' \
HEKATE_DEV_JWT_SECRET=e2e-secret HEKATE_CORS_ORIGINS=http://localhost:3000 go run ./api/cmd/api

# 2. Web (build with the API address baked in)
cd web && NEXT_PUBLIC_API_URL=http://localhost:8080 NEXT_PUBLIC_DEV_LOGIN=1 npm run build && npm start

# 3. Tests (need playwright-core and a Chromium; set CHROMIUM=/path/to/chromium if not at /opt/pw-browsers/chromium)
HEKATE_DEV_JWT_SECRET=e2e-secret node e2e/erp.mjs
```

`smoke.mjs` also needs `staff.tok` and `client.tok` (see the comments at its top): mint them with `go run ./api/cmd/api devtoken ...`.

Notes: `npm start` warns that `output: standalone` prefers `node .next/standalone/server.js`; production images use that (see `web/Dockerfile`). Stop an old web server before rebuilding, or it will keep serving stale chunks.
