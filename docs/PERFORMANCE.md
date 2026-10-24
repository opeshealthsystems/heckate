# Performance

These are **measurements, not promises**. They show where the platform stands and what was found by measuring; they are not a capacity guarantee for any production deployment.

## How it was measured

- Tool: `api/cmd/loadtest` (build: `go build -o loadtest ./cmd/loadtest`). Mix of 10% ticket creation, 45% ticket list, 45% `GET /v1/me`, spread over 20 client companies, each client calling as their own user.
- One API instance, PostgreSQL 16 with default settings, **all on one 4-vCPU machine that also ran the load generator**. The database held about 30,000 tickets. Dev-token authentication (no identity-provider round trip), no TLS, no network between the parts.
- API started with `HEKATE_RATE_LIMIT` raised, because the default limit (300 requests per minute per caller) correctly rejects a single synthetic user. Pool: 25 database connections.

## Results

| Scenario | Throughput | p50 | p95 | p99 | Errors |
| --- | --- | --- | --- | --- | --- |
| Mix, 10 clients | 2,900 req/s | 1.9 to 5.1 ms | 5 to 10 ms | 8 to 13 ms | 0 |
| Mix, 50 clients | 3,500 req/s | 11 to 19 ms | 26 to 38 ms | 34 to 48 ms | 0 |
| Mix, 200 clients | 3,600 req/s | 52 to 59 ms | 76 to 87 ms | 92 to 102 ms | 0 |
| Client ticket list only, 50 clients | 4,100 req/s | 11 ms | 25 ms | 33 ms | 0 |
| `GET /v1/me` only, 50 clients | 6,800 req/s | 6 ms | 16 ms | 23 ms | 0 |

Past about 3,500 requests per second the machine is saturated (the load generator, the API and PostgreSQL share four cores), so latency rises with concurrency while throughput stays flat. A real deployment separates them.

## What measuring found (and fixed)

1. **Every client list was a full table scan.** The row-level-security policies read "staff, or `tenant_id` is mine". PostgreSQL cannot use an index through that `OR`, so listing a client's tickets read the whole table: about 10 ms of database CPU per request at 15,000 rows, growing linearly. Client list queries now carry a redundant `tenant_id = mine` predicate (`Scope.TenantFilter`); security still rests on the policy, the predicate only lets the planner use the index. Client ticket list: **367 to 4,122 requests per second** on a table twice as large. A test (`plan_test.go`) keeps this true.
2. **The database pool defaulted to 4 connections.** It is now 25 per instance (`HEKATE_DB_MAX_CONNS`, or `pool_max_conns` in the URL). Instances times this value must stay below PostgreSQL's `max_connections`. (Raising it alone did not change this machine's throughput, which was CPU bound; it matters when the database is on another host.)
3. **The built-in rate limit worked as designed** and masked everything until it was made configurable (`HEKATE_RATE_LIMIT`).

## Still to do

- Only the client-facing list endpoints for tickets, quotes, invoices, projects, interventions and API keys have the tenant predicate. Staff list queries (which see every tenant) still scan with `LIMIT`; as tables grow they need keyset pagination and indexes on the sort columns. Not measured yet.
- Test the heavy paths: PDF generation, payroll runs for hundreds of employees, ledger posting under concurrency (it serialises writers by design), trial balance over a year of entries.
- Repeat on separate hosts with TLS and a real identity provider, with production-sized data, and soak for hours.
- Do not quote the numbers above to a customer as capacity.
