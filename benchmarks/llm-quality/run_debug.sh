#!/bin/bash
set -e

SERVER_URL="${NANO_BRAIN_URL:-http://localhost:3199}"
QUERIES_FILE="${1:-benchmarks/llm-quality/queries_debug.json}"
RESULTS_FILE="${2:-benchmarks/llm-quality/results_debug_$(date +%Y%m%d_%H%M%S).json}"
export RESULTS_FILE

WORKSPACE=$(python3 -c "import json; print(json.load(open('$QUERIES_FILE'))['workspace'])")

python3 << PYEOF
import json, subprocess, time, sys

url = "$SERVER_URL"
workspace = "$WORKSPACE"
queries_file = "$QUERIES_FILE"
results_file = "$RESULTS_FILE"

with open(queries_file) as f:
    queries_data = json.load(f)
queries = queries_data["queries"]

def search(ws, query, max_results=5):
    payload = json.dumps({"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_search","arguments":{"workspace":ws,"query":query,"max_results":max_results}}})
    start = time.time()
    r = subprocess.run(["curl","-s","-X","POST",f"{url}/mcp","-H","Content-Type: application/json","-d",payload], capture_output=True, text=True, timeout=10)
    latency_ms = int((time.time() - start) * 1000)
    for line in r.stdout.split("\n"):
        if line.startswith("data: "):
            data = json.loads(line[6:])
            content = data["result"]["content"][0]["text"]
            result = json.loads(content)
            return result.get("results", []), latency_ms
    return [], latency_ms

def hybrid_search(ws, query, max_results=5):
    payload = json.dumps({"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_query","arguments":{"workspace":ws,"query":query,"max_results":max_results}}})
    start = time.time()
    r = subprocess.run(["curl","-s","-X","POST",f"{url}/mcp","-H","Content-Type: application/json","-d",payload], capture_output=True, text=True, timeout=10)
    latency_ms = int((time.time() - start) * 1000)
    for line in r.stdout.split("\n"):
        if line.startswith("data: "):
            data = json.loads(line[6:])
            content = data["result"]["content"][0]["text"]
            result = json.loads(content)
            return result.get("results", []), latency_ms
    return [], latency_ms

def score_terms(results, terms):
    all_text = " ".join([r.get("snippet","") + " " + r.get("title","") for r in results]).lower()
    matches = sum(1 for t in terms if t.lower() in all_text)
    return matches / len(terms) if terms else 0

def count_tags(results, tag):
    return sum(1 for r in results if tag in r.get("tags", []))

all_results = []
for q in queries:
    query = q["query"]
    print(f"  {query[:50]:50s}", end="", flush=True)

    code_results, t1 = search(workspace, query, 5)
    session_results, t2 = hybrid_search(workspace, query + " debug session", 5)
    latency = max(t1, t2)

    code_score = score_terms(code_results, q["expect_code"])
    session_score = score_terms(session_results, q["expect_sessions"])

    all_code = code_results + session_results
    combined_score = score_terms(all_code, q["expect_code"] + q["expect_sessions"])

    summaries_found = count_tags(all_code, "symbol-summary")

    result = {
        "id": q["id"], "query": query, "category": q["category"],
        "code_results": len(code_results), "session_results": len(session_results),
        "code_score": round(code_score, 3), "session_score": round(session_score, 3),
        "combined_score": round(combined_score, 3),
        "summaries_found": summaries_found,
        "latency_ms": latency
    }
    all_results.append(result)
    print(f" | code={code_score:.2f} session={session_score:.2f} combined={combined_score:.2f} summaries={summaries_found} {latency}ms")

avg_code = sum(r["code_score"] for r in all_results) / len(all_results)
avg_session = sum(r["session_score"] for r in all_results) / len(all_results)
avg_combined = sum(r["combined_score"] for r in all_results) / len(all_results)
avg_latency = sum(r["latency_ms"] for r in all_results) / len(all_results)
total_summaries = sum(r["summaries_found"] for r in all_results)

print(f"\nDEBUGGING BENCHMARK: code={avg_code:.3f} session={avg_session:.3f} combined={avg_combined:.3f} latency={avg_latency:.0f}ms summaries={total_summaries}")

output = {
    "run": "debugging", "workspace": workspace,
    "avg_code_score": round(avg_code, 3), "avg_session_score": round(avg_session, 3),
    "avg_combined_score": round(avg_combined, 3), "avg_latency_ms": round(avg_latency, 0),
    "total_summaries_found": total_summaries, "queries": all_results
}

with open(results_file, "w") as f:
    json.dump(output, f, indent=2)
print(f"\nResults: {results_file}")
PYEOF
