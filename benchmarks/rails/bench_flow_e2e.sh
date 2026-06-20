#!/usr/bin/env bash
# Flow Builder End-to-End Benchmark
#
# Validates that the flow builder can resolve a complete request flow for
# a Rails HTTP entry point. The benchmark:
#   1. Requests the flow for `GET /story_statuses` (a known Rails route)
#   2. Verifies the response has found:true
#   3. Checks that expected participants appear in the chain
#   4. Reports E2E success/failure
#
# Current limitations (v1): Ruby call-graph extraction is same-file only.
# Cross-file call chains (controller -> service -> model) are not yet
# traversed. This benchmark establishes the baseline and will improve as
# Ruby extraction matures.
#
# Usage:
#   ./bench_flow_e2e.sh
#   NANO_BRAIN_URL=http://localhost:3199 ./bench_flow_e2e.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

# Start server + register fixtures if not running
if ! server_healthy; then
  echo "Server not running. Starting setup..."
  "$SCRIPT_DIR/setup.sh"
fi

# ---- Resolve workspace ----
FIXTURES_DIR="$(cd "$SCRIPT_DIR/fixtures" && pwd)"
WS=$(resolve_workspace "$FIXTURES_DIR")
if [ -z "$WS" ]; then
  echo "ERROR: Could not resolve workspace for $FIXTURES_DIR"
  echo "Register the fixtures first: ./setup.sh"
  exit 1
fi
echo "Workspace: $WS"

# ---- Define test cases ----
# Each test case is an HTTP entry point with expected participants in the flow.
# "expected_chain" is a list of string substrings that must appear in chain.
# "expected_externals" is a list of external nodes that should appear.
#
# NOTE: With v1 Ruby extraction (same-file calls only), cross-file participants
# like service and model calls may not resolve. The benchmark reports which
# participants ARE found and scores accordingly.
TEST_CASES=$(
cat <<'CASES'
GET /story_statuses|["StoryStatusesController#index","StoryStatusesController"]|[]
POST /api/v1/signup|["Api::V1::TokensController#signup"]|[]
GET /|["HomeController#index","HomeController"]|[]
GET /users|["UsersController#index","UsersController"]|[]
CASES
)

TOTAL_CASES=$(echo "$TEST_CASES" | wc -l)
PASSED=0

echo ""
echo "==> Running $TOTAL_CASES flow E2E test cases..."
echo ""

CASE_RESULTS="["
FIRST=true
while IFS='|' read -r entry expected_chain expected_externals; do
  [ -z "$entry" ] && continue

  RESPONSE=$(flow_for_entry "$WS" "$entry")
  FOUND=$(echo "$RESPONSE" | python3 -c 'import json,sys;print("true" if json.load(sys.stdin).get("found") else "false")' 2>/dev/null || echo "false")

  # Extract participant names from chain
  CHAIN_NAMES=$(echo "$RESPONSE" | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin)
    for node in d.get("chain",[]):
        print(node.get("name",""))
except:
    pass
' 2>/dev/null || echo "")

  EXTERNAL_NAMES=$(echo "$RESPONSE" | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin)
    for node in d.get("externals",[]):
        print(node.get("name",""))
except:
    pass
' 2>/dev/null || echo "")

  CHAIN_COUNT=$(echo "$CHAIN_NAMES" | grep -c . || true)
  EXTERNAL_COUNT=$(echo "$EXTERNAL_NAMES" | grep -c . || true)

  # Check expected chain participants
  MISSING_CHAIN=""
  CHAIN_FOUND=0
  CHAIN_TOTAL=$(echo "$expected_chain" | python3 -c 'import json,sys;print(len(json.load(sys.stdin)))' 2>/dev/null || echo "0")

  for name in $(echo "$expected_chain" | python3 -c '
import json,sys
for n in json.load(sys.stdin):
    print(n)
' 2>/dev/null || true); do
    if echo "$CHAIN_NAMES" | grep -Fq "$name"; then
      CHAIN_FOUND=$((CHAIN_FOUND + 1))
    else
      MISSING_CHAIN="$MISSING_CHAIN $name"
    fi
  done

  # Check expected external participants
  EXTERNAL_FOUND=0
  EXTERNAL_TOTAL=$(echo "$expected_externals" | python3 -c 'import json,sys;print(len(json.load(sys.stdin)))' 2>/dev/null || echo "0")

  for name in $(echo "$expected_externals" | python3 -c '
import json,sys
for n in json.load(sys.stdin):
    print(n)
' 2>/dev/null || true); do
    if echo "$EXTERNAL_NAMES" | grep -Fq "$name"; then
      EXTERNAL_FOUND=$((EXTERNAL_FOUND + 1))
    fi
  done

  TOTAL_EXPECTED=$((CHAIN_TOTAL + EXTERNAL_TOTAL))
  TOTAL_FOUND=$((CHAIN_FOUND + EXTERNAL_FOUND))

  PASS="false"
  if [ "$FOUND" = "true" ] && [ "$TOTAL_FOUND" -eq "$TOTAL_EXPECTED" ]; then
    PASS="true"
    PASSED=$((PASSED + 1))
  fi

  [ "$FIRST" = false ] && CASE_RESULTS+=","
  FIRST=false

  CASE_RESULTS+=$(python3 -c "
import json
d = {
    \"entry\": \"$entry\",
    \"found\": $FOUND,
    \"chain_count\": $CHAIN_COUNT,
    \"external_count\": $EXTERNAL_COUNT,
    \"expected_participants\": $TOTAL_EXPECTED,
    \"found_participants\": $TOTAL_FOUND,
    \"missing_participants\": $(echo "$MISSING_CHAIN" | python3 -c 'import json,sys;print(json.dumps([l for l in sys.stdin.read().split() if l]))'),
    \"pass\": $PASS
}
print(json.dumps(d))
")

  STATUS="PASS" ; [ "$PASS" = "true" ] || STATUS="FAIL"
  echo "  $entry  ->  found=$FOUND  chain=$CHAIN_COUNT  externals=$EXTERNAL_COUNT  [$STATUS]"

  if [ -n "$MISSING_CHAIN" ]; then
    echo "          MISSING:$MISSING_CHAIN"
  fi
done <<< "$TEST_CASES"

CASE_RESULTS+="]"

# ---- Report ----
E2E_PASS=$([ "$PASSED" -eq "$TOTAL_CASES" ] && echo "true" || echo "false")

DETAILS=$(python3 -c "
import json
d = {
    \"benchmark\": \"flow_e2e\",
    \"workspace\": \"$WS\",
    \"total_cases\": $TOTAL_CASES,
    \"passed\": $PASSED,
    \"e2e_pass\": $E2E_PASS,
    \"cases\": $CASE_RESULTS,
    \"note\": \"Ruby call extraction is same-file only in v1. Cross-file participants (service, model) may not resolve yet.\"
}
print(json.dumps(d, indent=2))
")

print_scorecard "Flow Builder E2E" "$PASSED" "$TOTAL_CASES" "$DETAILS"
save_results "flow_e2e" "$DETAILS"

if [ "$E2E_PASS" = "true" ]; then
  exit 0
else
  exit 1
fi
