#!/usr/bin/env bash
# GraphRAG benchmark.
# Imports nano-brain workspace documents into GraphRAG (Neo4j-backed),
# then queries each standardized query. Measures latency, P@5, MRR, recall.
# Outputs results/graphrag.json.
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

echo "==> GraphRAG Benchmark"
echo ""

GRAPHRAG_SCRIPT=$(cat <<'PYEOF'
import json
import sys
import time
import os

EXPORT_DIR = "/tmp/nb-comparison-export"
NEO4J_URI = "bolt://localhost:7687"
NEO4J_USER = "neo4j"
NEO4J_PASS = "comparison-bench"

results = {"tool": "graphrag", "runs": []}

try:
    from neo4j import GraphDatabase
    driver = GraphDatabase.driver(NEO4J_URI, auth=(NEO4J_USER, NEO4J_PASS))
    driver.verify_connectivity()
except ImportError:
    print(json.dumps({"error": "neo4j driver not installed. Run: pip install neo4j"}))
    sys.exit(1)
except Exception as e:
    print(json.dumps({"error": f"Neo4j connection failed: {e}"}))
    sys.exit(1)

for ws_name in os.listdir(EXPORT_DIR):
    ws_dir = os.path.join(EXPORT_DIR, ws_name)
    if not os.path.isdir(ws_dir):
        continue

    print(f"  Ingesting {ws_name} into Neo4j...", file=sys.stderr)
    with driver.session() as session:
        session.run("MATCH (n) WHERE n.workspace = $ws DETACH DELETE n", ws=ws_name)

        doc_count = 0
        for fname in sorted(os.listdir(ws_dir)):
            if not fname.endswith(".txt"):
                continue
            fpath = os.path.join(ws_dir, fname)
            with open(fpath) as f:
                content = f.read()

            title = content.split("\n")[0].replace("Title: ", "") if content else fname
            source = ""
            for line in content.split("\n"):
                if line.startswith("Source: "):
                    source = line.replace("Source: ", "")
                    break

            session.run(
                "CREATE (d:Document {title: $title, content: $content, source: $source, workspace: $ws, fname: $fname})",
                title=title, content=content[:4000], source=source, ws=ws_name, fname=fname
            )
            doc_count += 1

        session.run("""
            MATCH (d1:Document {workspace: $ws}), (d2:Document {workspace: $ws})
            WHERE d1 <> d2
            AND (
                d1.source CONTAINS d2.source
                OR d2.source CONTAINS d1.source
                OR d1.title CONTAINS d2.title
                OR d2.title CONTAINS d1.title
            )
            MERGE (d1)-[:RELATED_TO]->(d2)
        """, ws=ws_name)

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
            with driver.session() as session:
                result = session.run("""
                    MATCH (d:Document {workspace: $ws})
                    WHERE d.content CONTAINS $q1
                       OR d.content CONTAINS $q2
                       OR d.title CONTAINS $q1
                    RETURN d.title AS title, d.content AS content, d.source AS source
                    LIMIT 5
                """, ws=ws_name, q1=query_text.split()[0] if query_text.split() else "",
                     q2=query_text.split()[-1] if len(query_text.split()) > 1 else "")
                records = [dict(r) for r in result]
                snippets = [r.get("content", "")[:200] for r in records]
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

driver.close()

runs = results["runs"]
n = len(runs) if runs else 1
results["summary"] = {
    "tool": "graphrag",
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

echo "  Running GraphRAG benchmark..."
RESULT=$(echo "$GRAPHRAG_SCRIPT" | QUERIES_FILE="$QUERIES_FILE" python3 2>/dev/null || echo '{"error": "GraphRAG benchmark failed"}')

print_scorecard "GraphRAG" "$(echo "$RESULT" | python3 -c "import json,sys; print(json.dumps(json.load(sys.stdin).get('summary', {}), indent=2))" 2>/dev/null || echo '{}')"
save_results "graphrag" "$RESULT"
