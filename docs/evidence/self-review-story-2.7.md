# Story 2.7 Self-Review Evidence

## Oracle Review
- **2 Critical**: CLI panic on trailing flag (fixed bounds check), Rename not updating watcher (fixed: pass watcher + call Watch)
- **4 Major**: CLI URL-encoding (fixed: url.PathEscape/QueryEscape), Rename 500→404 (fixed: sql.ErrNoRows check), Name validation (fixed: regex), N+1 query (fixed: ListCollectionsWithDocCount JOIN)
- **3 Minor**: CLI status code check, watcher test coverage, glob validation — deferred (low risk)

## Gemini Review (PR #55)
- **1 Critical**: Rename doesn't update documents table (fixed: UpdateDocumentsCollection query)
- **3 High**: Watcher rename, N+1, CLI panic — already fixed by Oracle findings
- **1 Medium**: json.Marshal error ignored (fixed: error handling added)

## Verification
- `go build ./...` ✅
- `go test -race -short ./...` ✅ (7 packages pass)
