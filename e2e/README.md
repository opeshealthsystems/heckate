# End-to-end smoke test

Drives the real web app against a real API and Postgres with Playwright (Chromium).
Covers: client reports a P1, staff works it, internal notes stay hidden, client confirms and rates,
dashboard report, role blocking, website enquiry to CRM, document issue and public verification,
and the 360 px layout.

```
# 1. API + database (HEKATE_DEV_JWT_SECRET enables dev tokens; never set it in production)
HEKATE_DEV_JWT_SECRET=e2e-secret HEKATE_CORS_ORIGINS=http://localhost:3000 go run ./api/cmd/api
# 2. Web
cd web && NEXT_PUBLIC_DEV_LOGIN=1 npm run build && npm start
# 3. Tokens: create a client with the staff token, then mint a client token for its tenant
#    go run ./api/cmd/api devtoken manager,support   > staff.tok
#    go run ./api/cmd/api devtoken client_user <tenant_id> > client.tok
# 4. node e2e/smoke.mjs   (needs playwright-core and Chromium)
```
