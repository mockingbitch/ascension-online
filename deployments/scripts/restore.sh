#!/usr/bin/env bash
# Restore a backup.sh dump into a database (recreated from scratch).
#
#   deployments/scripts/restore.sh backups/tiendao_20261107T101500Z.dump            # into PGDATABASE (tiendao)
#   TARGET_DB=tiendao_restore deployments/scripts/restore.sh <file>                  # into a scratch DB
#
# Guard rails: refuses APP_ENV=prod unless ALLOW_PROD=1; asks to type "restore <db>" unless RESTORE_CONFIRM="restore <db>".
# Restoring the LIVE database: stop api + worker first (the DB is dropped WITH (FORCE)), then follow the runbook:
# REBUILD_REDIS=1 flushes Redis (cache only) and restarts the worker, whose startup reconcile rebuilds the leaderboard
# and faction rankings from Postgres (see deployments/README.md).
source "$(dirname "${BASH_SOURCE[0]}")/_pg.sh"

file="${1:-}"
[[ -n "$file" && -f "$file" ]] || die "usage: restore.sh <dump file>   (got '${file}')"
target="${TARGET_DB:-$PGDATABASE}"
[[ "$target" =~ ^[a-z_][a-z0-9_]{0,62}$ ]] || die "TARGET_DB must be a plain identifier"
if [[ "${APP_ENV:-}" == "prod" && "${ALLOW_PROD:-}" != "1" ]]; then
  die "APP_ENV=prod: set ALLOW_PROD=1 to restore"
fi

want="restore ${target}"
given="${RESTORE_CONFIRM:-}"
if [[ -z "$given" ]]; then
  printf 'This DROPS database %s and restores %s.\nType "%s" to continue: ' "$target" "$(basename "$file")" "$want" >&2
  read -r given
fi
[[ "$given" == "$want" ]] || die "confirmation mismatch"

log "recreating ${target}"
psql_q postgres "DROP DATABASE IF EXISTS \"${target}\" WITH (FORCE)" >/dev/null
psql_q postgres "CREATE DATABASE \"${target}\"" >/dev/null

log "restoring $(basename "$file") → ${target}"
pg_tool pg_restore -d "$target" --no-owner --no-privileges --exit-on-error <"$file"

version="$(psql_q "$target" "SELECT version || CASE WHEN dirty THEN ' (DIRTY)' ELSE '' END FROM schema_migrations")"
log "ok: ${target} at migration ${version}"

if [[ "${REBUILD_REDIS:-}" == "1" ]]; then
  log "flushing Redis (rebuildable cache) and restarting worker (startup reconcile rebuilds rankings)"
  $COMPOSE exec -T redis redis-cli FLUSHDB >/dev/null
  $COMPOSE restart worker >/dev/null
  log "redis rebuilt by worker reconcile; restart api to reset in-process state: $COMPOSE restart api"
fi
