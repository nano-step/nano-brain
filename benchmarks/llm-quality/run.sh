#!/bin/bash
set -e

SERVER_URL="${NANO_BRAIN_URL:-http://localhost:3199}"
QUERIES_FILE="${1:-benchmarks/llm-quality/queries.json}"
RESULTS_FILE="${2:-benchmarks/llm-quality/results_$(date +%Y%m%d_%H%M%S).json}"
export RESULTS_FILE

WORKSPACE=$(python3 -c "import json; print(json.load(open('$QUERIES_FILE'))['workspace'])")

mkdir -p "$(dirname "$RESULTS_FILE")"
echo '{"runs":[' > "$RESULTS_FILE"

FIRST=true
TOTAL_QUERIES=$(python3 -c "import json; print(len(json.load(open('$QUERIES_FILE'))['queries']))")

for i in $(seq 0 $((TOTAL_QUERIES - 1))); do
  QUERY=$(python3 -c "import json; print(json.load(open('$QUERIES_FILE'))['queries'][$i]['query'])")
  ID=$(python3 -c "import json; print(json.load(open('$QUERIES_FILE'))['queries'][$i]['id'])")
  CATEGORY=$(python3 -c "import json; print(json.load(open('$QUERIES_FILE'))['queries'][$i]['category'])")
  EXPECT=$(python3 -c "import json; print(json.dumps(json.load(open('$QUERIES_FILE'))['queries'][$i]['expect']))")
  
  echo "[$((i+1))/$TOTAL_QUERIES] $QUERY"
  echo "  EXPECT: $EXPECT"
  
  START=$(date +%s%N)
  
  SEARCH_RESULT=$(curl -s -X POST "$SERVER_URL/mcp" \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": 1,
      "method": "tools/call",
      "params": {
        "name": "memory_search",
        "arguments": {
          "workspace": "'$WORKSPACE'",
          "query": "'"$QUERY"'",
          "max_results": 5
        }
      }
    }' 2>/dev/null | grep '^data:' | sed 's/^data: //' || echo '{"error": "request failed"}')
  
  END=$(date +%s%N)
  LATENCY_MS=$(( (END - START) / 1000000 ))
  
  RESULTS_COUNT=$(echo "$SEARCH_RESULT" | python3 -c "
import json, sys
try:
    d = json.load(sys.stdin)
    content = d.get('result', {}).get('content', [{}])[0].get('text', '{}')
    r = json.loads(content)
    print(len(r.get('results', [])))
except:
    print(0)" 2>/dev/null || echo "0")
  
  SNIPPETS=$(echo "$SEARCH_RESULT" | python3 -c "
import json, sys
try:
    d = json.load(sys.stdin)
    content = d.get('result', {}).get('content', [{}])[0].get('text', '{}')
    r = json.loads(content)
    snippets = [item.get('snippet', '')[:200] for item in r.get('results', [])[:3]]
    print(json.dumps(snippets))
except:
    print('[]')" 2>/dev/null || echo "[]")
  
  RELEVANCE=$(echo "$SEARCH_RESULT" | python3 -c "
import json, sys
try:
    d = json.load(sys.stdin)
    content = d.get('result', {}).get('content', [{}])[0].get('text', '{}')
    r = json.loads(content)
    all_text = ' '.join([item.get('snippet', '') + ' ' + item.get('title', '') for item in r.get('results', [])]).lower()
    expect = json.loads(sys.argv[1])
    matches = sum(1 for term in expect if term.lower() in all_text)
    result = matches / len(expect) if expect else 0
    print(result)
except Exception as e:
    import traceback
    traceback.print_exc(file=sys.stderr)
    print(0)" "$EXPECT" 2>/dev/null || echo "0")
  
  if [ "$FIRST" = true ]; then
    FIRST=false
  else
    echo "," >> "$RESULTS_FILE"
  fi
  
  python3 -c "
import json
result = {
    'id': '$ID',
    'query': '$QUERY',
    'category': '$CATEGORY',
    'latency_ms': $LATENCY_MS,
    'results_count': $RESULTS_COUNT,
    'relevance_score': $RELEVANCE,
    'snippets': $SNIPPETS
}
print(json.dumps(result, indent=2))" >> "$RESULTS_FILE"
  
  echo "  Results: $RESULTS_COUNT | Relevance: $RELEVANCE | Latency: ${LATENCY_MS}ms"
done

echo ']}' >> "$RESULTS_FILE"

python3 -c "
import json, os

results_file = os.environ.get('RESULTS_FILE', 'benchmarks/llm-quality/results.json')
with open(results_file) as f:
    data = json.load(f)

runs = data['runs']

avg_latency = sum(r['latency_ms'] for r in runs) / len(runs)
avg_relevance = sum(r['relevance_score'] for r in runs) / len(runs)
total_results = sum(r['results_count'] for r in runs)

data['summary'] = {
    'total_queries': len(runs),
    'avg_latency_ms': round(avg_latency, 2),
    'avg_relevance': round(avg_relevance, 3),
    'total_results': total_results
}

with open(results_file, 'w') as f:
    json.dump(data, f, indent=2)

print()
print('=== Summary ===')
print(f'Queries: {len(runs)}')
print(f'Avg Latency: {avg_latency:.0f}ms')
print(f'Avg Relevance: {avg_relevance:.3f}')
print(f'Total Results: {total_results}')
"

echo
echo "Results saved to: $RESULTS_FILE"
