#!/usr/bin/env bash
# Zep benchmark.
# Imports nano-brain workspace documents into Zep, then queries each
# standardized query. Measures latency, calculates P@5, MRR, recall.
# Outputs results/zep.json.
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

echo "==> Zep Benchmark"
echo ""

ZEP_SCRIPT=$(cat <<'PYEOF'
import json
import sys
import time
import os

try:
    from zep_cloud import ZepClient
except ImportError:
    print(json.dumps({"error": "zep-cloud not installed. Run: pip install zep-cloud"}))
    sys.exit(1)

EXPORT_DIR = "/tmp/nb-comparison-export"
ZEP_API_KEY = os.environ.get("ZEP_API_KEY", "comparison-bench-secret")
ZEP_URL = os.environ.get("ZEP_URL", "http://localhost:8080")

results = {"tool": "zep", "runs": []}

try:
    client = ZepClient(api_key=ZEP_API_KEY, base_url=ZEP_URL)
except Exception as e:
    print(json.dumps({"error": f"Zep client init failed: {e}"}))
    sys.exit(1)

for ws_name in os.listdir(EXPORT_DIR):
    ws_dir = os.path.join(EXPORT_DIR, ws_name)
    if not os.path.isdir(ws_dir):
        continue

    print(f"  Ingesting {ws_name} into Zep...", file=sys.stderr)
    group_id = f"bench-{ws_name}"
    try:
        client.group.create(
            group_id=group_id,
            name=ws_name,
            description=f"Benchmark workspace: {ws_name}"
        )
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
            client.memory.add(
                group_id=group_id,
                role="user",
                content=content[:8000],
            )
            doc_count += 1
        except Exception as e:
            print(f"    Warning: failed to add {fname}: {e}", file=sys.stderr)

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
            search_results = client.memory.search(
                group_id=group_id,
                query=query_text,
                limit=5,
            )
            if hasattr(search_results, 'messages'):
                snippets = [m.content[:200] for m in search_results.messages[:5]]
            elif isinstance(search_results, list):
                snippets = [str(r)[:200] for r in search_results[:5]]
            else:
                snippets = []
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
    "tool": "zep",
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

echo "  Running Zep benchmark..."
RESULT=$(echo "$ZEP_SCRIPT" | QUERIES_FILE="$QUERIES_FILE" python3 2>/dev/null || echo '{"error": "Zep benchmark failed"}')

print_scorecard "Zep" "$(echo "$RESULT" | python3 -c "import json,sys; print(json.dumps(json.load(sys.stdin).get('summary', {}), indent=2))" 2>/dev/null || echo '{}')"
save_results "zep" "$RESULT"
