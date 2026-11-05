#!/usr/bin/env bash
# Logical backup of the HEKATE database: consistent snapshot, checksum, optional encryption, retention.
#   HEKATE_DATABASE_URL    connection string of the database to back up (required)
#   BACKUP_DIR             where backups go (default ./backups)
#   BACKUP_KEEP            how many backups to keep (default 14)
#   BACKUP_AGE_RECIPIENT   age public key: if set the dump is encrypted and the plaintext removed.
#                          Without it the backup is written UNENCRYPTED and a warning is printed.
set -euo pipefail
: "${HEKATE_DATABASE_URL:?set HEKATE_DATABASE_URL}"
dir="${BACKUP_DIR:-./backups}"; keep="${BACKUP_KEEP:-14}"
mkdir -p "$dir"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
file="$dir/hekate-$stamp.dump"

pg_dump --format=custom --no-owner --compress=6 --file="$file.partial" "$HEKATE_DATABASE_URL"
mv "$file.partial" "$file"

if [[ -n "${BACKUP_AGE_RECIPIENT:-}" ]]; then
  age -r "$BACKUP_AGE_RECIPIENT" -o "$file.age" "$file"
  rm -f "$file"
  file="$file.age"
else
  echo "WARNING: this backup is not encrypted (set BACKUP_AGE_RECIPIENT). Store it only on encrypted media." >&2
fi
( cd "$dir" && sha256sum "$(basename "$file")" > "$(basename "$file").sha256" )

# Retention: the newest $keep survive.
shopt -s nullglob
backups=("$dir"/hekate-*.dump "$dir"/hekate-*.dump.age)
if (( ${#backups[@]} > keep )); then
  # names carry a sortable UTC timestamp, so lexical order is age order
  mapfile -t sorted < <(printf '%s\n' "${backups[@]}" | sort -r)
  for old in "${sorted[@]:keep}"; do rm -f "$old" "$old.sha256"; done
fi
echo "$file"
