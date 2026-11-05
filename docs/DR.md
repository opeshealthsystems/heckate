# Disaster recovery

**Status: logical backups and a tested restore procedure exist; recovery objectives are proposed, not yet agreed or measured at production size.**

## Objectives (proposed, for the owner to confirm)

| | Target | What delivers it today |
| --- | --- | --- |
| RPO (data you can lose) | 15 minutes | **Not met yet.** Logical backups give an RPO equal to the backup interval (nightly = up to 24 h). Reaching 15 minutes needs WAL archiving or a streaming replica on the database host (infrastructure work) |
| RTO (time to be back) | 4 hours | Restore is scripted and tested; measure with `deploy/backup/drill.sh` on production-sized data and update this number with the result |

## Scenarios

| Scenario | Recovery |
| --- | --- |
| API container fails | Orchestrator restarts it (`/livez`); stateless, nothing to recover |
| Database host fails | Fail over to the replica if one exists; otherwise restore the latest backup into a new server and repoint `HEKATE_DATABASE_URL` |
| Data corrupted or deleted by mistake | Restore into a **separate** database from before the event; copy the needed rows back with a reviewed script. Never restore over the live database |
| Tampering suspected | Preserve the database, restore the last verified backup beside it, compare with `api verify-data` (see `RUNBOOK.md`) |
| Site lost | Backups must be copied off-site (a second location and provider) and encrypted; **no off-site copy is configured yet** |
| Backup key lost | Backups encrypted with `age` cannot be recovered. Keep the private key in two separate secure places |

## What must exist outside this repository

1. A second location for backups, encrypted, with its own access control.
2. WAL archiving or a replica for the 15 minute target.
3. A quarterly restore drill run by someone other than the person who wrote the scripts, with the result recorded here.
4. Named people for the on-call and escalation roles in `RUNBOOK.md`.
