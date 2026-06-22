#!/usr/bin/env bash
# Setup comparison benchmark infrastructure:
#   - Ensure Docker is available
#   - Create Python venv with mem0, cognee, llamaindex, zep dependencies
#   - Start Docker Compose for tools needing databases
#   - Verify nano-brain test server is running on port 3199
#   - Export workspace documents for ingestion by comparison tools
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "==> Checking prerequisites"

# --- Docker ---
if ! command -v docker &>/dev/null; then
  echo "ERROR: Docker not found. Install Docker Desktop or docker-ce."
  exit 1
fi
echo "    Docker: $(docker --version | head -1)"

# --- Docker Compose ---
if ! docker compose version &>/dev/null 2>&1 && ! docker-compose version &>/dev/null 2>&1; then
  echo "ERROR: Docker Compose not found."
  exit 1
fi
echo "    Docker Compose: available"

# --- Python ---
if ! command -v python3 &>/dev/null; then
  echo "ERROR: Python 3 not found."
  exit 1
fi
echo "    Python: $(python3 --version)"

# --- nano-brain server ---
echo ""
echo "==> Checking nano-brain test server on port 3199"
if server_healthy "http://localhost:3199"; then
  echo "    Server is healthy on :3199"
else
  echo "    Server not running. Starting isolated test server..."
  TEST_DB="postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_test?sslmode=disable"

  if [ ! -x "$ROOT/nano-brain" ]; then
    echo "    Building nano-brain binary..."
    (cd "$ROOT" && CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain)
  fi

  NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1 NANO_BRAIN_SERVER_PORT=3199 \
    DATABASE_URL="$TEST_DB" "$ROOT/nano-brain" serve > /tmp/nb-comparison-bench.log 2>&1 &
  echo $! > /tmp/nb-comparison-bench.pid
  echo "    Started pid $(cat /tmp/nb-comparison-bench.pid)"

  for i in $(seq 1 30); do
    if server_healthy "http://localhost:3199"; then
      echo "    Server healthy after ${i}s"
      break
    fi
    sleep 2
  done
fi

# --- Wait for embedding queue ---
echo ""
echo "==> Waiting for embedding queue to drain"
for i in $(seq 1 120); do
  if server_queue_empty "http://localhost:3199"; then
    echo "    Queue drained"
    break
  fi
  sleep 5
done

# --- Python virtual environment ---
echo ""
echo "==> Setting up Python virtual environment"
VENV_DIR="$SCRIPT_DIR/.venv"
if [ ! -d "$VENV_DIR" ]; then
  python3 -m venv "$VENV_DIR"
fi
source "$VENV_DIR/bin/activate"

echo "    Installing Python dependencies..."
pip install --quiet --upgrade pip
pip install --quiet \
  mem0ai \
  cognee \
  llama-index-core \
  llama-index-embeddings-openai \
  llama-index-vector-stores-qdrant \
  qdrant-client \
  requests \
  json5 \
  zep-cloud \
  pydantic \
  2>/dev/null || {
    echo "    WARNING: Some Python packages failed to install."
    echo "    Attempting individual installs..."
    pip install --quiet mem0ai 2>/dev/null || echo "    - mem0ai: FAILED"
    pip install --quiet cognee 2>/dev/null || echo "    - cognee: FAILED"
    pip install --quiet llama-index-core 2>/dev/null || echo "    - llama-index-core: FAILED"
    pip install --quiet requests 2>/dev/null || echo "    - requests: FAILED"
    pip install --quiet zep-cloud 2>/dev/null || echo "    - zep-cloud: FAILED"
  }

echo "    Python dependencies installed"

# --- Docker Compose for comparison tools ---
echo ""
echo "==> Starting Docker services for comparison tools"
cat > "$SCRIPT_DIR/docker-compose.yml" <<'DOCKEREOF'
version: "3.8"
services:
  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"
      - "6334:6334"
    volumes:
      - qdrant_data:/qdrant/storage
    environment:
      QDRANT__SERVICE__GRPC_PORT: "6334"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:6333/healthz"]
      interval: 10s
      timeout: 5s
      retries: 5

  neo4j:
    image: neo4j:5-community
    ports:
      - "7474:7474"
      - "7687:7687"
    environment:
      NEO4J_AUTH: neo4j/comparison-bench
      NEO4J_PLUGINS: '["graph-data-science"]'
    volumes:
      - neo4j_data:/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:7474"]
      interval: 10s
      timeout: 5s
      retries: 5

  zep:
    image: ghcr.io/getzep/zep:latest
    ports:
      - "8080:8080"
    environment:
      ZEP_STORAGE_POSTGRES_URL: "postgres://zep:zep@zep-postgres:5432/zep?sslmode=disable"
      ZEP_AUTH_SECRET: "comparison-bench-secret"
    depends_on:
      zep-postgres:
        condition: service_healthy

  zep-postgres:
    image: postgres:17
    ports:
      - "5433:5432"
    environment:
      POSTGRES_USER: zep
      POSTGRES_PASSWORD: zep
      POSTGRES_DB: zep
    volumes:
      - zep_pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U zep"]
      interval: 5s
      timeout: 3s
      retries: 10

volumes:
  qdrant_data:
  neo4j_data:
  zep_pgdata:
DOCKEREOF

cd "$SCRIPT_DIR"
docker compose up -d 2>/dev/null || docker-compose up -d 2>/dev/null || {
  echo "    WARNING: Docker Compose failed. Some tools may not be available."
  echo "    You can start services manually: docker compose up -d"
}

echo "    Waiting for Docker services to be healthy..."
for i in $(seq 1 60); do
  QDRANT_OK=$(curl -sf http://localhost:6333/healthz >/dev/null 2>&1 && echo "ok" || echo "wait")
  NEO4J_OK=$(curl -sf http://localhost:7474 >/dev/null 2>&1 && echo "ok" || echo "wait")
  if [ "$QDRANT_OK" = "ok" ] && [ "$NEO4J_OK" = "ok" ]; then
    echo "    Docker services ready"
    break
  fi
  sleep 5
done

# --- Export workspace documents ---
echo ""
echo "==> Exporting workspace documents for comparison tools"
EXPORT_DIR="/tmp/nb-comparison-export"
mkdir -p "$EXPORT_DIR"

for ws_name in $(list_workspaces); do
  ws_hash=$(get_workspace "$ws_name")
  if [ -n "$ws_hash" ]; then
    echo "    Exporting $ws_name ($ws_hash)..."
    export_workspace_docs "$ws_hash" "$EXPORT_DIR/$ws_name"
  fi
done

echo ""
echo "==> Setup complete"
echo "    nano-brain:  http://localhost:3199"
echo "    Qdrant:      http://localhost:6333"
echo "    Neo4j:       http://localhost:7474"
echo "    Zep:         http://localhost:8080"
echo "    Exports:     $EXPORT_DIR"
echo ""
echo "    Run benchmarks:"
echo "      ./bench_nanobrain.sh"
echo "      ./bench_mem0.sh"
echo "      ./bench_cognee.sh"
echo "      ./bench_graphrag.sh"
echo "      ./bench_llamaindex.sh"
echo "      ./bench_zep.sh"
echo ""
echo "    Stop: ./teardown.sh"
