#!/usr/bin/env bash
# CFG Extraction Completeness Benchmark
#
# Measures how many controller actions have a valid control-flow graph (CFG)
# extracted. The benchmark:
#   1. Defines the expected controller actions from the fixture project
#   2. For each action, queries POST /api/v1/graph/flowchart on the HTTP route
#   3. Counts how many return found:true with a valid CFG
#   4. Reports completeness = cfgs_found / actions_expected
#
# This provides a baseline: Ruby CFG extraction may not yet be implemented
# (Phase 2), in which case 0/actions will be found. Run this benchmark after
# implementing Ruby CFG extraction to track the improvement.
#
# Usage:
#   ./bench_cfg_completeness.sh
#   NANO_BRAIN_URL=http://localhost:3199 ./bench_cfg_completeness.sh
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

# ---- Expected controller actions ----
#
# These are the actions defined in fixtures/app/controllers/ that a complete
# Ruby CFG extractor SHOULD produce CFGs for:
#
#   HomeController:           index
#   StoryStatusesController:  index, show, new, create, edit, update, destroy, status_params
#   UsersController:          index, show, new, create, edit, update, destroy, token_check, user_params
#   Api::V1::TokensController: signup
#   Api::V1::MomentsController: index, create, moment_params
#   Api::V1::PaymentsController: index, billing, upcoming_month
#   ApplicationController:    (no actions)
#
EXPECTED_ACTIONS=$(
cat <<'ACTIONS'
GET /|HomeController#index
GET /story_statuses|StoryStatusesController#index
POST /story_statuses|StoryStatusesController#create
GET /story_statuses/new|StoryStatusesController#new
GET /story_statuses/:id/edit|StoryStatusesController#edit
GET /story_statuses/:id|StoryStatusesController#show
PATCH /story_statuses/:id|StoryStatusesController#update
DELETE /story_statuses/:id|StoryStatusesController#destroy
GET /api/v1/moments|Api::V1::MomentsController#index
POST /api/v1/moments|Api::V1::MomentsController#create
GET /api/v1/payments|Api::V1::PaymentsController#index
POST /api/v1/payments/billing|Api::V1::PaymentsController#billing
GET /api/v1/payments/upcoming-month|Api::V1::PaymentsController#upcoming_month
POST /api/v1/signup|Api::V1::TokensController#signup
GET /users|UsersController#index
POST /users|UsersController#create
GET /users/:id|UsersController#show
PATCH /users/:id|UsersController#update
DELETE /users/:id|UsersController#destroy
GET /users/token_check|UsersController#token_check
ACTIONS
)

TOTAL_ACTIONS=$(echo "$EXPECTED_ACTIONS" | wc -l)

# ---- Query CFG for each action ----
echo ""
echo "==> Querying CFGs for $TOTAL_ACTIONS controller actions..."
echo ""

CFGS_FOUND=0
CFG_DETAILS="["

FIRST=true
while IFS='|' read -r entry action; do
  [ -z "$entry" ] && continue

  RESPONSE=$(flowchart_for_entry "$WS" "$entry")
  FOUND=$(echo "$RESPONSE" | python3 -c 'import json,sys;print("true" if json.load(sys.stdin).get("found") else "false")' 2>/dev/null || echo "false")
  STATUS=$(echo "$RESPONSE" | python3 -c 'import json,sys;print(json.load(sys.stdin).get("status",""))' 2>/dev/null || echo "")

  if [ "$FOUND" = "true" ]; then
    CFGS_FOUND=$((CFGS_FOUND + 1))
    NODE_COUNT=$(echo "$RESPONSE" | python3 -c '
import json,sys
try:
    cfg=json.load(sys.stdin).get("cfg",None)
    if cfg:
        print(len(cfg.get("nodes",[])))
    else:
        print(0)
except:
    print(0)
' 2>/dev/null || echo "0")
    RESULT="found"
  else
    NODE_COUNT=0
    RESULT="not_found"
  fi

  [ "$FIRST" = false ] && CFG_DETAILS+=","
  FIRST=false
  CFG_DETAILS+=$(python3 -c "
import json
print(json.dumps({\"entry\":\"$entry\",\"action\":\"$action\",\"result\":\"$RESULT\",\"node_count\":$NODE_COUNT}))
")

  echo "  $entry  ->  $action  [$RESULT]"
done <<< "$EXPECTED_ACTIONS"

CFG_DETAILS+="]"

# ---- Report ----
PCT=$(python3 -c "print(round($CFGS_FOUND/$TOTAL_ACTIONS*100,1))" 2>/dev/null || echo "0")

DETAILS=$(python3 -c "
import json
d = {
    \"benchmark\": \"cfg_completeness\",
    \"workspace\": \"$WS\",
    \"expected_actions\": $TOTAL_ACTIONS,
    \"cfgs_found\": $CFGS_FOUND,
    \"completeness_pct\": $PCT,
    \"details\": $CFG_DETAILS,
    \"note\": \"Ruby CFG extraction is Phase 2 (not yet implemented). A score of 0 is the current baseline.\"
}
print(json.dumps(d, indent=2))
")

print_scorecard "CFG Extraction Completeness" "$CFGS_FOUND" "$TOTAL_ACTIONS" "$DETAILS"
save_results "cfg_completeness" "$DETAILS"

# Exit code: 0 if perfect score, 1 otherwise (tolerate 0 for baseline)
if [ "$CFGS_FOUND" -eq "$TOTAL_ACTIONS" ]; then
  exit 0
elif [ "$CFGS_FOUND" -eq 0 ]; then
  echo "NOTE: 0 CFGs found is expected — Ruby CFG extraction is Phase 2 (not yet implemented)."
  exit 0
else
  exit 1
fi
