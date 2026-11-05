#!/usr/bin/env bash
# Restore a backup into an EMPTY database, after checking its checksum.
#   usage: restore.sh <backup file> <target database url>
#   BACKUP_AGE_IDENTITY   path to the age private key (for .age files)
# The target database must already exist and be empty; this script never drops anything.
set -euo pipefail
file="${1:?backup file}"; target="${2:?target database url}"

( cd "$(dirname "$file")" && sha256sum --check --quiet "$(basename "$file").sha256" ) || { echo "checksum mismatch: refusing to restore" >&2; exit 3; }

src="$file"
if [[ "$file" == *.age ]]; then
  : "${BACKUP_AGE_IDENTITY:?set BACKUP_AGE_IDENTITY to decrypt}"
  src="$(mktemp)"; trap 'rm -f "$src"' EXIT
  age -d -i "$BACKUP_AGE_IDENTITY" -o "$src" "$file"
fi

existing="$(psql "$target" -Atc "select count(*) from information_schema.tables where table_schema not in ('pg_catalog','information_schema')")"
[[ "$existing" == "0" ]] || { echo "target database is not empty ($existing tables): refusing" >&2; exit 4; }

# The application role is cluster-wide, so a fresh server may not have it yet.
psql "$target" -v ON_ERROR_STOP=1 -q -c "do \$\$ begin if not exists (select 1 from pg_roles where rolname='hekate_app') then create role hekate_app nologin; end if; end \$\$"
pg_restore --no-owner --exit-on-error --dbname="$target" "$src"
echo "restored $file"
