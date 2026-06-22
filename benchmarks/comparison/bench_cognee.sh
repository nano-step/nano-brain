#!/usr/bin/env bash
# Cognee benchmark.
# Imports nano-brain workspace documents into Cognee, then queries each
# standardized query. Measures latency, calculates P@5, MRR, recall.
# Outputs results/cognee.json.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/helpers.sh"

RESULTS_DIR="$SCRIPT_DIR/results"
mkdir -p "$RESULTS_DIR"

VENV_DIR="$SCRIPT_DIR/.venv"
if [ ! -d "$VENV_DIR" ]; then
  echo "ERROR: Python venv not found. Run ./setup.sh first."
  exit 1
fi
source "$VENV_DIR/bin/activate"

echo "==> Cognee Benchmark"
echo ""

COGNEE_SCRIPT=$(cat <<'PYEOF'
import json
import sys
import time
import os

try:
    import cognee
except ImportError:
    print(json.dumps({"error": "cognee not installed. Run: pip install cognee"}))
    sys.exit(1)

EXPORT_DIR = "/tmp/nb-comparison-export"
results = {"tool": "cognee", "runs": []}

try:
    cognee.config.set(
        llm_provider="openai",
        embedding_provider="openai",
    )
except Exception as e:
    print(json.dumps({"error": f"Cognee config failed: {e}"}))
    sys.exit(1)

for ws_name in os.listdir(EXPORT_DIR):
    ws_dir = os.path.join(EXPORT_DIR, ws_name)
    if not os.path.isdir(ws_dir):
        continue

    print(f"  Ingesting {ws_name} into Cognee...", file=sys.stderr)
    try:
        cognee.delete_all_data()
    except Exception:
        pass

    doc_count = 0
    for fname in sorted(os.listdir(ws_dir)):
        if not fname.endswith(".txt"):
            continue
        fpath = os.path.join(ws_dir, fname)
        with open(fpath) as f:
            content = f.read()
        try:
            cognee.add(content, metadata={"source": fname, "workspace": ws_name})
            doc_count += 1
        except Exception as e:
            print(f"    Warning: failed to add {fname}: {e}", file=sys.stderr)

    try:
        cognee.cognify()
    except Exception as e:
        print(f"    Warning: cognify failed: {e}", file=sys.stderr)

    print(f"  Ingested {doc_count} documents for {ws_name}", file=sys.stderr)

    queries_file = os.environ.get("QUERIES_FILE", "queries.json")
    with open(queries_file) as f:
        qdata = json.load(f)

    for q in qdata.get("queries", []):
        query_text = q["query"]
        query_id = q["id"]
        category = q["category"]
        expect = q["expect"]

        start_ms = int(time.time() * 1000)
        try:
            search_results = cognee.search(query_text)
            if isinstance(search_results, list):
                snippets = [str(r) for r in search_results[:5]]
            elif isinstance(search_results, dict):
                snippets = [str(r) for r in search_results.get("results", [])[:5]]
            else:
                snippets = [str(search_results)]
        except Exception:
            snippets = []
        end_ms = int(time.time() * 1000)
        latency_ms = end_ms - start_ms

        all_text = " ".join(snippets).lower()
        matches = sum(1 for term in expect if term.lower() in all_text)
        p5 = round(matches / len(expect), 3) if expect else 0

        mrr = 0
        for i, snippet in enumerate(snippets):
            if any(term.lower() in snippet.lower() for term in expect):
                mrr = round(1.0 / (i + 1), 3)
                break

        recall = p5

        results["runs"].append({
            "workspace": ws_name,
            "query_id": query_id,
            "query": query_text,
            "category": category,
            "latency_ms": latency_ms,
            "results_count": len(snippets),
            "p_at_5": p5,
            "mrr": mrr,
            "recall": recall,
            "snippets": snippets[:3]
        })

        print(f"    {query_id}: P@5={p5} MRR={mrr} recall={recall} {latency_ms}ms", file=sys.stderr)

runs = results["runs"]
n = len(runs) if runs else 1
results["summary"] = {
    "tool": "cognee",
    "total_queries": len(runs),
    "avg_p_at_5": round(sum(r["p_at_5"] for r in runs) / n, 3),
    "avg_mrr": round(sum(r["mrr"] for r in runs) / n, 3),
    "avg_recall": round(sum(r["recall"] for r in runs) / n, 3),
    "avg_latency_ms": round(sum(r["latency_ms"] for r in runs) / n),
    "total_latency_ms": sum(r["latency_ms"] for r in runs)
}

print(json.dumps(results, indent=2))
PYEOF
)

echo "  Running Cognee benchmark..."
RESULT=$(echo "$COGNEE_SCRIPT" | QUERIES_FILE="$QUERIES_FILE" python3 2>/dev/null || echo '{"error": "Cognee benchmark failed"}')

print_scorecard "Cognee" "$(echo "$RESULT" | python3 -c "import json,sys; print(json.dumps(json.load(sys.stdin).get('summary', {}), indent=2))" 2>/dev/null || echo '{}')"
save_results "cognee" "$RESULT"
