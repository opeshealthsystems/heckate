# Operations runbook

For the person on call. Each alert in `deploy/monitoring/alerts.yml` links to a section here.

## What to watch

| Signal | Where | Healthy |
| --- | --- | --- |
| Liveness | `GET /livez` | 200, never touches the database. Use for "restart me" probes |
| Readiness | `GET /readyz` (alias `/healthz`) | 200 within 2 s. Use for "send me traffic" probes |
| Metrics | `HEKATE_METRICS_ADDR` (internal network only), `/metrics` | see below |
| Recurring invoices | log line `recurring invoices: drafts created` (actor `system:recurring-invoices` in the audit log) | `HEKATE_RECURRING_INTERVAL` (default `1h`, minimum `1m`, `off` to disable). Creates DRAFTS only; finance issues them. Safe in every instance |
| Request log | stdout, one line per request: `trace_id`, `route`, `dur` | quote the trace id from any error response (`X-Trace-Id`) to find the request |

Metrics that matter: `hekate_http_requests_total{class="5xx"}`, `hekate_http_request_duration_seconds`, `hekate_db_up`, `hekate_db_pool_connections`, `hekate_outbox_pending` and `hekate_outbox_oldest_pending_seconds` (events not yet published; should hover near 0), `hekate_audit_unsealed` (audit rows not yet in the hash chain; should stay small).

## API down

1. `curl -fsS https://api.../livez` from outside. No answer: the instance or the load balancer is down; restart the container, check its logs for `unsafe configuration` (the server refuses to start with unsafe settings and says which).
2. Answer but `/readyz` fails: see *Database unreachable*.
3. A new release just went out: roll back to the previous image first, investigate second.

## Database unreachable

`hekate_db_up == 0`, `/readyz` returns 503. The API keeps answering `/livez` on purpose, so it is not restarted pointlessly.

1. Check the PostgreSQL host, its disk space (a full disk makes writes fail first) and connection count.
2. Credentials or TLS recently changed? The connection string must keep `sslmode=verify-full` (or `require`); the API refuses `disable` in production.
3. The API recovers by itself when the database returns; no restart is needed.

## High error rate

1. `hekate_http_requests_total{class="5xx"}` by `route` shows which endpoint.
2. Take a `trace_id` from a failing response and find its log line; error bodies never contain internals by design, the log does.
3. Deferred checks (unbalanced ledger entry, negative stock) return 422 / 409, not 5xx. A 5xx is a bug or an infrastructure fault: file it with the trace id.

## Slow requests

1. Which route? (`hekate_http_request_duration_seconds` by `route`.)
2. `hekate_db_pool_connections{state="acquired"}` at the maximum means requests wait for a connection: look for a long transaction in PostgreSQL (`pg_stat_activity`), then consider more connections or instances.
3. Ledger posting serialises writers by design (one hash chain). A burst of postings queues briefly; sustained saturation needs the posting path reviewed, not just more hardware.

## Outbox stuck

Events (push notifications, live updates, the message broker) are written with the data and relayed afterwards. `hekate_outbox_oldest_pending_seconds` rising means the relay is failing; **no data is lost**, delivery is late.

1. Look for `outbox relay` errors in the log. NATS unreachable is the usual cause (`HEKATE_NATS_URL`).
2. When the broker returns, the backlog is published in order. Subscribers de-duplicate on the event id.

## Audit not sealing

`hekate_audit_unsealed` climbing means the background sealer is failing. Audit rows are still written and immutable; only the hash chain lags.

1. Look for `audit sealer` errors in the log.
2. `GET /v1/audit/verify` (management or auditor) reports `waiting_to_be_sealed`.
3. If the chain reports `intact: false`, **treat it as a security incident**: stop, preserve the database, do not "repair" rows. The first broken `chain_seq` is in the response. See *Suspected tampering*.

## Suspected tampering

1. Do not modify the database. Take a snapshot.
2. `GET /v1/accounting/verify` and `GET /v1/audit/verify` name the first bad entry. `api verify-data` produces the same report offline.
3. Restore the last verified backup into a **separate** database (below) and compare.
4. Escalate to the company owner. If client data may be affected, contractual and legal notification clocks may already be running.

## Credential or key compromise

| What | Do |
| --- | --- |
| A client API key | `DELETE /v1/api-keys/{id}` (immediate); the audit trail shows its use |
| A user's session | Disable the user in Keycloak; the token expires within its lifetime (set it short: 5 to 10 minutes) |
| `HEKATE_PAYMENTS_WEBHOOK_SECRET` | Rotate it in the environment and at the provider; callbacks with the old signature are refused |
| Database password | Change it, update `HEKATE_DATABASE_URL`, restart (rolling) |
| Signing keys of the identity provider | Rotate in Keycloak; the API re-reads the key set within an hour or on an unknown key id |

## Backups and restore

Scripts are in `deploy/backup/`. They were run against a database with real ledger and audit rows (see the CI job "Backup and restore drill"), including encryption, a tampered file, a wrong key, and a non-empty target.

```
# nightly (cron or a scheduled job)
HEKATE_DATABASE_URL=... BACKUP_DIR=/backups BACKUP_AGE_RECIPIENT=age1... deploy/backup/backup.sh

# restore into a new, empty database (never over the live one)
createdb hekate_restored
BACKUP_AGE_IDENTITY=/secure/key.txt deploy/backup/restore.sh /backups/hekate-<stamp>.dump.age postgres://.../hekate_restored

# prove the restore is whole (hash chains, every table's row count)
HEKATE_DATABASE_URL=postgres://.../hekate_restored api verify-data

# monthly: the whole cycle, with timings
HEKATE_DATABASE_URL=... DRILL_ADMIN_URL=... deploy/backup/drill.sh
```

Keep the age private key **offline and separate** from the backups. A backup without the key is unreadable, by design.

## What is untested (be honest with auditors)

- The alert rules have not been loaded into a real Prometheus.
- Restore time is measured only on a small database. Run the drill on production-sized data before quoting a recovery time.
- Point-in-time recovery needs WAL archiving or a replica configured on the database server; the logical backups here recover to the moment of the backup only. See `DR.md`.
- No pager or on-call rota exists yet; the alerts name severity `page` or `ticket` for when one does.
