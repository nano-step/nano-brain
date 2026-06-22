#!/usr/bin/env bash
# Shared helper functions for comparison benchmarks.
# Source this from any benchmark script: source "$(dirname "$0")/helpers.sh"
set -euo pipefail

# --- Configuration defaults ---
NANO_BRAIN_URL="${NANO_BRAIN_URL:-http://localhost:3199}"
MCP_ENDPOINT="${NANO_BRAIN_URL}/mcp"
RESULTS_DIR="${RESULTS_DIR:-$(dirname "$0")/results}"
QUERIES_FILE="${QUERIES_FILE:-$(dirname "$0")/queries.json}"

# --- Nano-brain MCP helpers ---

# query_nano_brain <workspace> <query> [max_results]
# Calls nano-brain MCP memory_search and returns JSON results.
query_nano_brain() {
  local workspace="$1"
  local query="$2"
  local max_results="${3:-5}"

  local escaped_query
  escaped_query=$(python3 -c "import json; print(json.dumps('$query'))" 2>/dev/null || echo "\"$query\"")

  local response
  response=$(curl -s -X POST "$MCP_ENDPOINT" \
    -H "Content-Type: application/json" \
    -d "{
      \"jsonrpc\": \"2.0\",
      \"id\": 1,
      \"method\": \"tools/call\",
      \"params\": {
        \"name\": \"memory_search\",
        \"arguments\": {
          \"workspace\": \"$workspace\",
          \"query\": $escaped_query,
          \"max_results\": $max_results
        }
      }
    }" 2>/dev/null || echo '{"error": "request failed"}')

  echo "$response" | grep '^data:' | sed 's/^data: //' | head -1
}

# query_nano_brain_hybrid <workspace> <query> [max_results]
# Calls nano-brain MCP memory_query (hybrid BM25+vector) and returns JSON results.
query_nano_brain_hybrid() {
  local workspace="$1"
  local query="$2"
  local max_results="${3:-5}"

  local escaped_query
  escaped_query=$(python3 -c "import json; print(json.dumps('$query'))" 2>/dev/null || echo "\"$query\"")

  local response
  response=$(curl -s -X POST "$MCP_ENDPOINT" \
    -H "Content-Type: application/json" \
    -d "{
      \"jsonrpc\": \"2.0\",
      \"id\": 1,
      \"method\": \"tools/call\",
      \"params\": {
        \"name\": \"memory_query\",
        \"arguments\": {
          \"workspace\": \"$workspace\",
          \"query\": $escaped_query,
          \"max_results\": $max_results
        }
      }
    }" 2>/dev/null || echo '{"error": "request failed"}')

  echo "$response" | grep '^data:' | sed 's/^data: //' | head -1
}

# --- Result parsing helpers ---

# parse_nano_brain_result <json>
# Extracts the results array from a nano-brain MCP response.
parse_nano_brain_result() {
  local json="$1"
  echo "$json" | python3 -c "
import json, sys
try:
    d = json.load(sys.stdin)
    content = d.get('result', {}).get('content', [{}])[0].get('text', '{}')
    r = json.loads(content)
    print(json.dumps(r.get('results', [])))
except:
    print('[]')
" 2>/dev/null || echo "[]"
}

# extract_snippets <json>
# Extracts snippet text from parsed results array.
extract_snippets() {
  local json="$1"
  echo "$json" | python3 -c "
import json, sys
try:
    results = json.load(sys.stdin)
    snippets = [item.get('snippet', '') for item in results]
    print(json.dumps(snippets))
except:
    print('[]')
" 2>/dev/null || echo "[]"
}

# count_results <json>
# Returns the number of results from a parsed results array.
count_results() {
  local json="$1"
  echo "$json" | python3 -c "
import json, sys
try:
    results = json.load(sys.stdin)
    print(len(results))
except:
    print(0)
" 2>/dev/null || echo "0"
}

# --- Timing helpers ---

# measure_latency <command>
# Runs a command and returns elapsed time in milliseconds.
measure_latency() {
  local start_ns
  start_ns=$(date +%s%N 2>/dev/null || python3 -c 'import time; print(int(time.time()*1000000000))')
  eval "$@" >/dev/null 2>&1
  local end_ns
  end_ns=$(date +%s%N 2>/dev/null || python3 -c 'import time; print(int(time.time()*1000000000))')
  local elapsed_ms=$(( (end_ns - start_ns) / 1000000 ))
  echo "$elapsed_ms"
}

# measure_latency_ms
# Returns current time in milliseconds (for manual timing).
now_ms() {
  python3 -c 'import time; print(int(time.time()*1000))' 2>/dev/null || echo "0"
}

# --- Scoring helpers ---

# calculate_p_at_5 <expected_terms_json> <actual_snippets_json>
# Calculates Precision@5: fraction of expected terms found in top-5 snippets.
calculate_p_at_5() {
  local expected_json="$1"
  local snippets_json="$2"
  python3 -c "
import json, sys
expected = json.loads(sys.argv[1])
snippets = json.loads(sys.argv[2])
all_text = ' '.join(snippets).lower()
matches = sum(1 for term in expected if term.lower() in all_text)
result = matches / len(expected) if expected else 0
print(round(result, 3))
" "$expected_json" "$snippets_json" 2>/dev/null || echo "0"
}

# calculate_mrr <expected_terms_json> <actual_snippets_json>
# Calculates Mean Reciprocal Rank: 1/rank of first relevant result.
calculate_mrr() {
  local expected_json="$1"
  local snippets_json="$2"
  python3 -c "
import json, sys
expected = json.loads(sys.argv[1])
snippets = json.loads(sys.argv[2])
for i, snippet in enumerate(snippets):
    text = snippet.lower()
    if any(term.lower() in text for term in expected):
        print(round(1.0 / (i + 1), 3))
        sys.exit(0)
print(0)
" "$expected_json" "$snippets_json" 2>/dev/null || echo "0"
}

# calculate_recall <expected_terms_json> <actual_snippets_json>
# Calculates recall: fraction of expected terms found across all results.
calculate_recall() {
  local expected_json="$1"
  local snippets_json="$2"
  python3 -c "
import json, sys
expected = json.loads(sys.argv[1])
snippets = json.loads(sys.argv[2])
all_text = ' '.join(snippets).lower()
matches = sum(1 for term in expected if term.lower() in all_text)
result = matches / len(expected) if expected else 0
print(round(result, 3))
" "$expected_json" "$snippets_json" 2>/dev/null || echo "0"
}

# --- Output helpers ---

# log_result <tool> <query_id> <query> <result_json>
# Appends a result row to the results file.
log_result() {
  local tool="$1"
  local query_id="$2"
  local query="$3"
  local result_json="$4"
  mkdir -p "$RESULTS_DIR"
  local file="$RESULTS_DIR/${tool}_$(date +%Y%m%d_%H%M%S).jsonl"
  echo "{\"tool\":\"$tool\",\"query_id\":\"$query_id\",\"query\":\"$query\",\"ts\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"data\":$result_json}" >> "$file"
}

# print_scorecard <bench_name> <summary_json>
# Prints a formatted summary scorecard.
print_scorecard() {
  local name="$1"
  local summary="$2"
  echo ""
  echo "========================================="
  echo "  BENCHMARK: $name"
  echo "========================================="
  echo "$summary" | python3 -m json.tool 2>/dev/null || echo "$summary"
  echo ""
}

# save_results <tool> <json_object>
# Saves final JSON results to the results directory.
save_results() {
  local tool="$1"
  local json="$2"
  mkdir -p "$RESULTS_DIR"
  local file="$RESULTS_DIR/${tool}.json"
  echo "$json" > "$file"
  echo "Results saved to: $file"
}

# --- Server helpers ---

# server_healthy [url]
# Returns 0 if the nano-brain server is reachable.
server_healthy() {
  local url="${1:-$NANO_BRAIN_URL}"
  curl -sf -m 3 "$url/api/v1/health" >/dev/null 2>&1 || \
    curl -s -m 3 "$url/api/v1/health" 2>/dev/null | grep -q workspace_required
}

# server_queue_empty [url]
# Returns 0 if the embedding queue is drained.
server_queue_empty() {
  local url="${1:-$NANO_BRAIN_URL}"
  local depth
  depth=$(curl -s -m 5 "$url/api/status" 2>/dev/null | python3 -c 'import json,sys;print(json.load(sys.stdin).get("queue_pending",1))' 2>/dev/null || echo "1")
  [ "$depth" = "0" ]
}

# --- Workspace helpers ---

# get_workspace <workspace_name>
# Returns workspace hash from queries.json for a named workspace.
get_workspace() {
  local name="$1"
  python3 -c "
import json
data = json.load(open('$QUERIES_FILE'))
print(data.get('workspaces', {}).get('$name', ''))
" 2>/dev/null || echo ""
}

# list_workspaces
# Returns space-separated workspace names from queries.json.
list_workspaces() {
  python3 -c "
import json
data = json.load(open('$QUERIES_FILE'))
# New format: queries_by_workspace keys
for name in data.get('queries_by_workspace', {}):
    print(name)
# Fallback: old format workspaces keys
if not data.get('queries_by_workspace'):
    for name in data.get('workspaces', {}):
        print(name)
" 2>/dev/null || echo ""
}

# list_queries <workspace_name>
# Returns JSON lines of queries for a specific workspace.
list_queries() {
  local ws_name="$1"
  python3 -c "
import json
data = json.load(open('$QUERIES_FILE'))
if 'queries_by_workspace' in data and '$ws_name' in data['queries_by_workspace']:
    for q in data['queries_by_workspace']['$ws_name']:
        print(json.dumps(q))
elif 'queries' in data:
    for q in data['queries']:
        print(json.dumps(q))
" 2>/dev/null || echo ""
}

# list_queries <workspace_name>
# Returns queries for a specific workspace from queries.json.
# New format: queries_by_workspace[workspace]
# Fallback: old format queries[]
list_queries() {
  local ws_name="$1"
  python3 -c "
import json
data = json.load(open('$QUERIES_FILE'))
# New format
if 'queries_by_workspace' in data and '$ws_name' in data['queries_by_workspace']:
    for q in data['queries_by_workspace']['$ws_name']:
        print(json.dumps(q))
# Fallback: old format
elif 'queries' in data:
    for q in data['queries']:
        print(json.dumps(q))
" 2>/dev/null || echo ""
}

# --- Document export helpers ---

# export_workspace_docs <workspace> <output_dir>
# Exports all documents from a nano-brain workspace as individual text files.
# Usage: export_workspace_docs <workspace_hash> /tmp/nb-export
export_workspace_docs() {
  local workspace="$1"
  local output_dir="$2"
  mkdir -p "$output_dir"

  python3 -c "
import json, urllib.request, os, sys

workspace = sys.argv[1]
output_dir = sys.argv[2]
server_url = sys.argv[3]

# Get document list via search with broad query
req = urllib.request.Request(
    f'{server_url}/api/v1/search',
    data=json.dumps({'workspace': workspace, 'query': '*', 'max_results': 1000}).encode(),
    headers={'Content-Type': 'application/json'}
)
try:
    with urllib.request.urlopen(req, timeout=30) as resp:
        data = json.loads(resp.read())
        docs = data.get('results', [])
        for doc in docs:
            doc_id = doc.get('id', 'unknown')
            title = doc.get('title', 'untitled')
            source = doc.get('source_path', 'memory://unknown')
            snippet = doc.get('snippet', '')
            filename = f'{doc_id}.txt'
            filepath = os.path.join(output_dir, filename)
            with open(filepath, 'w') as f:
                f.write(f'Title: {title}\nSource: {source}\n\n{snippet}\n')
        print(f'Exported {len(docs)} documents')
except Exception as e:
    print(f'Export failed: {e}', file=sys.stderr)
    sys.exit(1)
" "$workspace" "$output_dir" "$NANO_BRAIN_URL" 2>/dev/null || echo "Export failed"
}
