# Self-review: issue 716

## Actions Taken

Traced outbound CLI and doctor HTTP requests, moved shared CLI authentication
into a transport/helper layer so `client.go` stays untouched, checked credential
precedence and redirect handling, and ran isolated CLI smoke tests with real
auth middleware. An independent review found remote HTTP and cross-origin MCP
token exposure; both findings were fixed. The implementation was then reshaped
to remove file overlap with open PR #618.

## Files Changed

`cmd/nano-brain/client_auth.go`, `client_helpers.go`, `collection.go`,
`doctor.go`, CLI auth tests, `internal/clientauth`, doctor checks, isolated
integration test, and configuration documentation.

## Findings Summary

Authenticated redirects are restricted to the original origin; remote HTTP is
refused by default; doctor does not forward the server token to a different
MCP origin. The embedding-provider probe does not receive the server token.
The final file set has zero overlap with PR #618.

## Resolution Status

The isolated smoke tests and targeted race tests pass. Full integration and
project gates must pass before merge.
