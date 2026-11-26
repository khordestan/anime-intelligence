#!/usr/bin/env bash
# Verify the supplied images against a disposable PostgreSQL cluster.
set -euo pipefail
if [[ $# != 2 ]]; then
  echo 'usage: check-bundle-migrations.sh <migrator-image> <api-image>' >&2
  exit 1
fi
repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
database=
api=
cleanup() {
  if [[ -n "$api" ]]; then docker rm -f "$api" >/dev/null 2>&1 || true; fi
  if [[ -n "$database" ]]; then docker rm -f "$database" >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT

migrate_image=$1
api_image=$2
test "$(docker image inspect --format '{{.Config.User}}' "$migrate_image")" = migrate
database=$(docker run --network none -d \
  -e POSTGRES_PASSWORD=synthetic-bundle-admin-password postgres:17)
for ((i=0; i<30; i++)); do
  if docker exec "$database" pg_isready -q -h 127.0.0.1 -U postgres; then break; fi
  sleep 1
done
docker exec "$database" pg_isready -q -h 127.0.0.1 -U postgres

migrate() {
  docker run --rm --network "container:$database" \
    -e 'MIGRATION_DATABASE_URL=postgres://postgres:synthetic-bundle-admin-password@127.0.0.1:5432/postgres?sslmode=disable' \
    "$migrate_image"
}
ledger() {
  docker exec "$database" psql -U postgres -d postgres -v ON_ERROR_STOP=1 -Atc \
    "SELECT name || ' ' || sha256 FROM schema_migrations ORDER BY name"
}
migrate
first=$(ledger)
test -n "$first"
migrate
test "$(ledger)" = "$first"

docker exec -i "$database" psql -U postgres -d postgres -v ON_ERROR_STOP=1 < "$repo/deployments/runtime-role.sql"
docker exec "$database" psql -U postgres -d postgres -v ON_ERROR_STOP=1 \
  -c "ALTER ROLE anime_runtime PASSWORD 'synthetic-bundle-runtime-password'"
api=$(docker run --network "container:$database" -d \
  -e REQUIRE_POSTGRES=true \
  -e 'DATABASE_URL=postgres://anime_runtime:synthetic-bundle-runtime-password@127.0.0.1:5432/postgres?sslmode=disable' \
  "$api_image")
for ((i=0; i<30; i++)); do
  if docker exec "$api" wget -Y off -qO /dev/null http://127.0.0.1:8080/readyz; then
    echo 'Verified the bundled migrations and restricted runtime readiness.'
    exit 0
  fi
  sleep 1
done
docker logs "$api" >&2
exit 1
