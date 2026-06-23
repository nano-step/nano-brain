## 1. Research & Setup

- [ ] 1.1 Research each comparison tool (Mem0, Cognee, GraphRAG, LlamaIndex, Zep) — API, setup, Docker
- [ ] 1.2 Create `benchmarks/comparison/setup.sh` — Docker Compose for all comparison tools
- [ ] 1.3 Create `benchmarks/comparison/teardown.sh` — clean up
- [ ] 1.4 Create `benchmarks/comparison/helpers.sh` — shared functions for running queries across tools
- [ ] 1.5 Create `benchmarks/comparison/queries.json` — 20 standardized test queries

## 2. Tool Integration Scripts

- [ ] 2.1 Create `benchmarks/comparison/bench_nanobrain.sh` — run nano-brain baseline
- [ ] 2.2 Create `benchmarks/comparison/bench_mem0.sh` — run Mem0 queries
- [ ] 2.3 Create `benchmarks/comparison/bench_cognee.sh` — run Cognee queries
- [ ] 2.4 Create `benchmarks/comparison/bench_graphrag.sh` — run GraphRAG queries
- [ ] 2.5 Create `benchmarks/comparison/bench_llamaindex.sh` — run LlamaIndex queries
- [ ] 2.6 Create `benchmarks/comparison/bench_zep.sh` — run Zep queries

## 3. Search Quality Benchmark

- [ ] 3.1 Create `benchmarks/comparison/bench_search_quality.sh` — P@5, MRR, recall@20
- [ ] 3.2 Run against nano-brain workspace (our own codebase)
- [ ] 3.3 Run against express-app workspace (large mixed-language codebase)
- [ ] 3.4 Run against rails-app workspace (Rails project)
- [ ] 3.5 Generate `results/search_quality.json`

## 4. Performance Benchmark

- [ ] 4.1 Create `benchmarks/comparison/bench_latency.sh` — p50/p95/p99 query latency
- [ ] 4.2 Create `benchmarks/comparison/bench_throughput.sh` — concurrent query handling
- [ ] 4.3 Create `benchmarks/comparison/bench_resources.sh` — memory, CPU, disk usage
- [ ] 4.4 Generate `results/performance.json`

## 5. Code Intelligence Benchmark (nano-brain only)

- [ ] 5.1 Create `benchmarks/comparison/bench_code_intel.sh` — symbol graph, call chains, impact analysis
- [ ] 5.2 Manually annotate 10 ground-truth functions with expected call chains
- [ ] 5.3 Measure precision/recall against ground truth
- [ ] 5.4 Generate `results/code_intelligence.json`

## 6. Setup Complexity Benchmark

- [ ] 6.1 Create `benchmarks/comparison/bench_setup.sh` — measure zero-to-first-query time
- [ ] 6.2 Time Docker Compose startup for each tool
- [ ] 6.3 Time workspace indexing for each tool
- [ ] 6.4 Generate `results/setup_complexity.json`

## 7. Report

- [ ] 7.1 Create `benchmarks/comparison/REPORT.md` — full comparison report
- [ ] 7.2 Include competitive positioning matrix
- [ ] 7.3 Include feature gap analysis
- [ ] 7.4 Include recommendations for nano-brain roadmap
- [ ] 7.5 Create summary table in PR description

## 8. Validation

- [ ] 8.1 Run all benchmarks against real workspaces
- [ ] 8.2 Verify results are reproducible
- [ ] 8.3 Run `go build ./...` to ensure no build breakage
- [ ] 8.4 Create PR with benchmark results
