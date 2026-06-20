#!/usr/bin/env bash
# Shared helper functions for Rails benchmarks.
# Source this from any benchmark script: source "$(dirname "$0")/helpers.sh"
set -euo pipefail

# --- Configuration defaults ---
SERVER_URL="${NANO_BRAIN_URL:-http://localhost:3199}"
RESULTS_DIR="${RESULTS_DIR:-$(dirname "$0")/results}"
FIXTURES_DIR="$(cd "$(dirname "$0")/fixtures" && pwd)"

# --- API helpers ---

# api_get <endpoint> [args...]
# Calls GET on the server endpoint with JSON body (if args provided).
api_get() {
  local endpoint="$1"; shift
  if [ $# -gt 0 ]; then
    curl -sf -X GET "$SERVER_URL$endpoint" -H 'Content-Type: application/json' -d "$*" 2>/dev/null || echo '{}'
  else
    curl -sf "$SERVER_URL$endpoint" 2>/dev/null || echo '{}'
  fi
}

# api_post <endpoint> <json-body>
api_post() {
  local endpoint="$1"; shift
  curl -sf -X POST "$SERVER_URL$endpoint" -H 'Content-Type: application/json' -d "$*" 2>/dev/null || echo '{}'
}

# api_post_raw <endpoint> <json-body>
# Returns raw curl output (including HTTP errors).
api_post_raw() {
  local endpoint="$1"; shift
  curl -s -X POST "$SERVER_URL$endpoint" -H 'Content-Type: application/json' -d "$*" 2>/dev/null
}

# resolve_workspace <path>
# Returns the workspace hash for a registered path.
resolve_workspace() {
  local path="$1"
  api_post "/api/v1/workspaces/resolve" "{\"path\":\"$path\"}" | python3 -c 'import json,sys;print(json.load(sys.stdin).get("workspace_hash",""))'
}

# server_healthy
# Returns 0 if the server is reachable.
server_healthy() {
  curl -sf -m 3 "$SERVER_URL/api/v1/health" >/dev/null 2>&1 || \
    curl -s -m 3 "$SERVER_URL/api/v1/health" 2>/dev/null | grep -q workspace_required
}

# server_queue_empty
# Returns 0 if the embedding queue is drained.
server_queue_empty() {
  local depth
  depth=$(curl -s -m 5 "$SERVER_URL/api/status" 2>/dev/null | python3 -c 'import json,sys;print(json.load(sys.stdin).get("queue_pending",1))' 2>/dev/null || echo "1")
  [ "$depth" = "0" ]
}

# --- HTTP endpoint helpers ---

# count_http_edges <workspace>
# Returns the number of HTTP edges in the graph.
count_http_edges() {
  local ws="$1"
  api_get "/api/v1/graph/flow/list-endpoints?workspace=$ws" | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin)
    endpoints=d.get("endpoints",[])
    print(len(endpoints))
except:
    print(0)
'
}

# list_http_endpoints <workspace>
# Returns a newline-separated list of HTTP source nodes.
list_http_endpoints() {
  local ws="$1"
  api_get "/api/v1/graph/flow/list-endpoints?workspace=$ws" | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin)
    for ep in d.get("endpoints",[]):
        print(ep.get("source",""))
except:
    pass
'
}

# flow_for_entry <workspace> <entry>
# Returns a JSON blob with the flow result for the given entry.
flow_for_entry() {
  local ws="$1"; local entry="$2"
  api_post "/api/v1/graph/flow" "{\"workspace\":\"$ws\",\"entry\":\"$entry\",\"max_depth\":8,\"format\":\"json\""
}

# flowchart_for_entry <workspace> <entry>
# Returns the control-flow graph for the handler matching the given entry.
flowchart_for_entry() {
  local ws="$1"; local entry="$2"
  api_post "/api/v1/graph/flowchart" "{\"workspace\":\"$ws\",\"entry\":\"$entry\""
}

# count_cfgs <workspace>
# Returns the number of stored CFGs (function flowcharts).
count_cfgs() {
  local ws="$1"
  api_get "/api/v1/graph/flowchart?workspace=$ws" | python3 -c '
import json,sys
try:
    d=json.load(sys.stdin)
    print(len(d))
except:
    print(0)
' 2>/dev/null || echo "0"
}

# --- Output helpers ---

# print_scorecard <bench_name> <score> <total> <details_json>
print_scorecard() {
  local name="$1" score="$2" total="$3" details="$4"
  local pct
  pct=$(python3 -c "print(round($score/$total*100,1) if $total>0 else 0)" 2>/dev/null || echo "0")
  echo ""
  echo "========================================="
  echo "  BENCHMARK: $name"
  echo "  Score:     $score / $total ($pct%)"
  echo "========================================="
  echo "$details" | python3 -m json.tool 2>/dev/null || echo "$details"
  echo ""
}

# save_results <bench_name> <json_object>
save_results() {
  local name="$1" json="$2"
  mkdir -p "$RESULTS_DIR"
  local file="$RESULTS_DIR/${name}_$(date +%Y%m%d_%H%M%S).json"
  echo "$json" > "$file"
  echo "Results saved to: $file"
}
