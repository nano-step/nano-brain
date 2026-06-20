#!/usr/bin/env bash
# Spin up the isolated Rails-benchmark server: a dedicated nano-brain instance
# on port 3199 backed by a clean `nanobrain_test` DB, indexing the Rails
# fixture project. Never touches the dev server (3100 / nanobrain_dev).
#
# Usage:
#   ./setup.sh                    # start server and register fixtures
#   ./setup.sh --skip-register    # start server only
#   ./teardown.sh                 # stop
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BIN="$ROOT/nano-brain"
PORT=3199
FIXTURES_DIR="$(cd "$(dirname "$0")/fixtures" && pwd)"
PG_SUPER="postgres://nanobrain:nanobrain@host.docker.internal:5432/postgres?sslmode=disable"
TEST_DB="postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_test?sslmode=disable"

cd "$ROOT"

# ---- Build binary ----
[ -x "$BIN" ] || { echo "==> Building nano-brain..."; CGO_ENABLED=0 go build -o "$BIN" ./cmd/nano-brain; }

# ---- Prepare test database ----
DB_EXISTS=$(psql "$PG_SUPER" -tAc "SELECT 1 FROM pg_database WHERE datname='nanobrain_test'" 2>/dev/null || echo "")
if [ "$DB_EXISTS" = "1" ]; then
  echo "==> nanobrain_test exists, running pending migrations"
  DATABASE_URL="$TEST_DB" "$BIN" db:migrate 2>/dev/null || true
else
  echo "==> Creating fresh nanobrain_test"
  psql "$PG_SUPER" -c "CREATE DATABASE nanobrain_test OWNER nanobrain;" 2>/dev/null || true
  psql "$TEST_DB" -c "CREATE EXTENSION IF NOT EXISTS vector;" 2>/dev/null || true
  DATABASE_URL="$TEST_DB" "$BIN" db:migrate 2>/dev/null || true
fi

# ---- Start isolated server ----
echo "==> Starting isolated flow-enabled server on :$PORT"
NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1 NANO_BRAIN_SERVER_PORT=$PORT NANO_BRAIN_FLOW_ENABLED=true \
  NANO_BRAIN_HYDE_ENABLED=false NANO_BRAIN_QUERY_PREPROCESSING_ENABLED=false \
  DATABASE_URL="$TEST_DB" "$BIN" serve > /tmp/nb-rails-bench.log 2>&1 &
echo $! > /tmp/nb-rails-bench.pid
echo "    pid $(cat /tmp/nb-rails-bench.pid) (log: /tmp/nb-rails-bench.log)"

# ---- Wait for server health ----
echo "==> Waiting for server health"
for i in $(seq 1 30); do
  if curl -sf -m 3 "http://localhost:$PORT/api/v1/health" >/dev/null 2>&1; then
    break
  fi
  if curl -s -m 3 "http://localhost:$PORT/api/v1/health" 2>/dev/null | grep -q workspace_required; then
    break
  fi
  sleep 2
done

# ---- Skip registration if requested ----
if [ "${1:-}" = "--skip-register" ]; then
  echo "==> Server running on :$PORT (skip-register mode)"
  echo "    Register fixtures later with:"
  echo "    curl -s -X POST http://localhost:$PORT/api/v1/init -H 'Content-Type: application/json' -d '{\"root_path\":\"$FIXTURES_DIR\"}'"
  exit 0
fi

# ---- Register Rails fixture project ----
echo "==> Registering Rails fixture project: $FIXTURES_DIR"
INIT_RESULT=$(curl -s -X POST "http://localhost:$PORT/api/v1/init" -H 'Content-Type: application/json' -d "{\"root_path\":\"$FIXTURES_DIR\"}")
echo "    $INIT_RESULT"

# ---- Resolve workspace hash ----
WS=$(curl -s -X POST "http://localhost:$PORT/api/v1/workspaces/resolve" -H 'Content-Type: application/json' -d "{\"path\":\"$FIXTURES_DIR\"}" | python3 -c 'import json,sys;print(json.load(sys.stdin).get("workspace_hash",""))' 2>/dev/null || echo "")
echo "==> Workspace hash: $WS"

# ---- Wait for HTTP edges to be indexed ----
echo "==> Waiting for routes to be indexed (http edges ≥ 20)"
for i in $(seq 1 60); do
  COUNT=$(psql "$TEST_DB" -t -A -c "select count(*) from graph_edges where workspace_hash='$WS' and edge_type='http'" 2>/dev/null || echo "0")
  if [ "$COUNT" -ge 20 ] 2>/dev/null; then
    echo "    $COUNT http edges found"
    break
  fi
  sleep 5
done

# ---- Wait for embeddings ----
echo "==> Waiting for embeddings to finish"
for i in $(seq 1 120); do
  DEPTH=$(curl -s -m 5 "http://localhost:$PORT/api/status" 2>/dev/null | python3 -c 'import json,sys;print(json.load(sys.stdin).get("queue_pending",1))' 2>/dev/null || echo "1")
  if [ "$DEPTH" = "0" ]; then
    echo "    Embedding queue drained"
    break
  fi
  sleep 5
done

echo ""
echo "==> Setup complete. Server running on :$PORT"
echo "    Workspace: $WS"
echo "    Run benchmarks:"
echo "      ./bench_route_extraction.sh"
echo "      ./bench_cfg_completeness.sh"
echo "      ./bench_flow_e2e.sh"
echo "    Stop: ./teardown.sh"
echo ""

# Export for use by sourcing scripts
export NANO_BRAIN_SERVER_URL="http://localhost:$PORT"
export NANO_BRAIN_TEST_WS="$WS"
export NANO_BRAIN_FIXTURES_DIR="$FIXTURES_DIR"
