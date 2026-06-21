#!/usr/bin/env bash
# Ruby vs JS Extraction Comparison Benchmark
#
# Compares Ruby extraction metrics (Phil-timeshel) vs JS/TS (zengamingx):
#   - Extraction speed (time per file)
#   - Edges per file
#   - Flow traversal time
#
# Usage:
#   ./bench_ruby_vs_js.sh
#   NANO_BRAIN_URL=http://localhost:3199 ./bench_ruby_vs_js.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

PHIL_WS="becf297d74539d99bb858bb91dd79b0611d2e47fd946e92149a1887af02b8d95"

if ! server_healthy; then
  echo "ERROR: Server not running on $SERVER_URL"
  echo "Start with: ./setup.sh"
  exit 1
fi

echo "==> Ruby vs JS Extraction Comparison"
echo "    Server: $SERVER_URL"
echo ""

echo "==> Measuring Ruby extraction (Phil-timeshel)..."
RUBY_ENDPOINTS=$(api_get "/api/v1/graph/flow/endpoints?workspace=$PHIL_WS" | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin)
    print(len(d.get("endpoints",[])))
except:
    print(0)
')

RUBY_ENTRIES=("POST /api/v1/signup" "GET /api/v1/payments" "POST /api/v1/moments" "GET /users" "GET /story_statuses")
RUBY_TOTAL_MS=0
RUBY_TOTAL_NODES=0
RUBY_COUNT=0

for entry in "${RUBY_ENTRIES[@]}"; do
  START_MS=$(python3 -c 'import time; print(int(time.time()*1000))')
  RESP=$(curl -s -X POST "$SERVER_URL/api/v1/graph/flow" \
    -H 'Content-Type: application/json' \
    -d "{\"workspace\":\"$PHIL_WS\",\"entry\":\"$entry\",\"max_depth\":8,\"format\":\"json\"}" 2>/dev/null)
  END_MS=$(python3 -c 'import time; print(int(time.time()*1000))')
  ELAPSED=$((END_MS - START_MS))

    FOUND=$(echo "$RESP" | python3 -c 'import json,sys;v=json.load(sys.stdin).get("found",False);print("True" if v else "False")' 2>/dev/null || echo "False")
  NODE_COUNT=$(echo "$RESP" | python3 -c 'import json,sys;print(len(json.load(sys.stdin).get("nodes",[])))' 2>/dev/null || echo "0")

  if [ "$FOUND" = "True" ]; then
    RUBY_TOTAL_MS=$((RUBY_TOTAL_MS + ELAPSED))
    RUBY_TOTAL_NODES=$((RUBY_TOTAL_NODES + NODE_COUNT))
    RUBY_COUNT=$((RUBY_COUNT + 1))
  fi
done

RUBY_AVG_MS=0
RUBY_AVG_NODES=0
if [ "$RUBY_COUNT" -gt 0 ]; then
  RUBY_AVG_MS=$((RUBY_TOTAL_MS / RUBY_COUNT))
  RUBY_AVG_NODES=$(python3 -c "print(round($RUBY_TOTAL_NODES/$RUBY_COUNT,1))")
fi

echo "    Ruby endpoints: $RUBY_ENDPOINTS"
echo "    Ruby flows resolved: $RUBY_COUNT/5"
echo "    Ruby avg nodes/flow: $RUBY_AVG_NODES"
echo "    Ruby avg traversal: ${RUBY_AVG_MS}ms"
echo ""

echo "==> Looking for JS/TS workspace..."
JS_WS=$(curl -s "$SERVER_URL/api/v1/workspaces" 2>/dev/null | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin)
    for w in d.get("workspaces",[]):
        name = w.get("name","")
        path = w.get("root_path","")
        if any(ext in path.lower() for ext in ["zengaming", "js", "ts", "node", "express", "next"]):
            print(w.get("hash",""))
            sys.exit(0)
    print("")
except:
    print("")
' 2>/dev/null || echo "")

JS_ENDPOINTS=0
JS_AVG_MS=0
JS_AVG_NODES=0
JS_FOUND=false

if [ -n "$JS_WS" ]; then
  JS_ENDPOINTS=$(api_get "/api/v1/graph/flow/endpoints?workspace=$JS_WS" | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin)
    print(len(d.get("endpoints",[])))
except:
    print(0)
  ')

  JS_ENTRIES=$(api_get "/api/v1/graph/flow/endpoints?workspace=$JS_WS" | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin)
    for ep in d.get("endpoints",[])[:5]:
        print(ep.get("source",""))
except:
    pass
  ')

  JS_TOTAL_MS=0
  JS_TOTAL_NODES=0
  JS_COUNT=0

  while IFS= read -r entry; do
    [ -z "$entry" ] && continue
    START_MS=$(python3 -c 'import time; print(int(time.time()*1000))')
    RESP=$(curl -s -X POST "$SERVER_URL/api/v1/graph/flow" \
      -H 'Content-Type: application/json' \
      -d "{\"workspace\":\"$JS_WS\",\"entry\":\"$entry\",\"max_depth\":8,\"format\":\"json\"}" 2>/dev/null)
    END_MS=$(python3 -c 'import time; print(int(time.time()*1000))')
    ELAPSED=$((END_MS - START_MS))

  FOUND=$(echo "$RESP" | python3 -c 'import json,sys;v=json.load(sys.stdin).get("found",False);print("True" if v else "False")' 2>/dev/null || echo "False")
    NODE_COUNT=$(echo "$RESP" | python3 -c 'import json,sys;print(len(json.load(sys.stdin).get("nodes",[])))' 2>/dev/null || echo "0")

    if [ "$FOUND" = "True" ]; then
      JS_TOTAL_MS=$((JS_TOTAL_MS + ELAPSED))
      JS_TOTAL_NODES=$((JS_TOTAL_NODES + NODE_COUNT))
      JS_COUNT=$((JS_COUNT + 1))
    fi
  done <<< "$JS_ENTRIES"

  if [ "$JS_COUNT" -gt 0 ]; then
    JS_AVG_MS=$((JS_TOTAL_MS / JS_COUNT))
    JS_AVG_NODES=$(python3 -c "print(round($JS_TOTAL_NODES/$JS_COUNT,1))")
    JS_FOUND=True
  fi

  echo "    JS workspace: $JS_WS"
  echo "    JS endpoints: $JS_ENDPOINTS"
  echo "    JS flows resolved: $JS_COUNT"
  echo "    JS avg nodes/flow: $JS_AVG_NODES"
  echo "    JS avg traversal: ${JS_AVG_MS}ms"
else
  echo "    No JS/TS workspace found — comparison will show Ruby-only metrics"
fi
echo ""

echo "==> Comparison"
if [ "$JS_FOUND" = "True" ]; then
  SPEED_RATIO=$(python3 -c "print(round($JS_AVG_MS/$RUBY_AVG_MS,2) if $RUBY_AVG_MS>0 else 'N/A')")
  NODES_RATIO=$(python3 -c "print(round($JS_AVG_NODES/$RUBY_AVG_NODES,2) if $RUBY_AVG_NODES>0 else 'N/A')")
  echo "    Speed ratio (JS/Ruby): ${SPEED_RATIO}x"
  echo "    Nodes ratio (JS/Ruby): ${NODES_RATIO}x"
else
  echo "    (JS workspace not available for comparison)"
fi
echo ""

DETAILS=$(python3 -c "
import json
d = {
    'benchmark': 'ruby_vs_js',
    'ruby': {
        'workspace': '$PHIL_WS',
        'endpoints': $RUBY_ENDPOINTS,
        'flows_resolved': $RUBY_COUNT,
        'avg_nodes_per_flow': $RUBY_AVG_NODES,
        'avg_traversal_time_ms': $RUBY_AVG_MS
    },
    'js': {
        'workspace': '$JS_WS',
        'endpoints': $JS_ENDPOINTS,
        'found': $JS_FOUND,
        'avg_nodes_per_flow': $JS_AVG_NODES,
        'avg_traversal_time_ms': $JS_AVG_MS
    }
}
print(json.dumps(d, indent=2))
")

print_scorecard "Ruby vs JS Comparison" "$RUBY_COUNT" "5" "$DETAILS"
save_results "ruby_vs_js" "$DETAILS"
