#!/usr/bin/env bash
# Run as the unprivileged postgres user in a dedicated disposable container.
set -euo pipefail
unset PGDATABASE PGUSER PGSERVICE PGSERVICEFILE PGOPTIONS PGPASSWORD PGPASSFILE
scratch=$(mktemp -d)
source_dir="$scratch/source"
restore_dir="$scratch/restored"
archive="$scratch/archive"
export PGHOST="$scratch/socket" PGPORT=5432 PGUSER=postgres PGDATABASE=postgres
mkdir -m 0700 "$PGHOST" "$archive"

cleanup() {
  local status=$?
  if (( status != 0 )); then
    for log in "$scratch/source.log" "$scratch/restore.log"; do
      if [[ -f "$log" ]]; then tail -40 "$log" >&2; fi
    done
  fi
  pg_ctl -D "$source_dir" -m immediate -w stop >/dev/null 2>&1 || true
  pg_ctl -D "$restore_dir" -m immediate -w stop >/dev/null 2>&1 || true
  rm -rf -- "$scratch"
}
trap cleanup EXIT
export PGCONNECT_TIMEOUT=3 PGOPTIONS='-c statement_timeout=15000'

initdb -D "$source_dir" --username=postgres --auth-local=trust --auth-host=reject >/dev/null
cat >> "$source_dir/postgresql.conf" <<CONFIG
listen_addresses = ''
unix_socket_directories = '$PGHOST'
wal_level = replica
archive_mode = on
archive_command = 'test ! -f "$archive/%f" && cp "%p" "$archive/%f"'
CONFIG
pg_ctl -D "$source_dir" -l "$scratch/source.log" -w -t 20 start >/dev/null

query() {
  psql -X -A -t -q -v ON_ERROR_STOP=1 -c "$1"
}
query "CREATE TABLE synthetic_recovery_marker (state text PRIMARY KEY)"
query "INSERT INTO synthetic_recovery_marker VALUES ('before-base-backup')"
pg_basebackup -w -D "$restore_dir" -X stream
query "INSERT INTO synthetic_recovery_marker VALUES ('after-base-backup')"
target_segment=$(query "SELECT pg_walfile_name(pg_create_restore_point('synthetic-before-deletion'))")
query "DELETE FROM synthetic_recovery_marker; INSERT INTO synthetic_recovery_marker VALUES ('after-recovery-point')"
query "SELECT pg_switch_wal()" >/dev/null
for attempt in {1..20}; do
  if [[ -s "$archive/$target_segment" ]]; then break; fi
  sleep 1
done
test -s "$archive/$target_segment"
test "$(query 'SELECT state FROM synthetic_recovery_marker')" = 'after-recovery-point'
pg_ctl -D "$source_dir" -w -t 20 stop >/dev/null

cat >> "$restore_dir/postgresql.conf" <<CONFIG
archive_mode = off
restore_command = 'cp "$archive/%f" "%p"'
recovery_target_name = 'synthetic-before-deletion'
recovery_target_action = 'promote'
CONFIG
touch "$restore_dir/recovery.signal"
pg_ctl -D "$restore_dir" -l "$scratch/restore.log" -w -t 20 start >/dev/null
for attempt in {1..20}; do
  if result=$(query 'SELECT pg_is_in_recovery()') && [[ "$result" == f ]]; then break; fi
  sleep 1
done
test "$(query 'SELECT pg_is_in_recovery()')" = f
test "$(query 'SELECT state FROM synthetic_recovery_marker ORDER BY state')" = $'after-base-backup\nbefore-base-backup'
pg_ctl -D "$restore_dir" -w -t 20 stop >/dev/null
echo 'Recovered archived PostgreSQL changes to the named point.'
