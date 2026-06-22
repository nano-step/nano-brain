#!/usr/bin/env bash
# nano-brain baseline benchmark.
# Queries nano-brain MCP for each standardized query, measures latency,
# calculates P@5, MRR, and recall. Outputs results/nanobrain.json.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

RESULTS_DIR="$SCRIPT_DIR/results"
mkdir -p "$RESULTS_DIR"

echo "==> nano-brain Baseline Benchmark"
echo "    Server: $NANO_BRAIN_URL"
echo ""

if ! server_healthy; then
  echo "ERROR: nano-brain server not running on $NANO_BRAIN_URL"
  echo "Start with: ./setup.sh"
  exit 1
fi

ALL_P5=0
ALL_MRR=0
ALL_RECALL=0
ALL_LATENCY=0
QUERY_COUNT=0

RESULTS_JSON='{"tool":"nanobrain","runs":['
FIRST=true

for ws_name in $(list_workspaces); do
  ws_hash=$(get_workspace "$ws_name")
  if [ -z "$ws_hash" ]; then
    echo "    Skipping $ws_name: no workspace hash"
    continue
  fi

  echo "  Workspace: $ws_name ($ws_hash)"

  TOTAL_QUERIES=$(python3 -c "import json; print(len(json.load(open('$QUERIES_FILE'))['queries']))")

  for i in $(seq 0 $((TOTAL_QUERIES - 1))); do
    QUERY=$(python3 -c "import json; print(json.load(open('$QUERIES_FILE'))['queries'][$i]['query'])")
    ID=$(python3 -c "import json; print(json.load(open('$QUERIES_FILE'))['queries'][$i]['id'])")
    CATEGORY=$(python3 -c "import json; print(json.load(open('$QUERIES_FILE'))['queries'][$i]['category'])")
    EXPECT=$(python3 -c "import json; print(json.dumps(json.load(open('$QUERIES_FILE'))['queries'][$i]['expect']))")

    echo -n "    [$((i+1))/$TOTAL_QUERIES] $QUERY ... "

    START_MS=$(now_ms)
    RAW_RESULT=$(query_nano_brain "$ws_hash" "$QUERY" 5)
    END_MS=$(now_ms)
    LATENCY_MS=$((END_MS - START_MS))

    PARSED=$(parse_nano_brain_result "$RAW_RESULT")
    SNIPPETS=$(extract_snippets "$PARSED")
    RESULT_COUNT=$(count_results "$PARSED")

    P5=$(calculate_p_at_5 "$EXPECT" "$SNIPPETS")
    MRR=$(calculate_mrr "$EXPECT" "$SNIPPETS")
    RECALL=$(calculate_recall "$EXPECT" "$SNIPPETS")

    ALL_P5=$(python3 -c "print(round($ALL_P5 + $P5, 3))")
    ALL_MRR=$(python3 -c "print(round($ALL_MRR + $MRR, 3))")
    ALL_RECALL=$(python3 -c "print(round($ALL_RECALL + $RECALL, 3))")
    ALL_LATENCY=$((ALL_LATENCY + LATENCY_MS))
    QUERY_COUNT=$((QUERY_COUNT + 1))

    if [ "$FIRST" = true ]; then
      FIRST=false
    else
      RESULTS_JSON="$RESULTS_JSON,"
    fi

    RESULTS_JSON="$RESULTS_JSON$(python3 -c "
import json
print(json.dumps({
    'workspace': '$ws_name',
    'query_id': '$ID',
    'query': '''$QUERY''',
    'category': '$CATEGORY',
    'latency_ms': $LATENCY_MS,
    'results_count': $RESULT_COUNT,
    'p_at_5': $P5,
    'mrr': $MRR,
    'recall': $RECALL,
    'snippets': json.loads('''$SNIPPETS''')
}))
")"

    echo "P@5=$P5 MRR=$MRR recall=$RECALL ${LATENCY_MS}ms results=$RESULT_COUNT"
  done
done

AVG_P5="0"
AVG_MRR="0"
AVG_RECALL="0"
AVG_LATENCY="0"
if [ "$QUERY_COUNT" -gt 0 ]; then
  AVG_P5=$(python3 -c "print(round($ALL_P5 / $QUERY_COUNT, 3))")
  AVG_MRR=$(python3 -c "print(round($ALL_MRR / $QUERY_COUNT, 3))")
  AVG_RECALL=$(python3 -c "print(round($ALL_RECALL / $QUERY_COUNT, 3))")
  AVG_LATENCY=$((ALL_LATENCY / QUERY_COUNT))
fi

SUMMARY=$(python3 -c "
import json
print(json.dumps({
    'tool': 'nanobrain',
    'total_queries': $QUERY_COUNT,
    'avg_p_at_5': $AVG_P5,
    'avg_mrr': $AVG_MRR,
    'avg_recall': $AVG_RECALL,
    'avg_latency_ms': $AVG_LATENCY,
    'total_latency_ms': $ALL_LATENCY
}, indent=2))
")

RESULTS_JSON="$RESULTS_JSON],\"summary\":$SUMMARY}"

print_scorecard "nano-brain Baseline" "$SUMMARY"
save_results "nanobrain" "$RESULTS_JSON"
