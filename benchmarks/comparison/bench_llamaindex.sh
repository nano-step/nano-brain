#!/usr/bin/env bash
# LlamaIndex benchmark.
# Builds a LlamaIndex vector store from nano-brain workspace documents,
# then queries each standardized query. Measures latency, P@5, MRR, recall.
# Outputs results/llamaindex.json.
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

echo "==> LlamaIndex Benchmark"
echo ""

LLAMAINDEX_SCRIPT=$(cat <<'PYEOF'
import json
import sys
import time
import os

EXPORT_DIR = "/tmp/nb-comparison-export"
results = {"tool": "llamaindex", "runs": []}

try:
    from llama_index.core import VectorStoreIndex, SimpleDirectoryReader, Settings
    from llama_index.core.node_parser import SentenceSplitter
except ImportError:
    print(json.dumps({"error": "llama-index not installed. Run: pip install llama-index-core"}))
    sys.exit(1)

Settings.chunk_size = 512
Settings.chunk_overlap = 50

for ws_name in os.listdir(EXPORT_DIR):
    ws_dir = os.path.join(EXPORT_DIR, ws_name)
    if not os.path.isdir(ws_dir):
        continue

    txt_files = [f for f in os.listdir(ws_dir) if f.endswith(".txt")]
    if not txt_files:
        continue

    print(f"  Building LlamaIndex for {ws_name} ({len(txt_files)} docs)...", file=sys.stderr)
    try:
        documents = SimpleDirectoryReader(input_dir=ws_dir, required_exts=[".txt"]).load_data()
        index = VectorStoreIndex.from_documents(documents, show_progress=False)
        query_engine = index.as_query_engine(similarity_top_k=5)
    except Exception as e:
        print(f"    Failed to build index: {e}", file=sys.stderr)
        continue

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
            response = query_engine.query(query_text)
            snippets = [node.get_text()[:200] for node in response.source_nodes[:5]] if hasattr(response, 'source_nodes') else [str(response)]
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
    "tool": "llamaindex",
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

echo "  Running LlamaIndex benchmark..."
RESULT=$(echo "$LLAMAINDEX_SCRIPT" | QUERIES_FILE="$QUERIES_FILE" python3 2>/dev/null || echo '{"error": "LlamaIndex benchmark failed"}')

print_scorecard "LlamaIndex" "$(echo "$RESULT" | python3 -c "import json,sys; print(json.dumps(json.load(sys.stdin).get('summary', {}), indent=2))" 2>/dev/null || echo '{}')"
save_results "llamaindex" "$RESULT"
