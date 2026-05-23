# Story 3.6 Self-Review Evidence

## Oracle Review
- **1 Critical**: VectorSearch Score type int32→float64 (fixed: CAST AS double precision)
- **3 Major**: Workspace isolation on mark queries (fixed), remove embedding from SELECT (fixed), pgvector-go type override (fixed: v0.2.2)
- **3 Minor**: Migration dedup safety (documented), param naming (fixed via sqlc.arg), ResetEmbedStatus semantics (intentional)

## Gemini Review (PR #59)
- No comments at time of merge

## Verification
- `go build ./...` ✅
- `go test -race -short ./...` ✅
