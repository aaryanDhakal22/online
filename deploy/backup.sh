#!/usr/bin/env bash
# backup.sh — nightly snapshot of online's SQLite (the vendor API key table).
#
# Losing this file means losing the live vendor key: every online order would
# be refused until the vendor and the owner agree a new one. So it is copied
# with SQLite's online backup API (consistent while the api is writing, WAL
# included), checked with PRAGMA integrity_check, and the newest $KEEP copies
# are kept. The copies hold the key plaintext: the directory is 0700, files
# 0600, root-owned.
#
# Usage:   sudo ./backup.sh
# Env:     ONLINE_DB          the database file (default: app.db in the
#                             docker volume $ONLINE_VOLUME)
#          ONLINE_VOLUME      default quicc-online-data (deploy/docker-compose.staging.yml)
#          ONLINE_BACKUP_DIR  default /var/backups/quicc-online
#          ONLINE_BACKUP_KEEP default 14
# Timer:   deploy/systemd/quicc-online-backup.{service,timer} (03:30 daily);
#          restore steps in deploy/README.md.
set -euo pipefail

VOLUME="${ONLINE_VOLUME:-quicc-online-data}"
DEST="${ONLINE_BACKUP_DIR:-/var/backups/quicc-online}"
KEEP="${ONLINE_BACKUP_KEEP:-14}"

die() { echo "backup: $*" >&2; exit 1; }

[[ "$KEEP" =~ ^[0-9]+$ && "$KEEP" -ge 1 ]] || die "ONLINE_BACKUP_KEEP must be a positive integer"

DB="${ONLINE_DB:-}"
if [[ -z "$DB" ]]; then
  mp=$(docker volume inspect -f '{{ .Mountpoint }}' "$VOLUME" 2>/dev/null) \
    || die "no ONLINE_DB given and docker volume '$VOLUME' not found"
  DB="$mp/app.db"
fi
[[ -f "$DB" ]] || die "database not found: $DB"

# sqlite3 on the host if present, else a throwaway alpine container (the api
# image is distroless — no sqlite3 inside).
sql() { # sql <dbfile> <statement>
  if command -v sqlite3 >/dev/null 2>&1; then
    sqlite3 -bail "$1" "$2"
  else
    local dir base
    dir=$(cd "$(dirname "$1")" && pwd); base=$(basename "$1")
    docker run --rm -v "$dir:/w" alpine:3.20 sh -c \
      'apk add --no-cache -q sqlite >/dev/null && sqlite3 -bail "/w/$0" "$1"' "$base" "$2"
  fi
}

umask 077
mkdir -p "$DEST"
chmod 700 "$DEST"

ts=$(date -u +%Y%m%dT%H%M%SZ)
out="$DEST/online-$ts.db"
tmp="$DEST/.online-$ts.db.partial"
trap 'rm -f "$tmp"' EXIT

if command -v sqlite3 >/dev/null 2>&1; then
  sqlite3 -bail "$DB" ".backup '$tmp'"
else
  # Back up next to the source inside the container, then move it out.
  dbdir=$(cd "$(dirname "$DB")" && pwd)
  docker run --rm -v "$dbdir:/w" -v "$DEST:/out" alpine:3.20 sh -c \
    'apk add --no-cache -q sqlite >/dev/null && sqlite3 -bail "/w/$0" ".backup /out/$1"' \
    "$(basename "$DB")" "$(basename "$tmp")"
fi

check=$(sql "$tmp" "PRAGMA integrity_check;")
[[ "$check" == "ok" ]] || die "integrity_check failed on the copy: $check"

active=$(sql "$tmp" "SELECT COUNT(*) FROM api_keys WHERE status = 'active';")
mv -f "$tmp" "$out"
chmod 600 "$out"
trap - EXIT
echo "backup: $out (integrity ok, active keys: $active)"
if [[ "$active" -lt 1 ]]; then
  echo "backup: WARNING — the database has NO active vendor key" >&2
fi

# Keep the newest $KEEP (names are UTC timestamps, so they sort by age).
mapfile -t old < <(ls -1 "$DEST"/online-*.db 2>/dev/null | sort -r | tail -n +"$((KEEP + 1))")
if (( ${#old[@]} )); then
  rm -f -- "${old[@]}"
  echo "backup: pruned ${#old[@]} old copies (keep $KEEP)"
fi
