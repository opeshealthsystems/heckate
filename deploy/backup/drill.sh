#!/usr/bin/env bash
# Restore drill: prove that a backup can actually be restored, and measure how long it takes.
# Backs up the source database, restores it into a scratch database, runs the platform's integrity check
# on both (ledger chain, audit chain, every table's row count) and requires the two reports to match.
# Run it against a quiet database or a replica: writes during the drill make the counts differ.
#   HEKATE_DATABASE_URL   source (required)
#   DRILL_ADMIN_URL       a URL that may CREATE/DROP DATABASE on the same server (required)
#   HEKATE_API            path to the api binary (default ./api)
set -euo pipefail
: "${HEKATE_DATABASE_URL:?}"; : "${DRILL_ADMIN_URL:?}"
api="${HEKATE_API:-./api}"; here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"; scratch="hekate_drill_$(date -u +%s)"
trap 'psql "$DRILL_ADMIN_URL" -qc "drop database if exists $scratch" >/dev/null 2>&1 || true; rm -rf "$work"' EXIT

echo "1/5 integrity of the source"
HEKATE_DATABASE_URL="$HEKATE_DATABASE_URL" "$api" verify-data > "$work/source.json"

echo "2/5 backup"
t0=$(date +%s)
file="$(BACKUP_DIR="$work" BACKUP_KEEP=3 "$here/backup.sh" 2>/dev/null)"
t1=$(date +%s)

echo "3/5 restore into $scratch"
psql "$DRILL_ADMIN_URL" -qc "create database $scratch"
target="$(python3 - "$HEKATE_DATABASE_URL" "$scratch" <<'PY'
import sys, urllib.parse as u
p = u.urlparse(sys.argv[1]); print(u.urlunparse(p._replace(path="/" + sys.argv[2])))
PY
)"
"$here/restore.sh" "$file" "$target" >/dev/null
t2=$(date +%s)

echo "4/5 integrity of the restored copy"
HEKATE_DATABASE_URL="$target" "$api" verify-data > "$work/restored.json"

echo "5/5 compare"
if diff -u "$work/source.json" "$work/restored.json"; then
  echo "DRILL PASSED: backup took $((t1 - t0))s, restore took $((t2 - t1))s, $(du -h "$file" | cut -f1) on disk"
else
  echo "DRILL FAILED: the restored copy differs from the source" >&2; exit 1
fi
