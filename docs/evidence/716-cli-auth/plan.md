# Issue 716: CLI authentication

## Goal

Authenticate CLI and online doctor requests when the server requires a bearer
token, without sending credentials to the embedding provider or exposing them
in command output.

## Plan

1. Resolve `NANO_BRAIN_TOKEN` first, then the first token in the selected
   config; retain unauthenticated behavior when neither is available.
2. Inject the bearer token through the shared CLI HTTP transport so existing
   `sendRequest`/`doRequest` callers are covered without editing `client.go`.
   Authenticate collection commands, startup health polling, and both online
   doctor probes explicitly. Restrict authenticated redirects to the original
   scheme and authority. Reject remote HTTP credentials unless explicitly opted
   in, and do not forward the server token to an MCP override on a different
   origin.
3. Test the actual CLI against an isolated HTTP server with the production auth
   middleware. Exercise token precedence, wrong credentials, command output,
   doctor probes, redirect isolation, and remote-HTTP refusal.
4. Run the project validation and independent-review gates before merging.

## Scope and limitations

The CLI's server URL is HTTP, not HTTPS. A remote deployment needs a secure
local tunnel, unless the operator explicitly opts into insecure remote HTTP.
The change intentionally avoids `cmd/nano-brain/client.go`, `README.md`, and
`docs/SETUP_AGENT.md` so it has zero file overlap with open PR #618.
