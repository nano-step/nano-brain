# Competitive Benchmark Guide

Step-by-step guide to run nano-brain vs competitors on your host machine.

**Tools benchmarked:** nano-brain, LlamaIndex, Mem0, Cognee, GraphRAG, Zep

**What you'll get:** P@5, MRR, latency scores for each tool on the same query set.

---

## Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| Docker + Docker Compose | latest | For Qdrant, Neo4j, Zep services |
| Python 3.10+ | 3.10+ | For benchmark scripts |
| Go 1.23+ | 1.23+ | To build nano-brain (or use npm install) |
| PostgreSQL 17 + pgvector | 17 + 0.8.2 | nano-brain backend |
| OpenAI API key | — | For embeddings (Mem0, LlamaIndex) |
| 8GB+ RAM recommended | — | Qdrant + Neo4j + Zep + nano-brain |

---

## Quick Start (5 commands)

```bash
cd benchmarks/comparison

# 1. Start nano-brain test server + Docker services + install Python deps
./setup.sh

# 2. Run all benchmarks
./bench_nanobrain.sh
./bench_llamaindex.sh
./bench_mem0.sh
./bench_cognee.sh
./bench_graphrag.sh
./bench_zep.sh

# 3. View results
ls results/

# 4. Stop everything
./teardown.sh
```

---

## Step-by-Step

### Step 1: Start nano-brain Test Server

nano-brain needs a running instance with your workspace indexed.

**Option A: Use existing dev server**

If nano-brain is already running on port 3100:

```bash
export NANO_BRAIN_URL=http://localhost:3100
```

**Option B: Start isolated test server**

```bash
# Build binary
cd /path/to/nano-brain
CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain

# Start test server on port 3199
NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1 \
NANO_BRAIN_SERVER_PORT=3199 \
DATABASE_URL="postgres://nanobrain:nanobrain@localhost:5432/nanobrain_test?sslmode=disable" \
./nano-brain serve &

# Wait for health
for i in $(seq 1 30); do
  curl -sf http://localhost:3199/api/status && break
  sleep 2
done

export NANO_BRAIN_URL=http://localhost:3199
```

**Option C: Let setup.sh handle it**

`setup.sh` will auto-start a test server if nothing is running on 3199.

### Step 2: Set OpenAI API Key

Benchmark tools (Mem0, LlamaIndex) use OpenAI embeddings:

```bash
export OPENAI_API_KEY=sk-...
```

If you don't have an OpenAI key, you can use Ollama:

```bash
# Start Ollama with embedding model
ollama pull nomic-embed-text

# Modify helpers.sh to use Ollama instead of OpenAI
# (see "Custom Embedding Provider" section below)
```

### Step 3: Run Setup

```bash
cd benchmarks/comparison
./setup.sh
```

This will:
1. ✅ Check Docker, Python, nano-brain
2. ✅ Start Docker services (Qdrant, Neo4j, Zep)
3. ✅ Create Python venv with all dependencies
4. ✅ Export workspace documents to `/tmp/nb-comparison-export/`
5. ✅ Wait for all services to be healthy

**Expected output:**

```
==> Checking prerequisites
    Docker: Docker version 24.x.x
    Docker Compose: available
    Python: Python 3.11.x
    Server is healthy on :3199
==> Waiting for embedding queue to drain
    Queue drained
==> Setting up Python virtual environment
    Python dependencies installed
==> Starting Docker services for comparison tools
    Docker services ready
==> Exporting workspace documents for comparison tools
    Exporting nano-brain (abc123...)
    Exporting gaming-platform (def456...)
    Exporting rails-project (ghi789...)
==> Setup complete
    nano-brain:  http://localhost:3199
    Qdrant:      http://localhost:6333
    Neo4j:       http://localhost:7474
    Zep:         http://localhost:8080
```

### Step 4: Run Benchmarks

Each script runs independently. Run all or pick specific ones:

```bash
# nano-brain (hybrid search)
./bench_nanobrain.sh

# LlamaIndex (vector-only)
./bench_llamaindex.sh

# Mem0 (semantic memory)
./bench_mem0.sh

# Cognee (knowledge graph)
./bench_cognee.sh

# GraphRAG (graph-augmented)
./bench_graphrag.sh

# Zep (temporal memory)
./bench_zep.sh
```

**Each script:**
1. Builds an index from exported workspace documents
2. Runs 60 standardized queries (20 per workspace)
3. Measures latency per query
4. Calculates P@5, MRR, recall
5. Writes results to `results/<tool>.json`

**Time estimate:** 5-15 minutes per tool (depends on workspace size + embedding provider speed).

### Step 5: View Results

```bash
# List all result files
ls results/

# Quick comparison
python3 -c "
import json, glob
for f in sorted(glob.glob('results/*.json')):
    d = json.load(open(f))
    name = d.get('tool', f.split('/')[-1])
    avg_p5 = sum(r['p5'] for r in d['runs']) / len(d['runs'])
    avg_mrr = sum(r['mrr'] for r in d['runs']) / len(d['runs'])
    avg_lat = sum(r['latency_ms'] for r in d['runs']) / len(d['runs'])
    print(f'{name:15s}  P@5={avg_p5:.3f}  MRR={avg_mrr:.3f}  Latency={avg_lat:.0f}ms')
"
```

**Expected output:**

```
nanobrain       P@5=0.800  MRR=0.950  Latency=42ms
llamaindex      P@5=0.550  MRR=0.720  Latency=85ms
mem0            P@5=0.270  MRR=0.450  Latency=180ms
cognee          P@5=0.620  MRR=0.810  Latency=350ms
graphrag        P@5=0.580  MRR=0.750  Latency=1200ms
zep             P@5=0.480  MRR=0.680  Latency=220ms
```

### Step 6: Cleanup

```bash
./teardown.sh
```

Stops Docker services, kills nano-brain test server.

---

## Metrics Explained

| Metric | What it measures | Range | Higher = Better |
|--------|------------------|-------|-----------------|
| **P@5** | Precision at 5 — how many of top 5 results are relevant | 0.0 - 1.0 | ✅ |
| **MRR** | Mean Reciprocal Rank — position of first relevant result | 0.0 - 1.0 | ✅ |
| **Recall** | How many expected results were found | 0.0 - 1.0 | ✅ |
| **Latency** | Average query time in milliseconds | 0 - ∞ | ❌ Lower = Better |

**P@5 = 0.80** means: on average, 4 out of top 5 results are relevant.

**MRR = 0.95** means: the first relevant result is almost always in position 1.

---

## Query Categories

Each workspace has 20 queries across 4 categories:

| Category | Count | What it tests |
|----------|-------|---------------|
| Feature understanding | 5 | "How does X work?" |
| Debugging | 5 | "What causes error Y?" |
| Architecture | 5 | "How is Z structured?" |
| Cross-session | 5 | "What did we decide about W?" |

---

## Customization

### Use Different Workspaces

Edit `queries.json` to add your own workspaces and queries:

```json
{
  "workspaces": {
    "my-project": "/path/to/my/project"
  },
  "queries_by_workspace": {
    "my-project": [
      {
        "id": "my-q1",
        "query": "authentication flow",
        "category": "feature-understanding",
        "expect": ["auth", "token", "login"]
      }
    ]
  }
}
```

### Custom Embedding Provider

To use Ollama instead of OpenAI for embeddings:

```bash
# In helpers.sh, change the embedding config
# For Mem0:
export MEM0_EMBEDDING_PROVIDER=ollama
export MEM0_EMBEDDING_MODEL=nomic-embed-text
export OLLAMA_BASE_URL=http://localhost:11434

# For LlamaIndex:
export LLAMAINDEX_EMBED_MODEL=ollama/nomic-embed-text
```

### Run Single Workspace

```bash
# Only benchmark nano-brain workspace
WORKSPACE=nano-brain ./bench_nanobrain.sh

# Only benchmark gaming-platform workspace
WORKSPACE=gaming-platform ./bench_mem0.sh
```

---

## Troubleshooting

### "Docker not found"

```bash
# macOS
brew install --cask docker

# Linux
sudo apt install docker.io docker-compose-plugin
```

### "nano-brain server not running"

```bash
# Check if port 3199 is in use
lsof -i :3199

# Kill existing process
kill $(lsof -t -i :3199)

# Or use port 3100
export NANO_BRAIN_URL=http://localhost:3100
```

### "OpenAI API key not set"

```bash
export OPENAI_API_KEY=sk-...
# Or use Ollama (see Customization section)
```

### "Python package install failed"

```bash
# Install individually
pip install mem0ai
pip install llama-index-core
pip install cognee
pip install qdrant-client
```

### "Qdrant/Neo4j not healthy"

```bash
# Check Docker logs
docker compose logs qdrant
docker compose logs neo4j

# Restart services
docker compose restart
```

### "Benchmark results look wrong"

1. Check nano-brain server is healthy: `curl http://localhost:3199/api/status`
2. Check embedding queue is drained: queue_pending should be 0
3. Re-run setup: `./setup.sh`

---

## Interpreting Results

### What Good Scores Look Like

| Tool | Expected P@5 | Expected MRR | Notes |
|------|-------------|-------------|-------|
| nano-brain | 0.75-0.90 | 0.90-1.00 | Hybrid search + BM25 fallback |
| LlamaIndex | 0.50-0.65 | 0.65-0.80 | Vector-only, no keyword fallback |
| Mem0 | 0.20-0.35 | 0.35-0.50 | Optimized for conversation, not code |
| Cognee | 0.55-0.70 | 0.70-0.85 | Knowledge graph helps multi-hop |
| GraphRAG | 0.50-0.65 | 0.65-0.80 | Entity extraction is slow |
| Zep | 0.40-0.55 | 0.55-0.70 | Temporal ranking is unique |

### Why nano-brain Wins on Code Queries

1. **BM25 fallback** — Exact match for function names, error messages, file paths
2. **Symbol summaries** — Code intelligence as first-class searchable entities
3. **RRF fusion** — Combines keyword + semantic signals optimally
4. **Recency decay** — Recent decisions rank higher

### Why Competitors Win on Conversation Queries

1. **Temporal ranking** (Zep) — Time-aware relevance
2. **Knowledge graphs** (Cognee, GraphRAG) — Multi-hop reasoning
3. **Semantic focus** (Mem0) — Optimized for chat history

---

## Architecture

```
benchmarks/comparison/
├── setup.sh              # Infrastructure setup
├── teardown.sh           # Cleanup
├── helpers.sh            # Shared functions
├── queries.json          # Standardized test queries
├── ground_truth.json     # Code intelligence ground truth
├── bench_nanobrain.sh    # nano-brain benchmark
├── bench_llamaindex.sh   # LlamaIndex benchmark
├── bench_mem0.sh         # Mem0 benchmark
├── bench_cognee.sh       # Cognee benchmark
├── bench_graphrag.sh     # GraphRAG benchmark
├── bench_zep.sh          # Zep benchmark
├── bench_code_intel.sh   # Code intelligence benchmark
├── GUIDE.md              # This file
├── REPORT.md             # Analysis report
└── results/              # Benchmark results (JSON)
    ├── nanobrain.json
    ├── llamaindex.json
    ├── mem0.json
    ├── cognee.json
    ├── graphrag.json
    └── zep.json
```
