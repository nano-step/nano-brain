# Phase 13: Interactive Init Wizard - Pattern Map

**Mapped:** 2026-07-02
**Files analyzed:** 10 (5 new, 3 modified, 2 doc)
**Analogs found:** 8 / 10

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `cmd/nano-brain/init.go` (restructured `runInteractiveInit`) | controller (CLI wizard orchestrator) | request-response (sequential prompts + writes) | `cmd/nano-brain/init.go` itself (current `runInteractiveInit`, 26-253) + `mcp_client_config.go`'s `promptMCPClientConfig` orchestration (244-279) | exact (self-refactor + established orchestration idiom) |
| `cmd/nano-brain/docker_provision.go` (NEW) | service (os/exec wrapper) | event-driven / process-exec | `cmd/nano-brain/daemon.go` (`os.StartProcess`/exit-code handling, 65-141) + `client_helpers.go` (`isTTY`/hook-var pattern) | role-match (only other os/exec-adjacent process-control code in repo) |
| `cmd/nano-brain/init_db.go` (NEW, DB detect/provision/poll step) | service (CRUD-ish: connect-poll + config field write) | request-response + polling | `internal/health/doctor/doctor.go` `CheckPostgreSQL` (61-86) + `client_helpers.go` `waitForServerHealthy` (85-118) | exact (same connect+ping shape, same poll-loop shape) |
| `cmd/nano-brain/init_embedding.go` (NEW, embedding step) | service | request-response | `cmd/nano-brain/init.go` embedding block (66-94) + `detectOllama` (16-24) | exact (this is literally the logic being extracted) |
| `cmd/nano-brain/init_serve.go` / `init_serve_unix.go` / `init_serve_windows.go` (NEW, serve step) | controller | request-response | `cmd/nano-brain/client.go` `maybeAutoStart`-style hook block (18-29, 157-167) + `commands_test.go` `withRecoveryHooks` (500-525) | exact (identical hook-var seam: `isTTYFn`, `promptReader`, `runServeDaemonFn`) |
| `cmd/nano-brain/init_register.go` (NEW, extracted registration helper) | service | request-response (HTTP POST) | `cmd/nano-brain/commands.go` `runInitCmd` `--root` branch (57-131) + `triggerInitBackground` (134-161) | exact (this is the extraction source) |
| `cmd/nano-brain/commands.go` (call extracted helper from `--root` branch) | controller | request-response | itself, `runInitCmd` (14-132) | exact (self-refactor) |
| `internal/health/doctor/doctor.go` (`CheckEmbeddingProvider`/`CheckEmbeddingModel` skip path) | service (pure function health check) | request-response | itself, existing functions (100-176) | exact (in-place edit) |
| `docs/SETUP_AGENT.md`, `README.md` | docs | — | existing doc files | n/a (docs, no code analog needed) |
| `*_test.go` for each new file | test | — | `cmd/nano-brain/commands_test.go` (`withRecoveryHooks`, `TestPromptStartServer`) + `internal/health/doctor/doctor_test.go` (table-driven Check tests) | exact |

## Pattern Assignments

### `cmd/nano-brain/init.go` (restructured orchestrator)

**Analog:** current `runInteractiveInit` (same file, lines 26-253) — becomes the skeleton to decompose; also borrow the **step-orchestration idiom** from `mcp_client_config.go`'s `promptMCPClientConfig`.

**Imports pattern** (current file, lines 1-14):
```go
import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
)
```

**Config-exists gate pattern** (lines 41-49) — reuse verbatim for D-03's keep/overwrite, just change the prompt to "[k]eep/[o]verwrite":
```go
if _, err := os.Stat(configPath); err == nil {
	fmt.Printf("  Config exists at %s\n", configPath)
	answer := promptWithDefault(scanner, "Overwrite?", "Y")
	if answer == "n" || answer == "N" {
		fmt.Println("Aborted.")
		return
	}
	fmt.Println()
}
```

**Prompt helper to reuse unchanged** (lines 255-267):
```go
func promptWithDefault(scanner *bufio.Scanner, prompt, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Printf("  %s: ", prompt)
	}
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return defaultVal
	}
	return input
}
```
For any NEW prompt that gates a file write / process start (advanced-gate Y/N, "start server now", "register directory", DB Docker-provision confirm), use `promptConsequential` instead (see Shared Patterns below) — NOT `promptWithDefault` — because those are consequential actions where EOF must mean "decline", per `mcp_client_config.go`'s CR-01 comment (lines 342-348).

**Step-orchestration idiom to copy** (`mcp_client_config.go:244-279`, `promptMCPClientConfig`): a top-level function that prints a section header, then calls a sequence of step functions each taking the shared `*bufio.Scanner` — this is the shape `runInteractiveInit` should collapse into once split into `stepConfigGate`, `stepDatabase`, `stepEmbedding`, `stepAdvanced`, `stepWriteAndDoctor`, `stepServe`, `stepRegister`, `stepMCP`.

**YAML assembly + write pattern** (lines 214-244) — keep unchanged, just feed it fields resolved by the new steps instead of always-prompted values:
```go
if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
	fmt.Fprintf(os.Stderr, "Failed to create config directory: %v\n", err)
	os.Exit(1)
}
if err := os.WriteFile(configPath, []byte(yaml), 0600); err != nil {
	fmt.Fprintf(os.Stderr, "Failed to write config: %v\n", err)
	os.Exit(1)
}
```

---

### `cmd/nano-brain/init_db.go` (NEW — D-05..D-10)

**Analog:** `internal/health/doctor/doctor.go` `CheckPostgreSQL` (lines 61-86) for the connect+ping shape; `client_helpers.go` `waitForServerHealthy` (lines 85-118) for the poll-loop shape.

**Connect+ping pattern to mirror** (`doctor.go:61-86`):
```go
func CheckPostgreSQL(dbURL string) (Check, *pgx.Conn) {
	if dbURL == "" {
		return Check{Name: "PostgreSQL", Status: "fail", Detail: "no URL configured", Hint: "..."}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return Check{Name: "PostgreSQL", Status: "fail", ...}, nil
	}
	if err := conn.Ping(ctx); err != nil {
		conn.Close(ctx)
		return Check{Name: "PostgreSQL", Status: "fail", ...}, nil
	}
	return Check{Name: "PostgreSQL", Status: "ok", Detail: host}, conn
}
```
Reuse this exact 3s-timeout connect+ping approach for the wizard's initial detection (D-05 step 1) and for D-09's live-validate-before-write. For D-08's post-provision poll, wrap the same connect+ping call in a retry loop shaped exactly like `waitForServerHealthy`:

**Poll-loop pattern to mirror** (`client_helpers.go:85-118`):
```go
func waitForServerHealthy(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err == nil {
			resp, doErr := httpClient.Do(req)
			if doErr == nil {
				status := resp.StatusCode
				_ = resp.Body.Close()
				if status == http.StatusOK {
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("server did not become healthy within %s", timeout)
		}
		remaining := time.Until(deadline)
		sleep := healthPollInterval
		if remaining < sleep {
			sleep = remaining
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}
	}
}
```
Substitute the HTTP GET with `pgx.Connect`+`Ping` per RESEARCH's `waitForPostgresReady` example (already spec'd in RESEARCH.md Pattern 3) — same deadline/remaining/sleep-clamp structure, so tests can use the same "manipulate `time.Now`-independent deadline" style already exercised in `commands_test.go` timeout tests (see `TestDoRequest_ConnectionRefused` region using `time.Since(start)` assertions, lines 460-466).

**Error-detail extraction pattern** (`doctor.go:69-73`) — reuse for printing user-facing host info without leaking full credentialed URL:
```go
parsed, _ := url.Parse(dbURL)
host := "unknown"
if parsed != nil {
	host = parsed.Host
}
```

---

### `cmd/nano-brain/docker_provision.go` (NEW — D-06/D-07)

**No close analog exists in the codebase** (no prior `os/exec` shell-out for a CLI subprocess with structured exit-code classification). Use the **hook-var test-seam idiom** from `client.go`/`commands_test.go` as the closest structural analog, since Docker calls must be equally injectable for tests:

**Hook-var seam pattern to mirror** (`client.go:18-29`):
```go
// runServeDaemonFn is the daemon launcher hook. Tests override it.
var runServeDaemonFn = runServeDaemon
var isTTYFn = isTTY
```
Apply the identical idiom: declare `var runDocker dockerRunner = defaultRunDocker` (per RESEARCH.md's `dockerRunner` type/example, lines 199-225) so `docker_provision_test.go` can substitute canned stdout/stderr/exit codes exactly like `commands_test.go`'s `withRecoveryHooks` substitutes `runServeDaemonFn`.

**Exit-code/PID-file style error handling to mirror** (`daemon.go:65-74`, `os.StartProcess` guard shape):
```go
if pid, err := readPID(); err == nil && isRunning(pid) {
	fmt.Fprintf(os.Stderr, "nano-brain is already running (PID: %d)\n", pid)
	os.Exit(1)
}
exe, err := os.Executable()
if err != nil {
	fmt.Fprintf(os.Stderr, "cannot resolve binary path: %v\n", err)
	os.Exit(1)
}
```
Mirror this "check-state → early-return-with-message" structure for `docker info` (not-installed vs daemon-down vs available) and `docker run` (name-conflict → `docker start`; port-conflict → `docker rm` + retry on 5433) per RESEARCH.md Patterns 1-2 and Pitfall 1.

---

### `cmd/nano-brain/init_embedding.go` (NEW — D-11/D-12)

**Analog:** `cmd/nano-brain/init.go` current embedding block (lines 66-94) — this is a direct extraction, not a new pattern.

**Core pattern to extract almost verbatim**:
```go
if detectOllama(embURL) {
	fmt.Printf("  ✓ Ollama detected at %s\n", embURL)
} else {
	fmt.Println("  Ollama not found at default URL. Make sure Ollama is running.")
	embURL = promptWithDefault(scanner, "Ollama URL", embURL)
}
```
`detectOllama` itself (lines 16-24) stays unchanged:
```go
func detectOllama(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}
```
New behavior needed on top: the D-11 "Enable semantic embeddings? [Y/n]" gate (use `promptWithDefault`, non-consequential — a "no" here doesn't destroy anything) wrapping this block, and on "no" writing `embedding:\n  provider: ""` per the existing YAML-block-string-building convention already used for `embBlock`/`harvesterBlock`/`summaryBlock` (see `init.go:82,93,132,178-182`).

---

### `cmd/nano-brain/init_serve.go` + `init_serve_unix.go` / `init_serve_windows.go` (NEW — D-14)

**Analog:** `cmd/nano-brain/client.go` lines 157-167 (existing auto-start-on-connection-refused logic) + `commands_test.go`'s `withRecoveryHooks` (500-525) and `TestPromptStartServer` (468-498).

**Auto-start decision pattern to mirror** (`client.go:157-167`):
```go
if os.Getenv("NANO_BRAIN_NO_AUTO_START") == "1" || !isTTYFn() {
	return /* skip */
}
if !promptStartServer(promptReader, promptWriter) {
	return /* declined */
}
runServeDaemonFn(config.ResolveConfigPath(""))
```
This is exactly D-14's shape: TTY-gated, prompt-gated, then daemon-launch via the `runServeDaemonFn` hook var — reuse `promptStartServer` (`client_helpers.go:18-33`) directly rather than writing a new Y/n prompt function.

**promptStartServer to reuse verbatim** (`client_helpers.go:18-33`):
```go
func promptStartServer(reader io.Reader, writer io.Writer) bool {
	fmt.Fprint(writer, "Start server now? [Y/n]: ")
	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		return false
	}
	answer := strings.TrimSpace(scanner.Text())
	if answer == "" {
		return true
	}
	switch answer[0] {
	case 'Y', 'y':
		return true
	}
	return false
}
```

**Test hook pattern to mirror** (`commands_test.go:500-525`):
```go
func withRecoveryHooks(t *testing.T, isTTYReturn bool, accept bool, daemon func()) {
	t.Helper()
	origIsTTY := isTTYFn
	origReader := promptReader
	origWriter := promptWriter
	origDaemon := runServeDaemonFn
	isTTYFn = func() bool { return isTTYReturn }
	if accept {
		promptReader = bytes.NewBufferString("Y\n")
	} else {
		promptReader = bytes.NewBufferString("n\n")
	}
	promptWriter = &bytes.Buffer{}
	runServeDaemonFn = func(string) { daemon() }
	t.Cleanup(func() {
		isTTYFn = origIsTTY
		promptReader = origReader
		promptWriter = origWriter
		runServeDaemonFn = origDaemon
	})
}
```
Reuse this exact save/override/`t.Cleanup`-restore idiom for the wizard's serve-step tests, and for the "already running" (PID+health check) and "doctor FAILed, abort" branches, follow `daemon.go:71-74`'s check-then-early-exit shape.

**Windows-guard split (RESEARCH.md Pattern 4, confirmed structurally necessary):** `daemon.go` carries `//go:build !windows` with no counterpart — `runServeDaemonFn` (declared in the non-tagged `client.go:19`) is the only symbol the wizard's serve step may reference; do NOT call `runServeDaemon` directly. Recommend a `!windows`/`windows` file pair (`init_serve_unix.go` doing the real call via `runServeDaemonFn`, `init_serve_windows.go` printing the manual `nano-brain serve` instruction) mirroring `daemon.go`'s own build-tag convention (line 1: `//go:build !windows`).

---

### `cmd/nano-brain/init_register.go` (NEW — D-15) + `cmd/nano-brain/commands.go` (caller update)

**Analog:** `cmd/nano-brain/commands.go` `runInitCmd`'s `--root` branch (lines 29-131) and `triggerInitBackground` (134-161) — this IS the extraction source, not an analogous-but-different file.

**Registration HTTP pattern to extract into the new shared helper** (`commands.go:78-93`):
```go
body := map[string]string{"root_path": root}
if workspace != "" {
	body["workspace"] = workspace
}
data, err := json.Marshal(body)
if err != nil {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
resp, _, err := doRequest("POST", getBaseURL()+"/api/v1/init", bytes.NewReader(data))
if err != nil {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	cliLog.Error().Err(err).Str("cmd", "init").Msg("init request failed")
	os.Exit(1)
}
```
**Response-parsing + MCP-chain pattern to extract** (`commands.go:101-131`):
```go
var result struct {
	WorkspaceHash string `json:"workspace_hash"`
	RootPath      string `json:"root_path"`
	Name          string `json:"name"`
	AgentsSnippet string `json:"agents_snippet"`
}
if err := json.Unmarshal(resp, &result); err != nil { ... }
...
if shouldPromptMCPConfig(jsonFlag, isTTY()) {
	if result.Name == "" {
		fmt.Println("Warning: server did not return a workspace name ...")
	} else {
		promptMCPClientConfig(bufio.NewScanner(os.Stdin), result.RootPath, result.Name)
	}
}
triggerInitBackground(result.WorkspaceHash, root)
```
**Extraction approach:** carve this into a `registerWorkspace(root, workspace string, jsonFlag bool) (result, error)`-shaped helper (or similar) in `init_register.go`, called both by `commands.go`'s `--root` branch and the wizard's new register step — same package, no import needed, per RESEARCH.md Pitfall 5's explicit confirmation that `runInitCmd` lives in `commands.go` (verified, not `init.go`).

**`doRequest`/`getBaseURL` helpers** — unchanged, already shared infra (defined in `client.go`), reuse as-is.

---

### `internal/health/doctor/doctor.go` (D-13 skip-path edit)

**Analog:** existing `CheckEmbeddingProvider`/`CheckEmbeddingModel` (same file, lines 100-176) — in-place edit, not a new file.

**Current fallback-to-ollama pattern being guarded** (lines 109-111):
```go
if cfg.Provider == "" {
	cfg.Provider = "ollama"
}
```
**Skip-check pattern to mirror** (`CheckPgvector`'s "skip" convention already used elsewhere in `RunAll`, line 44):
```go
results = append(results, Check{Name: "pgvector", Status: "skip", Detail: "no connection"})
```
Insert, BEFORE the existing ollama-fallback line in both `CheckEmbeddingProvider` (line 109) and `CheckEmbeddingModel` (line 147):
```go
if cfg.Provider == "" {
	return Check{Name: "Embedding provider", Status: "skip", Detail: "disabled — BM25-only"}, nil
}
```
(and the `CheckEmbeddingModel` equivalent returning just `Check`, no `[]byte`).

**Test pattern to extend** (`doctor_test.go` table-driven style, e.g. `TestCheckBinaryExists_Missing` lines 28-34) — add `TestCheckEmbeddingProvider_Disabled`/`TestCheckEmbeddingModel_Disabled` following the same `Check{...}` field-assertion style already used by `TestCheckQueueHealth_*` (lines 51-71).

---

## Shared Patterns

### Prompt helpers (non-consequential vs. consequential)
**Source:** `cmd/nano-brain/init.go:255-267` (`promptWithDefault`) and `cmd/nano-brain/mcp_client_config.go:342-358` (`promptConsequential`)
**Apply to:** All new wizard step files.
```go
// Non-consequential (informational defaults, e.g. picking a model name):
func promptWithDefault(scanner *bufio.Scanner, prompt, defaultVal string) string { /* ... */ }

// Consequential (gates a write/process-start — EOF must mean decline, not accept):
func promptConsequential(scanner *bufio.Scanner, prompt, defaultVal string) (answer string, ok bool) {
	fmt.Printf("  %s [%s]: ", prompt, defaultVal)
	if !scanner.Scan() {
		return "", false
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return defaultVal, true
	}
	return input, true
}
```
Rule of thumb per the D-14/D-15/D-06 decisions: "start Docker container", "start server", "register workspace", "overwrite config" all gate real side effects → use `promptConsequential` + `isAffirmative` (below), not `promptWithDefault`.

### isAffirmative helper
**Source:** `cmd/nano-brain/mcp_client_config.go:229-231`
**Apply to:** Every new Y/n prompt across DB/embedding/serve/register steps, for consistency with the existing MCP-step convention.
```go
func isAffirmative(answer string) bool {
	return answer != "n" && answer != "N"
}
```

### Test-hook seam (package-level var override)
**Source:** `cmd/nano-brain/client.go:18-29`, exercised by `cmd/nano-brain/commands_test.go:500-525`
**Apply to:** `docker_provision.go` (`runDocker`), `init_db.go` (a `connectPostgresFn`/`waitForPostgresReadyFn`-style var if the planner wants injectable pgx), `init_serve_unix.go` (already covered by existing `runServeDaemonFn`, `isTTYFn`, `promptReader`).
```go
var runDocker dockerRunner = defaultRunDocker
// tests: save orig, override, t.Cleanup(restore) — exact shape as withRecoveryHooks
```

### Config file write permissions (0600 file / 0700 dir)
**Source:** `cmd/nano-brain/init.go:234-242`, mirrored again in `cmd/nano-brain/mcp_client_config.go:80-91` (`mergeJSONMCPEntry`)
**Apply to:** Any new file this phase writes (config.yml overwrite, no new MCP files — Phase 10 code is reused verbatim per D-16).
```go
if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil { ... }
if err := os.WriteFile(configPath, []byte(yaml), 0600); err != nil { ... }
// If overwriting a pre-existing file, WriteFile's mode arg is ignored by the OS —
// explicitly os.Chmod(configPath, 0600) afterward (see mcp_client_config.go:86-91).
```

### Doctor "skip" status convention
**Source:** `internal/health/doctor/doctor.go:44` (`CheckPgvector` no-connection case) and `RunAll`'s inline skip-append
**Apply to:** D-13's embedding-disabled skip path — use `Status: "skip"` (not `"ok"` or `"fail"`) so `doctor`'s CLI output and any JSON consumers can distinguish "intentionally not configured" from "passing check".

### Poll-loop with deadline/remaining/clamp
**Source:** `cmd/nano-brain/client_helpers.go:88-118` (`waitForServerHealthy`)
**Apply to:** `init_db.go`'s `waitForPostgresReady` (D-08) — identical deadline math, just swap the HTTP GET for `pgx.Connect`+`Ping`.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `cmd/nano-brain/docker_provision.go` (exit-code classification logic itself, e.g. distinguishing "not installed" vs "daemon down" vs "port conflict") | service | event-driven / process-exec | No prior `os/exec`-driven CLI-subprocess-with-classified-exit-codes exists in this codebase (`daemon.go` uses `os.StartProcess`/signals, a different exec model, not a foreground `docker` CLI wrapper). Use RESEARCH.md's verified empirical exit-code/stderr patterns (Patterns 1-2, Code Examples section) as the primary source of truth instead of a codebase analog — those were verified against a real Docker daemon in the research session. |
| `docs/SETUP_AGENT.md` rewrite (D-18) | docs | — | No code analog applicable; follow RESEARCH.md's recommended structure (prerequisites → install → init → verify → manual/troubleshooting appendix) and the existing doc's current section ordering as the only available reference. |

## Metadata

**Analog search scope:** `cmd/nano-brain/` (all `*.go` and `*_test.go`), `internal/health/doctor/` (`doctor.go`, `doctor_test.go`)
**Files scanned:** `init.go`, `commands.go`, `commands_test.go`, `client.go`, `client_helpers.go`, `daemon.go`, `mcp_client_config.go`, `doctor.go`, `doctor_test.go`
**Pattern extraction date:** 2026-07-02
