# Shared helpers for backup.sh / restore.sh / backup-test.sh (sourced, not executed).
#
# Two modes:
#   compose (default): run pg_dump/pg_restore/psql inside the compose `postgres` service (no client tools on the host,
#                      matching server major version guaranteed);
#   local:             PG_MODE=local uses the host's pg_dump/pg_restore/psql with PGHOST/PGPORT/PGUSER/PGPASSWORD.
# Never echo connection strings or passwords.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PG_MODE="${PG_MODE:-compose}"
PGUSER="${PGUSER:-tiendao}"
PGDATABASE="${PGDATABASE:-tiendao}"

if [[ -z "${COMPOSE:-}" ]]; then
  COMPOSE="docker compose -f ${REPO_ROOT}/deployments/docker-compose.yml"
  [[ -f "${REPO_ROOT}/.env" ]] && COMPOSE="docker compose --env-file ${REPO_ROOT}/.env -f ${REPO_ROOT}/deployments/docker-compose.yml"
fi

# pg_tool <tool> [args...] — runs a PostgreSQL client tool; stdin/stdout are passed through.
pg_tool() {
  local tool="$1"; shift
  if [[ "$PG_MODE" == "local" ]]; then
    "$tool" "$@"
  else
    $COMPOSE exec -T postgres "$tool" -U "$PGUSER" "$@"
  fi
}

# psql_q <db> <sql> — one query, unaligned tuples only.
psql_q() {
  local db="$1" sql="$2"
  if [[ "$PG_MODE" == "local" ]]; then
    psql -X -v ON_ERROR_STOP=1 -At -d "$db" -c "$sql"
  else
    $COMPOSE exec -T postgres psql -X -v ON_ERROR_STOP=1 -U "$PGUSER" -At -d "$db" -c "$sql"
  fi
}

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*" >&2; }
die() { log "ERROR: $*"; exit 1; }
