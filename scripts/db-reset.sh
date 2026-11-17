#!/usr/bin/env bash
# Rebuild every service database from scratch: drop and recreate each one,
# apply its migrations, then load its seed data. Dev only — deletes everything.
#
# Each service owns its own database:
#   services/<name>/migrations  ->  database minibank_<name>
#   services/<name>/seeds       ->  loaded with -no-versioning
set -euo pipefail
cd "$(dirname "$0")/.."

SERVICES=(accounts ledger)

PG_USER=minibank
PG_PASS=minibank
PG_HOST=localhost
PG_PORT=5432
ADMIN_DB=minibank   # the default database the container creates; used to manage the others

psql_admin() {
  docker compose exec -T postgres \
    psql -U "$PG_USER" -d "$ADMIN_DB" -v ON_ERROR_STOP=1 -q "$@"
}

echo "==> starting postgres"
docker compose up -d --wait postgres

export GOOSE_DRIVER=postgres

for svc in "${SERVICES[@]}"; do
  db="minibank_${svc}"
  echo
  echo "==> ${svc}: recreating database ${db}"
  # WITH (FORCE) disconnects anything still attached (your Go service, pgAdmin)
  psql_admin -c "DROP DATABASE IF EXISTS ${db} WITH (FORCE);"
  psql_admin -c "CREATE DATABASE ${db} OWNER ${PG_USER};"

  export GOOSE_DBSTRING="postgres://${PG_USER}:${PG_PASS}@${PG_HOST}:${PG_PORT}/${db}?sslmode=disable"

  echo "==> ${svc}: migrating"
  go tool goose -dir "services/${svc}/migrations" up

  if [[ -d "services/${svc}/seeds" ]]; then
    echo "==> ${svc}: seeding"
    go tool goose -dir "services/${svc}/seeds" -no-versioning up
  fi

  go tool goose -dir "services/${svc}/migrations" status
done

echo
echo "==> done. Connection strings:"
for svc in "${SERVICES[@]}"; do
  echo "    ${svc}: postgres://${PG_USER}:${PG_PASS}@${PG_HOST}:${PG_PORT}/minibank_${svc}"
done