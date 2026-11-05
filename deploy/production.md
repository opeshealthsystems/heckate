# Production deployment

> Deploying without Docker on a shared Ubuntu server (the planned setup, shared with opesware.com)? Follow **[UBUNTU_SHARED_SERVER.md](UBUNTU_SHARED_SERVER.md)**. This page is the overview; that one is the step-by-step runbook.

## Domains (hekatesystems.com)

| Host | Serves | Built from |
| --- | --- | --- |
| `hekatesystems.com`, `www.hekatesystems.com` | Public website (static export) | `site/`, build vars `NEXT_PUBLIC_SITE_URL=https://hekatesystems.com`, `NEXT_PUBLIC_PORTAL_URL=https://app.hekatesystems.com`, `NEXT_PUBLIC_API_URL=https://api.hekatesystems.com` |
| `app.hekatesystems.com` | ERP, client portal, `/verify` and QR links (`/v/...`) | `web/`, build vars `NEXT_PUBLIC_API_URL=https://api.hekatesystems.com`, `NEXT_PUBLIC_OIDC_ISSUER=https://auth.hekatesystems.com/realms/hekate`, `NEXT_PUBLIC_DEV_LOGIN=0` |
| `api.hekatesystems.com` | Go API | `api/`, settings in `deploy/production.env.example` |
| `auth.hekatesystems.com` | Keycloak (OIDC) | `deploy/keycloak/` |

These are the defaults baked into the Dockerfiles and the env example. A different host name is a build-time change for `site/` and `web/` (the `NEXT_PUBLIC_*` values are compiled in) and a settings change for the API (`HEKATE_PORTAL_URL`, `HEKATE_CORS_ORIGINS`, `HEKATE_ISSUER`, `HEKATE_JWKS_URL`). DNS A/AAAA records for all five names must point at the server before TLS certificates can be issued. The mobile app reads `EXPO_PUBLIC_API_URL` and `EXPO_PUBLIC_OIDC_ISSUER` (see `mobile/.env.production`).

1. **Build** `api/Dockerfile` and `web/Dockerfile`. The web image bakes in `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_OIDC_ISSUER` and `NEXT_PUBLIC_OIDC_CLIENT_ID` as build args; leave `NEXT_PUBLIC_DEV_LOGIN=0`.
2. **Configure** the API from `deploy/production.env.example`, delivered from a secret store. With `HEKATE_ENV=production` the API refuses to start on unsafe settings (dev secret, missing OIDC, `sslmode=disable`, wildcard CORS, non-https or localhost `HEKATE_PORTAL_URL`, sandbox payments) and lists each problem.
3. **Database**: PostgreSQL 16+ with TLS. The API applies migrations on start (`heckate migrate` runs them alone).
4. **Identity**: an OIDC provider (Keycloak) issuing tokens with the `tenant_id` and `roles` claims; the audience must match `HEKATE_AUDIENCE`.
5. **TLS and proxy**: terminate TLS at a reverse proxy in front of both services; set `HEKATE_TRUSTED_PROXIES` to its CIDR. Keep the metrics port internal.
6. **Verify**: `GET /healthz` on the API; log in and open `/verify`. After any restore run `heckate verify-data` (exit 0 = both hash chains intact).
7. **Backups**: nightly `pg_dump` off-site plus restore drills (see `docs/RUNBOOK.md`).
8. **Optional**: set `HEKATE_QR_SIGNING_KEY_FILE` for signed QR codes (the API warns when absent); payment gateways stay off until MTN/Orange credentials are set.

Not yet done and outside this repository: penetration test, off-site backup automation, monitoring/alerting, real Keycloak realm. See `docs/BLOCKERS.md`.
