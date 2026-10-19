# HEKATE Platform

ERP, client/supplier portals and mobile app on one API. Full requirements: [docs/HEKATE_Platform_Specification.md](docs/HEKATE_Platform_Specification.md).

## Layout

| Path | Purpose |
| --- | --- |
| `api/` | Go modular monolith. One package per bounded context under `internal/modules/`; shared HTTP concerns in `internal/platform/` |
| `api/openapi/openapi.yaml` | OpenAPI 3.1 contract. Edit it before writing handlers |
| `db/migrations/` | PostgreSQL 16 schema, one schema per module, row-level security |
| `deploy/` | Local dev stack: Postgres, Keycloak, MinIO, NATS JetStream |

## Status

Phase 0 scaffold. Support & Incidents routes return 501 until Phase 1. Not yet scaffolded: Next.js web (`web/`), Expo app (`mobile/`), public site (`site/`), Keycloak realm export, outbox relay.

## Develop

```
make up      # dev dependencies
make test
make run     # API on :8080, GET /healthz
```
