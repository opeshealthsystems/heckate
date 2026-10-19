# Platform audit

A running record of what was checked, what was found, what was fixed and what remains. Each finding has an ID, a severity (our own judgement, assuming real client data), and a status. Fixes name the commit's test, so each one can be re-checked.

Status key: **Fixed** (change plus a test), **By design** (explained), **Open** (needs a decision or a person).

## 1. Data layer (68 tables, 29 migrations)

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
