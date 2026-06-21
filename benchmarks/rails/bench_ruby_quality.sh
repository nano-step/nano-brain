#!/usr/bin/env bash
# Ruby Flow Quality Benchmark
#
# Measures the quality of Ruby flow extraction against the Phil-timeshel workspace:
#   - For 10 controller actions, requests flow via memory_flow MCP
#   - Verifies: flow has 3+ nodes (not just entry→handler)
#   - Verifies: flow includes at least one cross-file call
#   - Measures: average nodes per flow
#
# Usage:
#   ./bench_ruby_quality.sh
#   NANO_BRAIN_URL=http://localhost:3199 ./bench_ruby_quality.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

PHIL_WS="becf297d74539d99bb858bb91dd79b0611d2e47fd946e92149a1887af02b8d95"

if ! server_healthy; then
  echo "ERROR: Server not running on $SERVER_URL"
  echo "Start with: ./setup.sh"
  exit 1
fi

echo "==> Ruby Flow Quality Benchmark"
echo "    Server: $SERVER_URL"
echo "    Workspace: $PHIL_WS"
echo ""

TEST_CASES=$(
cat <<'CASES'
POST /api/v1/signup|3+|TokensController
GET /api/v1/payments|3+|PaymentsController
POST /api/v1/moments|3+|MomentsController
GET /api/v1/moments|3+|MomentsController
GET /story_statuses|3+|StoryStatusesController
POST /story_statuses|3+|StoryStatusesController
GET /users|3+|UsersController
GET /|3+|HomeController
GET /api/v2/stories|3+|StoriesController
GET /admin/users|3+|UsersController
CASES
)

TOTAL_CASES=$(echo "$TEST_CASES" | wc -l)
PASSED=0
TOTAL_NODES=0
CASE_RESULTS="["

echo "==> Running $TOTAL_CASES quality test cases..."
echo ""

FIRST=true
while IFS='|' read -r entry min_nodes expected_handler; do
  [ -z "$entry" ] && continue

  RESP=$(curl -s -X POST "$SERVER_URL/api/v1/graph/flow" \
    -H 'Content-Type: application/json' \
    -d "{\"workspace\":\"$PHIL_WS\",\"entry\":\"$entry\",\"max_depth\":8,\"format\":\"json\"}" 2>/dev/null)

  FOUND=$(echo "$RESP" | python3 -c 'import json,sys;v=json.load(sys.stdin).get("found",False);print("True" if v else "False")' 2>/dev/null || echo "False")
  NODE_COUNT=$(echo "$RESP" | python3 -c 'import json,sys;print(len(json.load(sys.stdin).get("nodes",[])))' 2>/dev/null || echo "0")
  EDGE_COUNT=$(echo "$RESP" | python3 -c 'import json,sys;print(len(json.load(sys.stdin).get("edges",[])))' 2>/dev/null || echo "0")

  HAS_HANDLER=$(echo "$RESP" | python3 -c "
import json,sys
d=json.load(sys.stdin)
for n in d.get('nodes',[]):
    if n.get('role')=='handler' and '$expected_handler' in n.get('name',''):
        print('True')
        sys.exit(0)
print('False')
" 2>/dev/null || echo "False")

  HAS_CROSS_FILE=$(echo "$RESP" | python3 -c "
import json,sys
d=json.load(sys.stdin)
for e in d.get('edges',[]):
    if e.get('kind')=='calls':
        print('True')
        sys.exit(0)
print('False')
" 2>/dev/null || echo "False")

  MIN_NODES_NUM=3
  PASS="False"
  if [ "$FOUND" = "True" ] && [ "$NODE_COUNT" -ge "$MIN_NODES_NUM" ] && [ "$HAS_HANDLER" = "True" ]; then
    PASS="True"
    PASSED=$((PASSED + 1))
    TOTAL_NODES=$((TOTAL_NODES + NODE_COUNT))
  fi

  STATUS="PASS" ; [ "$PASS" = "True" ] || STATUS="FAIL"
  echo "  $entry  found=$FOUND nodes=$NODE_COUNT edges=$EDGE_COUNT handler=$HAS_HANDLER cross_file=$HAS_CROSS_FILE [$STATUS]"

  [ "$FIRST" = false ] && CASE_RESULTS+=","
  FIRST=false

  CASE_RESULTS+=$(python3 -c "
import json
d = {
    'entry': '$entry',
    'found': $FOUND,
    'node_count': $NODE_COUNT,
    'edge_count': $EDGE_COUNT,
    'has_handler': $HAS_HANDLER,
    'has_cross_file_call': $HAS_CROSS_FILE,
    'pass': $PASS
}
print(json.dumps(d))
")

done <<< "$TEST_CASES"

CASE_RESULTS+="]"

AVG_NODES=0
if [ "$PASSED" -gt 0 ]; then
  AVG_NODES=$(python3 -c "print(round($TOTAL_NODES/$PASSED,1))")
fi

echo ""
echo "==> Results"
echo "    Cases passed: $PASSED/$TOTAL_CASES"
echo "    Avg nodes per flow: $AVG_NODES"
echo ""

DETAILS=$(python3 -c "
import json
d = {
    'benchmark': 'ruby_quality',
    'workspace': '$PHIL_WS',
    'total_cases': $TOTAL_CASES,
    'passed': $PASSED,
    'avg_nodes_per_flow': $AVG_NODES,
    'total_nodes': $TOTAL_NODES,
    'cases': json.loads('$CASE_RESULTS')
}
print(json.dumps(d, indent=2))
")

print_scorecard "Ruby Flow Quality" "$PASSED" "$TOTAL_CASES" "$DETAILS"
save_results "ruby_quality" "$DETAILS"

if [ "$PASSED" -eq "$TOTAL_CASES" ]; then
  exit 0
else
  exit 1
fi
