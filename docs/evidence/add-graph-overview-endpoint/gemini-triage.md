# Gemini Review Triage — PR #288

## Cycle 1 (push 1c1aac7)

### Finding 1 — HIGH — `graph_overview.go:126` (knowledge mode doc enrichment)
**Verdict: ACCEPTED**

Gemini correctly identified that `OverviewQuerier` declares `ListDocumentsByIDs` but the handler never calls it. In knowledge mode, returned nodes therefore lack `title`, `collection`, `updated_at`, and `tags`, causing the frontend to render raw doc UUIDs.

Gemini's suggested code referenced non-existent `Kind` and `Label` fields on `neighborhoodNode`. Those fields do NOT exist in the actual struct (only `ID`, `Title`, `Collection`, `UpdatedAt`, `Tags`). Adopted Gemini's diagnosis but mirrored the exact enrichment pattern from `graph_neighborhood.go:186-216` instead of the suggested code.

**Fix:** Added knowledge-mode enrichment block that parses each node ID as `uuid.UUID`, calls `ListDocumentsByIDs`, and populates Title/Collection/UpdatedAt/Tags. Errors degrade to WARN-log + unenriched nodes (matches neighborhood handler behavior).

**Test added:** `TestGraphOverview_KnowledgeModeEnrichesDocs` — verifies `ListDocumentsByIDs` is called with the doc UUIDs and that the response nodes contain enriched fields.

### Finding 2 — MEDIUM — `graph_overview.go:46` (workspace fallback)
**Verdict: ACCEPTED**

Gemini correctly noted that the handler only reads `workspace` from echo context. The frontend `useGraphOverview` hook DOES send `workspace` in the body. If middleware ever fails to inject context (e.g. direct curl without `X-Workspace-Hash` header in some routing path), the request fails 400 even though the body has the workspace.

**Fix:** Added `if workspace == "" { workspace = req.Workspace }` fallback before the empty-check, exactly as Gemini suggested.

**Test added:** `TestGraphOverview_WorkspaceFallbackFromBody` — invokes handler WITHOUT setting `c.Set("workspace", ...)`, sends workspace in body, expects 200.

## Cycle 1 verification
- `go build ./...` exit 0
- `go test -race -short ./internal/server/handlers/ -run TestGraphOverview` → 9/9 PASS
- `bash scripts/smoke-ui.sh` → PASS

## Notes
Both findings were genuinely useful — Gemini caught real gaps in knowledge-mode UX and request-body robustness that the existing 7 tests had not covered. No Gemini findings rejected this cycle.
