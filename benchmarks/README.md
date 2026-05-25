# nano-brain Benchmark Suite

Measures search quality and latency of nano-brain's three search modes across a deterministic synthetic corpus.

## Quick Start

```bash
# Run benchmark (requires Ollama running for vector/hybrid metrics)
nano-brain bench run

# Run at larger scale
nano-brain bench run --scale 500

# Compare against a saved baseline
nano-brain bench compare results/2026-05-06T05-07-24-924Z.json results/baseline-2026.8.2.json
```

## What Gets Measured

### Quality Metrics

Three modes are evaluated against a ground-truth dataset:

| Mode | How it works |
|------|-------------|
| **FTS** | BM25 full-text search — fast, exact keyword matching |
| **Vector** | Cosine similarity on nomic-embed-text embeddings — finds semantic matches |
| **Hybrid** | FTS + Vector fused with RRF reranking — best of both |

Each mode reports three numbers:

#### P@5 — Precision at 5
> Of the top 5 results returned, what fraction are actually relevant?

- `1.000` = all 5 results are correct
- `0.800` = 4 out of 5 are correct, 1 is noise

**Why it matters:** An AI coding assistant fed 5 results with P@5=0.5 wastes ~half its context window on irrelevant files — leading to worse answers or hallucinations.

#### R@10 — Recall at 10
> Of all relevant documents for a query, what fraction appear in the top 10?

- `1.000` = found everything relevant within top 10
- `0.900` = missed 10% of relevant docs

**Why it matters:** If you ask "where do we handle auth token expiry?" and R@10=0.7, nano-brain misses 30% of relevant files. The AI may give you an incomplete picture.

#### MRR — Mean Reciprocal Rank
> On average, at what position does the *first* correct result appear? MRR=1.0 means always rank #1.

- `1.000` = correct answer is always the first result
- `0.500` = correct answer is on average at position 2

**Why it matters for AI agents:** Most agents read the top result first and summarize. MRR=1.0 means the right file/note is always immediately available — no scanning required.

### Latency Metrics

Measured at p50 (median) and p95 (worst-case 95th percentile):

| Metric | What it measures |
|--------|-----------------|
| **Insert p50/p95** | Time to store one document + generate its embedding |
| **Query p50/p95** | FTS search time |
| **Vector p50/p95** | Time to embed query + cosine search |
| **Hybrid p50/p95** | Full hybrid pipeline including reranking |

## Current Results (v2026.8.2, scale-100)

Measured on Apple Silicon (macOS), Ollama local with `nomic-embed-text`:

```
QUALITY
-------
Mode      P@5     R@10    MRR
FTS       0.975   0.985   1.000
Vector    0.875   0.925   1.000
Hybrid    0.835   0.970   1.000

LATENCY
-------
Insert   p50=32ms   p95=59ms   (includes Ollama embedding)
Query    p50=1ms    p95=3ms    (pure SQLite FTS)
Vector   p50=29ms   p95=50ms   (embed query + cosine search)
Hybrid   p50=34ms   p95=69ms   (FTS + vector + rerank)
```

### Reading the numbers

- **FTS MRR=1.000**: For keyword queries, the right result is always #1. At 1ms p50, this is the fastest path.
- **Vector MRR=1.000**: Semantic queries ("how do we verify a user is logged in?") also reliably surface the right result first — even when exact keywords don't match.
- **Hybrid R@10=0.970**: Combining both modes catches 97% of all relevant docs in top 10 — better recall than either mode alone.
- **Vector overhead**: ~28ms over FTS (29ms vs 1ms) — the cost of Ollama embedding the query. For interactive use, imperceptible. For batch processing, relevant.

## When Each Mode Wins

### FTS is best when:
- Query uses exact terms that appear in the documents
- Speed is critical (sub-millisecond)
- Ollama is unavailable

```bash
nano-brain search "JWT token expiry"
# → finds files literally containing "JWT", "token", "expiry"
```

### Vector is best when:
- Query uses different vocabulary than the stored content
- Conceptual/semantic matching matters

```bash
nano-brain vsearch "how do we verify a user is logged in"
# → finds files about "session validation", "auth middleware", "token check"
#   even if those exact words aren't in the query
```

### Hybrid is best when:
- You want maximum recall (don't want to miss anything relevant)
- Running an AI agent that needs comprehensive context
- Default for `nano-brain query`

```bash
nano-brain query "auth token handling"
# → FTS + Vector + RRF reranking → best combined results
```

## Regression Detection

The benchmark suite is designed to catch quality regressions before they ship.

### Save a baseline after a known-good state:

```bash
nano-brain bench run
# → saves result to benchmarks/results/<timestamp>.json
```

Then copy it as the baseline:
```bash
cp benchmarks/results/<timestamp>.json benchmarks/results/baseline-<version>.json
```

### Compare before merging a PR:

```bash
nano-brain bench run
nano-brain bench compare results/<new>.json results/baseline-<version>.json
```

The compare output flags regressions:
- MRR drop > 0.05 → **REGRESSION**
- P@5 drop > 0.10 → **REGRESSION**
- Latency increase > 2x → **WARNING**

### What triggers a regression:

| Change | Risk |
|--------|------|
| Modifying `searchFTS()` or `searchVec()` | High |
| Changing hybrid fusion weights | High |
| Updating schema / FTS5 tokenizer config | High |
| Adding a new embedding model | Medium |
| Refactoring store internals | Low–Medium |

## Fixture Structure

```
benchmarks/
  fixtures/
    scale-100/          # 100 synthetic documents
      docs/             # Markdown files (deterministic, seeded RNG)
      corpus.json       # Corpus metadata + hash
      ground-truth.json # Per-query relevant doc IDs
  results/
    baseline-2026.8.2.json   # Current baseline
    *.json                    # Historical runs
  README.md             # This file
```

Fixtures are **deterministic** — same seed always generates the same corpus. This ensures benchmark results are comparable across machines and time.

## Scales

| Scale | Docs | Typical runtime |
|-------|------|-----------------|
| 10 | 10 | ~10s |
| 100 | 100 | ~2min (default) |
| 500 | 500 | ~10min |
| 1000 | 1000 | ~20min |

Run larger scales when testing search quality at realistic corpus sizes.

## Command Tests

Beyond quality metrics, the suite also smoke-tests all CLI commands end-to-end:

```
COMMANDS  (7/7 pass)
  PASS  search
  PASS  query
  PASS  vsearch
  PASS  write
  PASS  reindex
  PASS  status
  PASS  tags
```

And multi-step combination workflows:

```
COMBINATION TESTS  (3/3 pass)
  PASS  write → reindex → query      (write a note, index it, find it)
  PASS  supersede → query            (supersede a note, old one disappears)
  PASS  harvest → reindex → search   (harvest sessions, index, search them)
```

These catch integration bugs that unit tests miss — e.g. a write that succeeds but doesn't appear in search results.
