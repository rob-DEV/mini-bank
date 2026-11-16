#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

psql() {
  docker compose exec -T postgres psql -U minibank -d minibank -v ON_ERROR_STOP=1 "$@"
}

docker compose up -d --wait postgres
echo "applying schema..."
psql < infra/postgres/schema.sql
echo "seeding..."
psql < infra/postgres/seed.sql
echo "done"