#!/usr/bin/env bash
# Stop comparison benchmark infrastructure and clean up.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RESULTS_DIR="${RESULTS_DIR:-$SCRIPT_DIR/results}"

echo "==> Stopping comparison benchmark infrastructure"

# Stop nano-brain test server if running
if [ -f /tmp/nb-comparison-bench.pid ]; then
  PID=$(cat /tmp/nb-comparison-bench.pid)
  echo "    Stopping nano-brain comparison server (pid $PID)"
  kill "$PID" 2>/dev/null || true
  rm -f /tmp/nb-comparison-bench.pid
fi

# Stop Docker containers for comparison tools
if [ -f "$SCRIPT_DIR/docker-compose.yml" ]; then
  echo "    Stopping Docker containers"
  cd "$SCRIPT_DIR"
  docker compose down -v 2>/dev/null || docker-compose down -v 2>/dev/null || true
fi

# Stop any Python venv processes
if [ -d "$SCRIPT_DIR/.venv" ]; then
  echo "    Cleaning up Python virtual environment"
  rm -rf "$SCRIPT_DIR/.venv"
fi

# Clean up temporary export directories
if [ -d /tmp/nb-comparison-export ]; then
  echo "    Cleaning up exported documents"
  rm -rf /tmp/nb-comparison-export
fi

# Clean up temporary Mem0 data
if [ -d /tmp/mem0-data ]; then
  echo "    Cleaning up Mem0 data"
  rm -rf /tmp/mem0-data
fi

# Clean up temporary GraphRAG data
if [ -d /tmp/graphrag-data ]; then
  echo "    Cleaning up GraphRAG data"
  rm -rf /tmp/graphrag-data
fi

# Clean up temporary Zep data
if [ -d /tmp/zep-data ]; then
  echo "    Cleaning up Zep data"
  rm -rf /tmp/zep-data
fi

# Clean up LlamaIndex index files
if [ -d "$SCRIPT_DIR/.llamaindex" ]; then
  echo "    Cleaning up LlamaIndex index"
  rm -rf "$SCRIPT_DIR/.llamaindex"
fi

echo "==> Teardown complete"
echo "    Results preserved in: $RESULTS_DIR"
