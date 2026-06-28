#!/usr/bin/env bash
# Spin up the isolated capability-benchmark server for Vue: a dedicated nano-brain
# instance on port 3199 backed by a clean `nanobrain_test` DB, indexing the Vue
# workspace. Never touches the dev server (3100 / nanobrain_dev).
#
# Usage:  benchmarks/vue/capability/setup.sh   (run from the Vue project root)
# Then:   go test -tags=capbench -run TestCapabilityBenchmark ./benchmarks/vue/capability/
set -euo pipefail

PG_SUPER="postgres://nanobrain:nanobrain@localhost:5432/postgres"
TEST_DB="postgres://nanobrain:nanobrain@localhost:5432/nanobrain_test"
BRAIN_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
BIN="$BRAIN_ROOT/nano-brain"
PORT=3199
VUE_ROOT="${VUE_ROOT:-.}"

cd "$BRAIN_ROOT"
[ -x "$BIN" ] || { echo "building nano-brain..."; CGO_ENABLED=0 go build -o "$BIN" ./cmd/nano-brain; }

DB_EXISTS=$(psql "$PG_SUPER" -tAc "SELECT 1 FROM pg_database WHERE datname='nanobrain_test'" 2>/dev/null || echo "")

if [ "$DB_EXISTS" = "1" ]; then
  echo "==> nanobrain_test exists, running pending migrations"
  DATABASE_URL="$TEST_DB" "$BIN" db:migrate
else
  echo "==> Creating fresh nanobrain_test"
  psql "$PG_SUPER" -c "CREATE DATABASE nanobrain_test OWNER nanobrain;"
  psql "$TEST_DB" -c "CREATE EXTENSION IF NOT EXISTS vector;"
  DATABASE_URL="$TEST_DB" "$BIN" db:migrate
fi

echo "==> Starting isolated flow-enabled server on :$PORT"
NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1 NANO_BRAIN_SERVER_PORT=$PORT NANO_BRAIN_FLOW_ENABLED=true \
  NANO_BRAIN_HYDE_ENABLED=false NANO_BRAIN_QUERY_PREPROCESSING_ENABLED=false \
  DATABASE_URL="$TEST_DB" "$BIN" serve > /tmp/nb-vue-bench.log 2>&1 &
echo $! > /tmp/nb-vue-bench.pid
echo "    pid $(cat /tmp/nb-vue-bench.pid) (log: /tmp/nb-vue-bench.log)"

echo "==> Waiting for server health"
until curl -sf -m 3 "http://localhost:$PORT/api/v1/health" >/dev/null 2>&1 || \
      curl -s -m 3 "http://localhost:$PORT/api/v1/health" 2>/dev/null | grep -q workspace_required; do sleep 2; done

echo "==> Indexing Vue project into isolated DB"
curl -s -X POST "http://localhost:$PORT/api/v1/init" -H 'Content-Type: application/json' \
  -d "{\"root_path\":\"$(cd "$VUE_ROOT" && pwd)\"}" >/dev/null

echo "==> Waiting for embeddings to finish (search-qa needs them; baseline assumes a complete index)"
until [ "$(curl -s -m 5 "http://localhost:$PORT/api/status" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("queue_pending",1))')" = "0" ]; do sleep 5; done

echo "==> Ready. Index + embeddings complete. Run the benchmark:"
echo "    go test -tags=capbench -run TestCapabilityBenchmark ./benchmarks/vue/capability/"
echo "    (stop the server later with: kill \$(cat /tmp/nb-vue-bench.pid) )"
