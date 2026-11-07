#!/usr/bin/env bash
# Postgres backup: pg_dump custom format (-Fc, compressed, restorable table by table), timestamped, verified with
# pg_restore --list, then retention keeps the newest BACKUP_RETENTION dumps.
#
#   deployments/scripts/backup.sh                 # → backups/tiendao_20261107T101500Z.dump
#   BACKUP_DIR=/srv/backups BACKUP_RETENTION=30 deployments/scripts/backup.sh
#
# Env: BACKUP_DIR (default <repo>/backups), BACKUP_RETENTION (default 14), PGDATABASE (tiendao), PGUSER (tiendao),
#      PG_MODE (compose|local, see _pg.sh). Prints the dump path on stdout (last line).
# Redis is not backed up: it only holds rebuildable data (see deployments/README.md, restore runbook).
source "$(dirname "${BASH_SOURCE[0]}")/_pg.sh"

BACKUP_DIR="${BACKUP_DIR:-${REPO_ROOT}/backups}"
BACKUP_RETENTION="${BACKUP_RETENTION:-14}"
[[ "$BACKUP_RETENTION" =~ ^[1-9][0-9]*$ ]] || die "BACKUP_RETENTION must be a positive integer"
mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR" 2>/dev/null || true

ts="$(date -u +%Y%m%dT%H%M%SZ)"
out="${BACKUP_DIR}/${PGDATABASE}_${ts}.dump"
tmp="${out}.partial"
trap 'rm -f "$tmp"' EXIT

log "dumping ${PGDATABASE} (${PG_MODE})"
# --no-owner/--no-privileges: restorable into a scratch DB or another role; the schema itself is fully included.
pg_tool pg_dump -d "$PGDATABASE" -Fc --no-owner --no-privileges >"$tmp"
[[ -s "$tmp" ]] || die "empty dump"

# Verify the archive is readable (TOC parse) before it replaces anything.
entries="$(pg_tool pg_restore --list <"$tmp" | grep -c ' TABLE DATA ' || true)"
[[ "$entries" -gt 0 ]] || die "dump has no TABLE DATA entries"
mv "$tmp" "$out"
chmod 600 "$out" 2>/dev/null || true
log "ok: $(du -h "$out" | cut -f1) ${entries} tables → ${out}"

# Retention: newest N dumps of this database survive.
ls -1t "${BACKUP_DIR}/${PGDATABASE}"_*.dump 2>/dev/null | tail -n +"$((BACKUP_RETENTION + 1))" | while IFS= read -r f; do
  rm -f -- "$f" && log "retention: removed $(basename "$f")"
done
echo "$out"
