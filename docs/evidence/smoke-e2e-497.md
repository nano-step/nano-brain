# smoke:e2e — Issue #497 (recursive fsnotify watches)

Status: **N/A — no REST API endpoint added or changed** (R19 applies to endpoint changes).

This bug-fix is internal to `internal/watcher`; it adds no route and changes no
request/response contract. The equivalent end-to-end verification was done at the
watcher layer instead:

- `go test -race -tags=integration ./internal/watcher/` — PASS, including
  `TestSubdirEdit_IndexedWithoutRootActivity`, which boots the real `Run` loop
  with a real `fsnotify.Watcher`, writes a file in a nested subdirectory with no
  root-level activity, and asserts it is indexed (upsert) off its own event.
- Live reproduction against the running server confirmed the pre-fix behavior
  (subdir file not indexed for 18s without root activity) and that a root-level
  event triggers a full walk that catches the subdir change.

No REST endpoints to exercise via curl; FAIL conditions in the smoke:e2e gate
(status codes, response JSON) are not applicable.
