#!/usr/bin/env bash
set -euo pipefail

# Code Intelligence Benchmark for nano-brain
# Measures symbol graph accuracy, call chain completeness, and impact analysis precision
#
# Usage: ./bench_code_intel.sh [server_url] [workspace_hash]
# Default: http://localhost:3199/mcp 7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f

SERVER_URL="${1:-http://localhost:3199/mcp}"
WORKSPACE="${2:-7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GROUND_TRUTH="$SCRIPT_DIR/ground_truth.json"
RESULTS_DIR="$SCRIPT_DIR/results"
RESULTS_FILE="$RESULTS_DIR/code_intelligence.json"

mkdir -p "$RESULTS_DIR"

command -v jq >/dev/null 2>&1 || { echo "jq is required"; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "curl is required"; exit 1; }

if [[ ! -f "$GROUND_TRUTH" ]]; then
  echo "ERROR: ground truth file not found at $GROUND_TRUTH"
  exit 1
fi

mcp_call() {
  local tool_name="$1"
  local arguments="$2"
  local request_id=$((RANDOM % 100000))

  local payload
  payload=$(jq -n \
    --argjson id "$request_id" \
    --arg name "$tool_name" \
    --argjson args "$arguments" \
    '{
      jsonrpc: "2.0",
      id: $id,
      method: "tools/call",
      params: {
        name: $name,
        arguments: $args
      }
    }')

  local response
  response=$(curl -s -X POST "$SERVER_URL" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d "$payload" \
    --max-time 30 2>/dev/null || echo '{"error":"connection failed"}')

  if echo "$response" | grep -q '^data: '; then
    response=$(echo "$response" | grep '^data: ' | tail -1 | sed 's/^data: //')
  fi

  echo "$response"
}

extract_mcp_text() {
  local response="$1"
  echo "$response" | jq -r '.result.content[0].text // empty' 2>/dev/null || echo ""
}

calc_metrics() {
  local expected="$1"
  local actual="$2"

  if [[ -z "$actual" || "$actual" == "[]" ]]; then
    echo '{"precision":0.0,"recall":0.0,"f1":0.0,"true_positives":0,"false_positives":0,"false_negatives":0}'
    return
  fi

  local tp=0 fp=0 fn=0

  while IFS= read -r item; do
    if [[ -z "$item" ]]; then continue; fi
    if echo "$expected" | jq -e --arg item "$item" 'map(select(. == $item or (. | test($item; "i")) or ($item | test(.; "i")))) | length > 0' >/dev/null 2>&1; then
      tp=$((tp + 1))
    else
      fp=$((fp + 1))
    fi
  done < <(echo "$actual" | jq -r '.[]' 2>/dev/null)

  while IFS= read -r item; do
    if [[ -z "$item" ]]; then continue; fi
    if ! echo "$actual" | jq -e --arg item "$item" 'map(select(. == $item or (. | test($item; "i")) or ($item | test(.; "i")))) | length > 0' >/dev/null 2>&1; then
      fn=$((fn + 1))
    fi
  done < <(echo "$expected" | jq -r '.[]' 2>/dev/null)

  local precision recall f1
  if [[ $((tp + fp)) -gt 0 ]]; then
    precision=$(awk "BEGIN {printf \"%.4f\", $tp / ($tp + $fp)}")
  else
    precision="0.0000"
  fi
  if [[ $((tp + fn)) -gt 0 ]]; then
    recall=$(awk "BEGIN {printf \"%.4f\", $tp / ($tp + $fn)}")
  else
    recall="0.0000"
  fi
  local sum
  sum=$(awk "BEGIN {printf \"%.4f\", $precision + $recall}")
  if awk "BEGIN {exit !($sum > 0)}"; then
    f1=$(awk "BEGIN {printf \"%.4f\", 2 * $precision * $recall / ($precision + $recall)}")
  else
    f1="0.0000"
  fi

  jq -n \
    --argjson p "$precision" \
    --argjson r "$recall" \
    --argjson f "$f1" \
    --argjson tp "$tp" \
    --argjson fp "$fp" \
    --argjson fn "$fn" \
    '{precision: $p, recall: $r, f1: $f, true_positives: $tp, false_positives: $fp, false_negatives: $fn}'
}

echo "================================================"
echo "nano-brain Code Intelligence Benchmark"
echo "================================================"
echo "Server: $SERVER_URL"
echo "Workspace: $WORKSPACE"
echo "Ground truth: $GROUND_TRUTH"
echo ""

NUM_FUNCTIONS=$(jq '.functions | length' "$GROUND_TRUTH")
echo "Functions to test: $NUM_FUNCTIONS"
echo ""

ALL_RESULTS="[]"
GRAPH_P_SUM="0"
GRAPH_R_SUM="0"
GRAPH_F1_SUM="0"
TRACE_P_SUM="0"
TRACE_R_SUM="0"
TRACE_F1_SUM="0"
IMPACT_P_SUM="0"
IMPACT_R_SUM="0"
IMPACT_F1_SUM="0"
TESTED=0

for ((i=0; i<NUM_FUNCTIONS; i++)); do
  FUNC_ID=$(jq -r ".functions[$i].id" "$GROUND_TRUTH")
  SOURCE_FILE=$(jq -r ".functions[$i].source_file" "$GROUND_TRUTH")
  FUNC_NAME=$(jq -r ".functions[$i].function_name" "$GROUND_TRUTH")
  NODE="${SOURCE_FILE}::${FUNC_NAME}"

  echo "--- [$((i+1))/$NUM_FUNCTIONS] $FUNC_ID ---"
  echo "  Node: $NODE"

  EXPECTED_CALLERS=$(jq -c ".functions[$i].expected_callers" "$GROUND_TRUTH")
  EXPECTED_CALLEES=$(jq -c ".functions[$i].expected_callees" "$GROUND_TRUTH")
  EXPECTED_IMPACT=$(jq -c ".functions[$i].expected_impact_nodes" "$GROUND_TRUTH")
  EXPECTED_FLOW=$(jq -c ".functions[$i].expected_flow_nodes" "$GROUND_TRUTH")

  echo "  Testing memory_graph (out)..."
  GRAPH_OUT_ARGS=$(jq -n --arg ws "$WORKSPACE" --arg node "$NODE" '{workspace: $ws, node: $node, direction: "out", edge_type: "calls"}')
  GRAPH_OUT_RAW=$(mcp_call "memory_graph" "$GRAPH_OUT_ARGS")
  GRAPH_OUT_TEXT=$(extract_mcp_text "$GRAPH_OUT_RAW")
  ACTUAL_CALLEES=$(echo "$GRAPH_OUT_TEXT" | jq -c '[.edges[]?.target // empty]' 2>/dev/null || echo "[]")
  GRAPH_OUT_METRICS=$(calc_metrics "$EXPECTED_CALLEES" "$ACTUAL_CALLEES")
  echo "    Callees: $(echo "$GRAPH_OUT_METRICS" | jq -r '.f1') F1"

  echo "  Testing memory_graph (in)..."
  GRAPH_IN_ARGS=$(jq -n --arg ws "$WORKSPACE" --arg node "$NODE" '{workspace: $ws, node: $node, direction: "in", edge_type: "calls"}')
  GRAPH_IN_RAW=$(mcp_call "memory_graph" "$GRAPH_IN_ARGS")
  GRAPH_IN_TEXT=$(extract_mcp_text "$GRAPH_IN_RAW")
  ACTUAL_CALLERS=$(echo "$GRAPH_IN_TEXT" | jq -c '[.edges[]?.source // empty]' 2>/dev/null || echo "[]")
  GRAPH_IN_METRICS=$(calc_metrics "$EXPECTED_CALLERS" "$ACTUAL_CALLERS")
  echo "    Callers: $(echo "$GRAPH_IN_METRICS" | jq -r '.f1') F1"

  GO=$(echo "$GRAPH_OUT_METRICS" | jq '.f1')
  GI=$(echo "$GRAPH_IN_METRICS" | jq '.f1')
  GP=$(echo "$GRAPH_OUT_METRICS" | jq '.precision')
  GPI=$(echo "$GRAPH_IN_METRICS" | jq '.precision')
  GR=$(echo "$GRAPH_OUT_METRICS" | jq '.recall')
  GRI=$(echo "$GRAPH_IN_METRICS" | jq '.recall')
  GRAPH_F1=$(awk "BEGIN {printf \"%.4f\", ($GO + $GI) / 2}")
  GRAPH_P=$(awk "BEGIN {printf \"%.4f\", ($GP + $GPI) / 2}")
  GRAPH_R=$(awk "BEGIN {printf \"%.4f\", ($GR + $GRI) / 2}")

  echo "  Testing memory_trace..."
  TRACE_ARGS=$(jq -n --arg ws "$WORKSPACE" --arg node "$NODE" '{workspace: $ws, node: $node, max_depth: 3}')
  TRACE_RAW=$(mcp_call "memory_trace" "$TRACE_ARGS")
  TRACE_TEXT=$(extract_mcp_text "$TRACE_RAW")
  ACTUAL_FLOW=$(echo "$TRACE_TEXT" | jq -c '[.chain[]?.node // empty]' 2>/dev/null || echo "[]")
  TRACE_METRICS=$(calc_metrics "$EXPECTED_FLOW" "$ACTUAL_FLOW")
  echo "    Trace: $(echo "$TRACE_METRICS" | jq -r '.f1') F1"

  echo "  Testing memory_impact..."
  IMPACT_ARGS=$(jq -n --arg ws "$WORKSPACE" --arg node "$NODE" '{workspace: $ws, node: $node, max_depth: 2}')
  IMPACT_RAW=$(mcp_call "memory_impact" "$IMPACT_ARGS")
  IMPACT_TEXT=$(extract_mcp_text "$IMPACT_RAW")
  ACTUAL_IMPACT=$(echo "$IMPACT_TEXT" | jq -c '[.impacted[]?.node // empty]' 2>/dev/null || echo "[]")
  IMPACT_METRICS=$(calc_metrics "$EXPECTED_IMPACT" "$ACTUAL_IMPACT")
  echo "    Impact: $(echo "$IMPACT_METRICS" | jq -r '.f1') F1"

  TP=$(echo "$TRACE_METRICS" | jq '.precision')
  TR=$(echo "$TRACE_METRICS" | jq '.recall')
  TF=$(echo "$TRACE_METRICS" | jq '.f1')
  IP=$(echo "$IMPACT_METRICS" | jq '.precision')
  IR=$(echo "$IMPACT_METRICS" | jq '.recall')
  IF=$(echo "$IMPACT_METRICS" | jq '.f1')

  GRAPH_P_SUM=$(awk "BEGIN {printf \"%.4f\", $GRAPH_P_SUM + $GRAPH_P}")
  GRAPH_R_SUM=$(awk "BEGIN {printf \"%.4f\", $GRAPH_R_SUM + $GRAPH_R}")
  GRAPH_F1_SUM=$(awk "BEGIN {printf \"%.4f\", $GRAPH_F1_SUM + $GRAPH_F1}")
  TRACE_P_SUM=$(awk "BEGIN {printf \"%.4f\", $TRACE_P_SUM + $TP}")
  TRACE_R_SUM=$(awk "BEGIN {printf \"%.4f\", $TRACE_R_SUM + $TR}")
  TRACE_F1_SUM=$(awk "BEGIN {printf \"%.4f\", $TRACE_F1_SUM + $TF}")
  IMPACT_P_SUM=$(awk "BEGIN {printf \"%.4f\", $IMPACT_P_SUM + $IP}")
  IMPACT_R_SUM=$(awk "BEGIN {printf \"%.4f\", $IMPACT_R_SUM + $IR}")
  IMPACT_F1_SUM=$(awk "BEGIN {printf \"%.4f\", $IMPACT_F1_SUM + $IF}")
  TESTED=$((TESTED + 1))

  PER_FUNC=$(jq -n \
    --arg id "$FUNC_ID" \
    --arg node "$NODE" \
    --argjson graph "$GRAPH_OUT_METRICS" \
    --argjson trace "$TRACE_METRICS" \
    --argjson impact "$IMPACT_METRICS" \
    --argjson actual_callees "$ACTUAL_CALLEES" \
    --argjson actual_callers "$ACTUAL_CALLERS" \
    --argjson actual_flow "$ACTUAL_FLOW" \
    --argjson actual_impact "$ACTUAL_IMPACT" \
    '{
      function_id: $id,
      node: $node,
      graph_out: $graph,
      graph_in: $graph,
      trace: $trace,
      impact: $impact,
      actual_callees: $actual_callees,
      actual_callers: $actual_callers,
      actual_flow: $actual_flow,
      actual_impact: $actual_impact
    }')

  ALL_RESULTS=$(echo "$ALL_RESULTS" | jq --argjson item "$PER_FUNC" '. + [$item]')

  echo ""
  sleep 0.5
done

if [[ $TESTED -gt 0 ]]; then
  GAP=$(awk "BEGIN {printf \"%.4f\", $GRAPH_P_SUM / $TESTED}")
  GAR=$(awk "BEGIN {printf \"%.4f\", $GRAPH_R_SUM / $TESTED}")
  GAF1=$(awk "BEGIN {printf \"%.4f\", $GRAPH_F1_SUM / $TESTED}")
  TAP=$(awk "BEGIN {printf \"%.4f\", $TRACE_P_SUM / $TESTED}")
  TAR=$(awk "BEGIN {printf \"%.4f\", $TRACE_R_SUM / $TESTED}")
  TAF1=$(awk "BEGIN {printf \"%.4f\", $TRACE_F1_SUM / $TESTED}")
  IAP=$(awk "BEGIN {printf \"%.4f\", $IMPACT_P_SUM / $TESTED}")
  IAR=$(awk "BEGIN {printf \"%.4f\", $IMPACT_R_SUM / $TESTED}")
  IAF1=$(awk "BEGIN {printf \"%.4f\", $IMPACT_F1_SUM / $TESTED}")
else
  GAP=0; GAR=0; GAF1=0
  TAP=0; TAR=0; TAF1=0
  IAP=0; IAR=0; IAF1=0
fi

RESULTS=$(jq -n \
  --arg tool "nanobrain" \
  --arg workspace "$WORKSPACE" \
  --argjson functions_tested "$TESTED" \
  --arg timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg server "$SERVER_URL" \
  --argjson gp "$GAP" --argjson gr "$GAR" --argjson gf1 "$GAF1" \
  --argjson tp "$TAP" --argjson tr "$TAR" --argjson tf1 "$TAF1" \
  --argjson ip "$IAP" --argjson ir "$IAR" --argjson if1 "$IAF1" \
  --argjson per_function "$ALL_RESULTS" \
  '{
    tool: $tool,
    workspace: $workspace,
    server_url: $server,
    timestamp: $timestamp,
    functions_tested: $functions_tested,
    graph_accuracy: {precision: $gp, recall: $gr, f1: $gf1},
    trace_accuracy: {precision: $tp, recall: $tr, f1: $tf1},
    impact_accuracy: {precision: $ip, recall: $ir, f1: $if1},
    per_function_results: $per_function
  }')

echo "$RESULTS" > "$RESULTS_FILE"

echo "================================================"
echo "BENCHMARK RESULTS"
echo "================================================"
echo "Functions tested: $TESTED"
echo ""
echo "Graph Accuracy (callers + callees):"
echo "  Precision: $GAP"
echo "  Recall:    $GAR"
echo "  F1:        $GAF1"
echo ""
echo "Trace Accuracy (call chain):"
echo "  Precision: $TAP"
echo "  Recall:    $TAR"
echo "  F1:        $TAF1"
echo ""
echo "Impact Accuracy (reverse BFS):"
echo "  Precision: $IAP"
echo "  Recall:    $IAR"
echo "  F1:        $IAF1"
echo ""
echo "Results saved to: $RESULTS_FILE"
