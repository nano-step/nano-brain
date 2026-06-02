# Gemini Review Triage — PR #314

## Cycle 1 — HIGH finding accepted

### Finding 1 HIGH — N+1 SELECT in propagate path
**Verdict: ACCEPTED**

When documents_title_propagate runs, the BEFORE-UPDATE row trigger chunks_search_vector_update fires for every chunk and runs a SELECT title FROM documents WHERE id = NEW.document_id. For a doc with N chunks, the title is looked up N+1 times (1 by propagate + N by row triggers) and the search_vector is computed twice (once by propagate, once by trigger).

**Fix:** Add pg_trigger_depth() > 1 check at the top of chunks_search_vector_update. When called as a cascade from documents_title_propagate (depth=2), skip recompute and accept NEW.search_vector as-is. Direct inserts/updates (depth=1) still recompute with the JOIN.

This is the canonical PG idiom for cooperative trigger chains.

## Cycle 1 verification

- go build PASS, go vet clean
- Migration SQL syntactically valid (CREATE OR REPLACE FUNCTION semantics ensure replays are safe on fresh DBs)
- Live TC-#305 still PASS (title-only search returns correct doc, score 1.000)

Finding real, accepted, ~10 LOC SQL added. No production code changes.
