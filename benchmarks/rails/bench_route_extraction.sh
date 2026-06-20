#!/usr/bin/env bash
# Route Extraction Accuracy Benchmark
#
# Measures how accurately nano-brain extracts HTTP routes from a Rails
# config/routes.rb. The benchmark:
#   1. Resolves the registered fixture workspace
#   2. Queries all HTTP edges via the flow/list-endpoints API
#   3. Counts total extracted routes
#   4. Verifies that specific expected routes (with correct controller handlers)
#      are present
#   5. Reports accuracy = found_routes / expected_routes
#
# Usage:
#   ./bench_route_extraction.sh
#   NANO_BRAIN_URL=http://localhost:3199 ./bench_route_extraction.sh
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

# ---- Expected routes (source -> target) ----
# These are the canonical routes from fixtures/config/routes.rb that MUST
# be correctly extracted. Redirect routes (/app -> redirect) are expected
# to be skipped.
EXPECTED_ROUTES=$(
cat <<'ROUTES'
GET /|HomeController#index
GET /story_statuses|StoryStatusesController#index
POST /story_statuses|StoryStatusesController#create
GET /story_statuses/new|StoryStatusesController#new
GET /story_statuses/:id/edit|StoryStatusesController#edit
GET /story_statuses/:id|StoryStatusesController#show
PATCH /story_statuses/:id|StoryStatusesController#update
DELETE /story_statuses/:id|StoryStatusesController#destroy
POST /api/v1/signup|Api::V1::TokensController#signup
GET /api/v1/payments/upcoming-month|Api::V1::PaymentsController#upcoming_month
GET /api/v1/moments|Api::V1::MomentsController#index
POST /api/v1/moments|Api::V1::MomentsController#create
GET /api/v1/payments|Api::V1::PaymentsController#index
GET /api/v1/payments/billing|Api::V1::PaymentsController#billing
GET /users|UsersController#index
GET /users/token_check|UsersController#token_check
POST /users/sign_in|UsersController#create
DELETE /users/sign_out|UsersController#destroy
ROUTES
)

TOTAL_EXPECTED=$(echo "$EXPECTED_ROUTES" | wc -l)

# ---- Fetch actual HTTP endpoints ----
echo ""
echo "==> Querying HTTP endpoints..."
ENDPOINTS_JSON=$(api_get "/api/v1/graph/flow/list-endpoints?workspace=$WS")
TOTAL_EXTRACTED=$(echo "$ENDPOINTS_JSON" | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin)
    print(len(d.get("endpoints",[])))
except:
    print(0)
')

echo "Total extracted HTTP edges: $TOTAL_EXTRACTED"
echo ""

# ---- Build lookup set: "SOURCE|TARGET" from actual endpoints ----
LOOKUP=$(echo "$ENDPOINTS_JSON" | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin)
    for ep in d.get("endpoints",[]):
        s = ep.get("source","")
        t = ep.get("target","")
        print("%s|%s" % (s,t))
except:
    pass
')

# ---- Check each expected route ----
FOUND=0
MISSING=""
while IFS='|' read -r source target; do
  [ -z "$source" ] && continue
  entry="$source|$target"
  if echo "$LOOKUP" | grep -Fqx "$entry"; then
    FOUND=$((FOUND + 1))
  else
    MISSING="$MISSING  $source  ->  $target"$'\n'
  fi
done <<< "$EXPECTED_ROUTES"

# ---- Check for any unexpected redirect routes that should have been skipped ----
REDIRECT_FOUND=$(echo "$LOOKUP" | grep -c "redirect" 2>/dev/null || echo "0")

# ---- Report ----
DETAILS=$(python3 -c "
import json
d = {
    \"benchmark\": \"route_extraction\",
    \"workspace\": \"$WS\",
    \"total_extracted_edges\": $TOTAL_EXTRACTED,
    \"expected_routes\": $TOTAL_EXPECTED,
    \"found_routes\": $FOUND,
    \"redirect_errors\": $REDIRECT_FOUND,
    \"missing_routes\": $(echo "$MISSING" | python3 -c 'import json,sys;print(json.dumps([l.strip() for l in sys.stdin if l.strip()]))'),
    \"accuracy\": round($FOUND / $TOTAL_EXPECTED, 4) if $TOTAL_EXPECTED > 0 else 0
}
print(json.dumps(d, indent=2))
")

print_scorecard "Route Extraction Accuracy" "$FOUND" "$TOTAL_EXPECTED" "$DETAILS"
save_results "route_extraction" "$DETAILS"

# Exit code: 0 if perfect score, 1 otherwise
if [ "$FOUND" -eq "$TOTAL_EXPECTED" ] && [ "$REDIRECT_FOUND" -eq 0 ]; then
  exit 0
else
  exit 1
fi
