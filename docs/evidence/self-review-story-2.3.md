# Self-Review Evidence: Story 2.3 — Document Write Endpoint

**Date:** 2026-05-23
**Reviewer:** Oracle (via Sisyphus orchestration)
**Branch:** story/2.3-document-write
**Session:** ses_1ab7f4b4effeBJ5bRc5Pn0lIZ1

## Findings

| ID | Severity | Description | Status |
|----|----------|-------------|--------|
| C1 | Critical | Metadata field omitted in UpsertDocumentParams — silent data loss on upsert | **Fixed** |
| M1 | Major | Collection default mismatch: handler "memory" vs schema "default" | **Fixed** (comment added) |
| m1 | Minor | Dead Workspace field in WriteRequest struct | **Fixed** (removed) |
| m2 | Minor | No SHA-256 hash verification test | **Fixed** (test added) |
| m3 | Minor | HTTP 201 for upserts (should be 200 on update) | Accepted — no way to distinguish insert vs update from RETURNING clause |

## Fixes Applied

### C1: Metadata field
- Added `Metadata json.RawMessage` to WriteRequest
- Handler defaults to `pqtype.NullRawMessage{RawMessage: []byte("{}"), Valid: true}`
- User-provided metadata is passed through

### M1: Collection default
- Added explanatory comment in handler
- Schema default "default" is never used because handler always sets explicitly

### m1: Dead field
- Removed `Workspace` from WriteRequest — middleware sets it via context

### m2: Hash test
- Added `TestWriteDocument_HashVerification` verifying SHA-256("hello world")

## Verification
- `go build ./...` ✅
- `go test -race -short -count=1 ./...` ✅ (6 test packages pass)
