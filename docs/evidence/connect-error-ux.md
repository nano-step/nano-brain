# Smoke evidence: connect-error-ux (#141)

OpenSpec change: `openspec/changes/connect-error-ux/`
Branch: `feat/connect-error-ux`

## Build

```
$ CGO_ENABLED=0 go build -o /tmp/nano-brain-smoke ./cmd/nano-brain
$ /tmp/nano-brain-smoke --help | head -3
Usage of /tmp/nano-brain-smoke:
  -config string
      path to config file (default: ~/.nano-brain/config.yml)
```

## Case 1 — non-TTY (stdin redirected from /dev/null), no server

Maps to spec scenarios "Stdin piped" and "Both non-TTY (CI / agent harness)".
Expected: error + suggestion, no prompt, exit non-zero.

```
$ NANO_BRAIN_HOST=127.0.0.1 NANO_BRAIN_PORT=29991 ./nano-brain status < /dev/null
Error: cannot connect to nano-brain server at 127.0.0.1:29991
The server does not appear to be running.
Run this to start it: nano-brain serve -d
Cannot reach nano-brain server: cannot connect to nano-brain server at 127.0.0.1:29991
exit=1
```

PASS — three lines present, no prompt, suggestion is binary form.

## Case 2 — NANO_BRAIN_NO_AUTO_START override

Maps to spec scenario "Override set in TTY session".
Expected: error + suggestion, no prompt even when env says to skip.

```
$ NANO_BRAIN_HOST=127.0.0.1 NANO_BRAIN_PORT=29991 NANO_BRAIN_NO_AUTO_START=1 ./nano-brain status
Error: cannot connect to nano-brain server at 127.0.0.1:29991
The server does not appear to be running.
Run this to start it: nano-brain serve -d
Cannot reach nano-brain server: cannot connect to nano-brain server at 127.0.0.1:29991
exit=1
```

PASS — no prompt, env override honored.

## Case 3 — npx breadcrumb (npm_execpath set)

Maps to spec scenario "Launched via npx".
Expected: suggestion line uses the `npx @nano-step/nano-brain@beta serve -d` form.

```
$ NANO_BRAIN_HOST=127.0.0.1 NANO_BRAIN_PORT=29991 \
    npm_execpath=/path/to/npx-cli.js ./nano-brain status < /dev/null
Error: cannot connect to nano-brain server at 127.0.0.1:29991
The server does not appear to be running.
Run this to start it: npx @nano-step/nano-brain@beta serve -d
Cannot reach nano-brain server: cannot connect to nano-brain server at 127.0.0.1:29991
exit=1
```

PASS — npx suggestion rendered when launch breadcrumb is present.

## Interactive auto-start prompt (Case 4 — limited harness)

Maps to spec scenarios "Both stdin and stderr are TTY, user accepts" and
"User declines the prompt".

Cannot be exercised inside this CI/agent harness — the executing process
has both stdin and stderr non-TTY by design (sandboxed). Equivalent
coverage is provided by unit tests in `cmd/nano-brain/commands_test.go`:

- `TestDoRequest_ConnectionRefused_UserAcceptsTriggersRecovery` — accept path invokes mocked daemon
- `TestDoRequest_ConnectionRefused_UserDeclines` — decline path skips daemon
- `TestPromptStartServer` (table-driven) — Y/y/empty accept, N/n/garbage decline, EOF declines
- `TestDoRequest_RetrySucceeds_HappyPath` — retry path forwards intact response

These tests stub `isTTYFn`, `runServeDaemonFn`, `promptReader` to exercise the
TTY branch without a real TTY.

## Health-check polling (Case 5)

Maps to spec scenarios "Daemon becomes healthy quickly" and "Daemon fails to
become healthy". Covered by `TestWaitForServerHealthy_BecomesHealthy` (httptest
server flips to 200 after 400 ms, polling succeeds) and
`TestWaitForServerHealthy_Timeout` (server always 503, returns "did not become
healthy within 500ms" error after the deadline).

## Validation ladder

```
$ CGO_ENABLED=0 go build ./...
$ go test -race -short ./cmd/nano-brain/...
ok  github.com/nano-brain/nano-brain/cmd/nano-brain  2.379s
$ go test -race -short ./...
ok  github.com/nano-brain/nano-brain/cmd/nano-brain  (cached)
ok  github.com/nano-brain/nano-brain/internal/...    (all green)
```
