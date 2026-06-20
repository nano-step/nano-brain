#!/usr/bin/env bash
# Stop the Rails benchmark server and clean up.
set -euo pipefail

if [ -f /tmp/nb-rails-bench.pid ]; then
  PID=$(cat /tmp/nb-rails-bench.pid)
  echo "==> Stopping Rails benchmark server (pid $PID)"
  kill "$PID" 2>/dev/null || true
  rm -f /tmp/nb-rails-bench.pid
  echo "    Stopped"
else
  echo "==> No Rails benchmark server PID file found"
fi

echo "==> Done"
