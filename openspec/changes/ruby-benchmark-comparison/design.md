## Context

nano-brain is a persistent memory + code intelligence daemon for AI coding agents. Key differentiators vs generic memory solutions:
- **Hybrid search**: BM25 + vector + RRF fusion (not just vector)
- **Code intelligence**: Symbol graph, call chains, impact analysis, CFGs (not just documents)
- **Multi-workspace**: Index multiple repos, cross-workspace stitching
- **Flow diagrams**: HTTP entry → full call chain visualization
- **Go binary**: Single static binary, no runtime deps

The agent memory space in 2026:

| Solution | Approach | Language | Key Feature |
|----------|----------|----------|-------------|
| **nano-brain** | Hybrid search + code graph | Go | Code intelligence, flow diagrams |
| **Mem0** | LLM memory layer | Python | Auto-extraction from conversations |
| **Zep** | Temporal memory graph | Python/Node | Time-aware recall, knowledge graph |
| **Cognee** | Knowledge graph + embeddings | Python | Graph-based RAG, policy engine |
| **GraphRAG** (Microsoft) | Graph-augmented RAG | Python | Community detection, global queries |
| **LlamaIndex** | Data framework | Python | Memory modules, RAG pipelines |

## Comparison Tools

| Tool | What We Compare | Why |
|------|----------------|-----|
| **Mem0** | Search quality + setup complexity | Most popular open-source agent memory |
| **Cognee** | Knowledge graph + hybrid search | Closest architecture to nano-brain (graph + embeddings) |
| **GraphRAG** | Graph-augmented search quality | Microsoft's graph approach, strong community |
| **LlamaIndex** | Memory module capabilities | Most widely used RAG framework |
| **Zep** | Temporal recall + knowledge graph | Unique time-aware approach |

## Benchmark Dimensions

### 1. Search Quality
- **Query set**: 20 queries (feature understanding, debugging, architecture, cross-session)
- **Metrics**: P@5, P@10, MRR, recall@20
- **Workspaces**: nano-brain, express-app, rails-app
- **Comparison**: nano-brain hybrid vs each tool's search

### 2. Code Intelligence (nano-brain unique)
- **Metric**: Symbol graph accuracy, call chain completeness, impact analysis precision
- **Note**: No competitor has equivalent code intelligence — this is our differentiator
- **Baseline**: Measure nano-brain against ground truth (manual annotation of 10 functions)

### 3. Latency
- **Metrics**: p50, p95, p99 query latency
- **Workload**: 100 sequential queries, 10 concurrent queries
- **Comparison**: nano-brain Go binary vs Python-based competitors

### 4. Setup Complexity
- **Metric**: Time from zero to first successful query
- **Target**: < 5 minutes for all tools
- **Measurement**: Docker Compose setup time, initialization time

### 5. Resource Usage
- **Metrics**: Memory (RSS), CPU (steady state), disk (index size)
- **Workload**: After indexing nano-brain + express-app workspaces (~50K docs)
- **Comparison**: Go binary vs Python + database + embedding service

## Decisions

### Decision 1: Standardized Test Queries
Use the same query set across all tools — 20 queries covering feature understanding, debugging, architecture, and cross-session recall. We already have query sets from our LLM benchmarks.

### Decision 2: Real Workspaces
Run benchmarks against real workspaces (nano-brain, express-app, rails-app) — not synthetic data. Real codebases test edge cases like incomplete code, mixed languages, and stale docs.

### Decision 3: Docker Isolation
Run comparison tools in Docker containers. Some tools (Mem0, Cognee) require Python + database + embedding services. Docker isolates these from our Go environment.

### Decision 4: Open-Source Only
Only benchmark open-source tools. Commercial solutions (Pinecone, Weaviate Cloud) are excluded for cost and reproducibility reasons.

## Risks / Trade-offs

### Risk 1: Tool Maturity
Some tools are newer and may have rough edges. **Mitigation**: Document versions, note maturity in report.

### Risk 2: Apples vs Oranges
Different tools solve different problems. nano-brain is code-focused; Mem0 is conversation-focused. **Mitigation**: Clearly scope each comparison. Compare dimensions where they overlap.

### Risk 3: Setup Overhead
Installing 5+ tools takes time. **Mitigation**: Docker Compose for one-click setup. Automate in `setup.sh`.

### Trade-off: Completeness vs Speed
5 tools × 5 dimensions = 25 benchmarks. Covers the most important comparisons without taking weeks.
