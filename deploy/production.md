# Production deployment

1. **Build** `api/Dockerfile` and `web/Dockerfile`. The web image bakes in `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_OIDC_ISSUER` and `NEXT_PUBLIC_OIDC_CLIENT_ID` as build args; leave `NEXT_PUBLIC_DEV_LOGIN=0`.
2. **Configure** the API from `deploy/production.env.example`, delivered from a secret store. With `HEKATE_ENV=production` the API refuses to start on unsafe settings (dev secret, missing OIDC, `sslmode=disable`, wildcard CORS, non-https or localhost `HEKATE_PORTAL_URL`, sandbox payments) and lists each problem.
3. **Database**: PostgreSQL 16+ with TLS. The API applies migrations on start (`heckate migrate` runs them alone).
4. **Identity**: an OIDC provider (Keycloak) issuing tokens with the `tenant_id` and `roles` claims; the audience must match `HEKATE_AUDIENCE`.
5. **TLS and proxy**: terminate TLS at a reverse proxy in front of both services; set `HEKATE_TRUSTED_PROXIES` to its CIDR. Keep the metrics port internal.
6. **Verify**: `GET /healthz` on the API; log in and open `/verify`. After any restore run `heckate verify-data` (exit 0 = both hash chains intact).
7. **Backups**: nightly `pg_dump` off-site plus restore drills (see `docs/RUNBOOK.md`).
8. **Optional**: set `HEKATE_QR_SIGNING_KEY_FILE` for signed QR codes (the API warns when absent); payment gateways stay off until MTN/Orange credentials are set.

Not yet done and outside this repository: penetration test, off-site backup automation, monitoring/alerting, real Keycloak realm. See `docs/BLOCKERS.md`.
