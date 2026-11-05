#!/usr/bin/env bash
# Exercises the backup scripts' safety properties against a real database (needs psql, pg_dump, age).
#   HEKATE_DATABASE_URL  a migrated database to back up      DRILL_ADMIN_URL  may create/drop databases
#   HEKATE_API           the api binary
set -uo pipefail
: "${HEKATE_DATABASE_URL:?}"; : "${DRILL_ADMIN_URL:?}"; api="${HEKATE_API:-./api}"; here="$(cd "$(dirname "$0")" && pwd)"
fail=0; ok() { echo "PASS $1"; }; bad() { echo "FAIL $1"; fail=1; }
work="$(mktemp -d)"; made=()
cleanup() { for d in "${made[@]}"; do psql "$DRILL_ADMIN_URL" -qc "drop database if exists $d" >/dev/null 2>&1; done; rm -rf "$work"; }
trap cleanup EXIT
newdb() { local n="hekate_bt_$RANDOM$RANDOM"; psql "$DRILL_ADMIN_URL" -qc "create database $n" && made+=("$n"); echo "$n"; }
url() { python3 -c "import sys,urllib.parse as u; p=u.urlparse(sys.argv[1]); print(u.urlunparse(p._replace(path='/'+sys.argv[2])))" "$HEKATE_DATABASE_URL" "$1"; }

# 1. a plain backup restores and matches
f="$(BACKUP_DIR="$work" "$here/backup.sh" 2>/dev/null)"; d="$(newdb)"
"$here/restore.sh" "$f" "$(url "$d")" >/dev/null && ok "plain backup restores" || bad "plain backup restores"
HEKATE_DATABASE_URL="$HEKATE_DATABASE_URL" "$api" verify-data > "$work/a.json"; HEKATE_DATABASE_URL="$(url "$d")" "$api" verify-data > "$work/b.json"
diff -q "$work/a.json" "$work/b.json" >/dev/null && ok "restored copy matches the source" || bad "restored copy matches the source"

# 2. restoring into a database that already has data is refused
"$here/restore.sh" "$f" "$(url "$d")" >/dev/null 2>&1; [[ $? == 4 ]] && ok "non-empty target refused" || bad "non-empty target refused"

# 3. a damaged backup is refused before anything is touched
cp "$f" "$work/damaged.dump"; cp "$f.sha256" "$work/damaged.dump.sha256"; sed -i 's/hekate-[^ ]*\.dump/damaged.dump/' "$work/damaged.dump.sha256"
printf 'x' >> "$work/damaged.dump"; d2="$(newdb)"
"$here/restore.sh" "$work/damaged.dump" "$(url "$d2")" >/dev/null 2>&1; [[ $? == 3 ]] && ok "tampered backup refused (checksum)" || bad "tampered backup refused (checksum)"

# 4. encrypted backups: not readable without the key, restorable with it
age-keygen -o "$work/key.txt" 2>/dev/null; pub="$(grep -o 'age1[a-z0-9]*' "$work/key.txt" | head -1)"
ef="$(BACKUP_DIR="$work/enc" BACKUP_AGE_RECIPIENT="$pub" "$here/backup.sh" 2>/dev/null)"
[[ "$ef" == *.age && ! -e "${ef%.age}" ]] && ok "encrypted backup leaves no plaintext" || bad "encrypted backup leaves no plaintext"
! grep -aq "PGDMP" "$ef" && ok "encrypted file is not a readable dump" || bad "encrypted file is not a readable dump"
d3="$(newdb)"; BACKUP_AGE_IDENTITY="$work/key.txt" "$here/restore.sh" "$ef" "$(url "$d3")" >/dev/null && ok "encrypted backup restores with the key" || bad "encrypted backup restores with the key"
d4="$(newdb)"; age-keygen -o "$work/other.txt" 2>/dev/null; BACKUP_AGE_IDENTITY="$work/other.txt" "$here/restore.sh" "$ef" "$(url "$d4")" >/dev/null 2>&1 && bad "wrong key must fail" || ok "wrong key fails"

# 5. retention keeps only the newest N
for i in 1 2 3 4; do sleep 1; BACKUP_DIR="$work/ret" BACKUP_KEEP=2 "$here/backup.sh" >/dev/null 2>&1; done
[[ "$(ls "$work"/ret/*.dump | wc -l)" == 2 ]] && ok "retention keeps the newest 2" || bad "retention keeps the newest 2"

exit $fail
