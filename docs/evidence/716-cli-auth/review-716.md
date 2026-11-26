# Independent review: issue 716

Review Verdict: PASS
Reviewer: independent reviewer

The initial review identified two blocking credential-disclosure risks:
remote plaintext HTTP and forwarding the server token to a different MCP
origin. Both were fixed with explicit insecure-HTTP opt-in and same-origin
checks.

The implementation was then refactored so shared CLI authentication lives in
`cmd/nano-brain/client_auth.go` and `internal/clientauth` instead of
`cmd/nano-brain/client.go`. The reviewer re-reviewed the final file set and
approved it: `sendRequest`/`doRequest` remain authenticated through the shared
HTTP transport, collection/doctor/health paths keep explicit protection,
authenticated redirects are confined to the original origin, remote HTTP is
refused unless explicitly enabled, and doctor does not forward the server token
to a different MCP origin.

The reviewer did not execute tests. Remaining compatibility limitations:
remote authenticated HTTP is rejected unless explicitly enabled, and the CLI
does not provide a first-class HTTPS server URL.

The final change set has zero file overlap with open PR #618.
