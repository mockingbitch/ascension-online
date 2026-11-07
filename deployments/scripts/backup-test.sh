#!/usr/bin/env bash
# Backup/restore drill (make backup-test): dump the dev DB with backup.sh, restore it into a scratch DB with
# restore.sh, compare schema_migrations exactly, the set of tables, and the row counts of key tables, then drop the
# scratch DB. Exit 1 on any mismatch.
#
# The source keeps serving while it is dumped, so a key table passes when its restored count lies between the counts
# read just before and just after the dump (pg_dump reads one consistent snapshot taken in between).
source "$(dirname "${BASH_SOURCE[0]}")/_pg.sh"

src="$PGDATABASE"
scratch="${SCRATCH_DB:-${src}_restore_test}"
[[ "$scratch" != "$src" ]] || die "SCRATCH_DB must differ from the source DB"
work="$(mktemp -d)"
cleanup() {
  rm -rf "$work"
  psql_q postgres "DROP DATABASE IF EXISTS \"${scratch}\" WITH (FORCE)" >/dev/null 2>&1 || true
}
trap cleanup EXIT

KEY_TABLES=(users user_identities characters character_progressions cultivation_settlements breakthrough_attempts
  currency_accounts currency_ledger inventory_items inventory_transactions vip_accounts vip_point_ledger outbox_events
  admin_audit_logs world_state world_daily_totals chat_messages character_faction payment_orders dao_lu_bonds)

existing() { # tables of KEY_TABLES present in db $1
  local list
  list="$(printf "'%s'," "${KEY_TABLES[@]}")"
  psql_q "$1" "SELECT table_name FROM information_schema.tables WHERE table_schema='public'
    AND table_name IN (${list%,}) ORDER BY 1"
}
counts() { # "table count" lines for db $1 and table list $2
  local db="$1" sql="" t
  for t in $2; do sql+="SELECT '${t}', count(*) FROM \"${t}\" UNION ALL "; done
  psql_q "$db" "${sql% UNION ALL }" | tr '|' ' '
}

tables="$(existing "$src")"
before="$(counts "$src" "$tables")"
dump="$(BACKUP_DIR="$work" BACKUP_RETENTION=1 "$(dirname "$0")/backup.sh" | tail -n 1)"
after="$(counts "$src" "$tables")"
RESTORE_CONFIRM="restore ${scratch}" TARGET_DB="$scratch" "$(dirname "$0")/restore.sh" "$dump"

fail=0
check() { if [[ "$2" == "$3" ]]; then printf '  ok    %-28s %s\n' "$1" "$2"; else printf '  FAIL  %-28s source=%s restored=%s\n' "$1" "$2" "$3"; fail=1; fi; }

echo "backup-test: ${src} → $(basename "$dump") → ${scratch}"
check schema_migrations "$(psql_q "$src" 'SELECT version, dirty FROM schema_migrations')" \
  "$(psql_q "$scratch" 'SELECT version, dirty FROM schema_migrations')"
tq="SELECT count(*) FROM information_schema.tables WHERE table_schema='public'"
check "public tables" "$(psql_q "$src" "$tq")" "$(psql_q "$scratch" "$tq")"
rq="SELECT count(*) FROM pg_indexes WHERE schemaname='public'"
check "public indexes" "$(psql_q "$src" "$rq")" "$(psql_q "$scratch" "$rq")"

restored="$(counts "$scratch" "$tables")"
while read -r t b; do
  a="$(awk -v t="$t" '$1==t{print $2}' <<<"$after")"
  r="$(awk -v t="$t" '$1==t{print $2}' <<<"$restored")"
  lo=$(( b < a ? b : a )); hi=$(( b > a ? b : a ))
  if [[ -n "$r" && "$r" -ge "$lo" && "$r" -le "$hi" ]]; then
    printf '  ok    %-28s %s\n' "$t" "$r"
  else
    printf '  FAIL  %-28s source=%s..%s restored=%s\n' "$t" "$lo" "$hi" "${r:-missing}"; fail=1
  fi
done <<<"$before"

if [[ "$fail" -ne 0 ]]; then
  echo "backup-test: FAILED"; exit 1
fi
echo "backup-test: PASSED ($(du -h "$dump" | cut -f1) dump)"
