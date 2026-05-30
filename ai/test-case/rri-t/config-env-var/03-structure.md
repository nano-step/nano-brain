# PHASE 3: STRUCTURE — Q-A-R-P-T Test Cases

Format: **Q**uestion → **A**ction → expected **R**esult → **P**riority → **T**ag(s)
Priority: P0 (critical, blocks release), P1 (major, needs fix), P2 (minor, can defer).

Test instance: nano-brain server on port 8899 (custom config) — should be RUNNING throughout.
Binary: `/Users/tamlh/workspaces/self/AI/Tools/nano-brain/nano-brain`

---

## API Dimension

### TC-001 (P0, api,data) — Server boots on env-pointed port
- **Q**: Does `NANO_BRAIN_CONFIG=<file>` cause the server to bind the port declared in that file?
- **A**: `NANO_BRAIN_CONFIG=/tmp/nano-brain-custom/config.yml ./nano-brain serve` (already running)
- **R**: HTTP request to `http://localhost:8899/api/status` returns HTTP 200 with valid JSON; port 8899 reachable; default 3100 unaffected.

### TC-002 (P0, api) — /health endpoint healthy on custom port
- **Q**: Does the standard health endpoint work on the env-pointed instance?
- **A**: `curl -s -m 5 -o /dev/null -w "%{http_code}" http://localhost:8899/health`
- **R**: HTTP 200.

### TC-003 (P1, api) — /api/status reflects active provider from env-pointed config
- **Q**: Is the response payload generated from the env-pointed config (not the default)?
- **A**: `curl -s http://localhost:8899/api/status | grep -oE '"active_provider":"[^"]+"'`
- **R**: Returns provider declared in `/tmp/nano-brain-custom/config.yml` (`ollama`).

### TC-004 (P1, api) — Default 3100 instance still works (no regression)
- **Q**: Does the sibling default-port server still respond?
- **A**: `curl -s -m 5 http://localhost:3100/api/status`
- **R**: Either valid JSON or connection refused (if no sibling process running) — NEITHER server affects the other.

---

## Data Integrity Dimension

### TC-010 (P0, data) — `config show` displays env-pointed file
- **Q**: Does `config show` print the env-pointed config, not the default?
- **A**: `NANO_BRAIN_CONFIG=/tmp/nano-brain-custom/config.yml ./nano-brain config show 2>/dev/null | head -1`
- **R**: First line starts with `# /tmp/nano-brain-custom/config.yml`.

### TC-011 (P0, data) — `--config` flag overrides env var
- **Q**: When both set, does the flag win?
- **A**: `NANO_BRAIN_CONFIG=/nonexistent/decoy.yml ./nano-brain --config /tmp/nano-brain-custom/config.yml config show 2>/dev/null | head -1`
- **R**: First line shows `# /tmp/nano-brain-custom/config.yml` (flag, NOT decoy).

### TC-012 (P0, data) — Default fallback when neither set
- **Q**: With neither flag nor env, does it use `~/.nano-brain/config.yml`?
- **A**: `unset NANO_BRAIN_CONFIG; ./nano-brain config show 2>/dev/null | head -1`
- **R**: First line is `# /home/agent/.nano-brain/config.yml`.

### TC-013 (P1, data,edge) — Empty `NANO_BRAIN_CONFIG=""` falls back to default
- **Q**: Does empty string short-circuit to default (per skill spec)?
- **A**: `NANO_BRAIN_CONFIG="" ./nano-brain config show 2>/dev/null | head -1`
- **R**: First line is `# /home/agent/.nano-brain/config.yml` (default).

### TC-014 (P1, data) — Port value from env-pointed config lands in `nano-brain starting` log
- **Q**: Does the server log the port from the env-pointed config (not default)?
- **A**: `grep -E '"nano-brain starting"' /tmp/nano-brain-custom/server.out | head -1`
- **R**: Log line contains `"port":8899`.

### TC-015 (P0, data) — All 12 refactored commands respect env var
- **Q**: Do all of: `config show`, `doctor`, `status`, `bench` (subcommands), `init`, `migrate`, `client`, `cleanup-stale-raw`, `reset-embeddings`, `ops`, `main` (serve), use ResolveConfigPath?
- **A**: `grep -l "ResolveConfigPath" cmd/nano-brain/*.go | wc -l` AND for each, run command (where safe) with env var pointing to non-default.
- **R**: ≥ 9 files contain `ResolveConfigPath`; each runnable command honors env var.

---

## Edge Cases Dimension

### TC-020 (P1, edge) — Non-existent file path errors gracefully
- **Q**: Does env var pointing to nonexistent file produce clear error?
- **A**: `NANO_BRAIN_CONFIG=/tmp/does-not-exist-rrit-12345.yml ./nano-brain config show 2>&1`
- **R**: Non-zero exit; error message mentions either the path or "no such file"; no panic/stack trace.

### TC-021 (P1, edge,security) — Path traversal `/etc/passwd` doesn't crash
- **Q**: Bad-YAML target — does parser error gracefully?
- **A**: `NANO_BRAIN_CONFIG=/etc/passwd ./nano-brain config show 2>&1 | head -10`
- **R**: Non-zero exit; YAML/parse error message; no panic.

### TC-022 (P2, edge) — Directory (not file) as target
- **Q**: Pointing to a directory — graceful error?
- **A**: `NANO_BRAIN_CONFIG=/tmp ./nano-brain config show 2>&1`
- **R**: Non-zero exit; clear error (is-a-directory or read error).

### TC-023 (P2, edge) — Relative path resolved against CWD
- **Q**: `NANO_BRAIN_CONFIG=relative.yml` — does it look in CWD?
- **A**: `cd /tmp/nano-brain-custom && NANO_BRAIN_CONFIG=config.yml /Users/tamlh/workspaces/self/AI/Tools/nano-brain/nano-brain config show 2>/dev/null | head -1`
- **R**: Loads `/tmp/nano-brain-custom/config.yml` (relative resolved).

### TC-024 (P2, edge) — Symlink followed
- **Q**: Does `ResolveConfigPath` follow symlinks?
- **A**: `ln -sf /tmp/nano-brain-custom/config.yml /tmp/cfg-symlink.yml && NANO_BRAIN_CONFIG=/tmp/cfg-symlink.yml ./nano-brain config show 2>/dev/null | head -1`
- **R**: Loads target via symlink; output shows the env var path (or resolved path), not "broken".

### TC-025 (P1, edge) — Whitespace in env value
- **Q**: `NANO_BRAIN_CONFIG=" /path "` (leading/trailing spaces) — trimmed or treated literally?
- **A**: `NANO_BRAIN_CONFIG=" /tmp/nano-brain-custom/config.yml " ./nano-brain config show 2>&1 | head -3`
- **R**: EITHER trims and works, OR fails with clear error. Document actual behavior (no panic).

### TC-026 (P1, edge) — Flag empty, env set → falls to env
- **Q**: `--config ""` with env set — does empty-string flag short-circuit correctly to env?
- **A**: `NANO_BRAIN_CONFIG=/tmp/nano-brain-custom/config.yml ./nano-brain --config "" config show 2>/dev/null | head -1`
- **R**: First line is `# /tmp/nano-brain-custom/config.yml` (env wins because flag is empty per `ResolveConfigPath` spec).

---

## Infrastructure Dimension

### TC-030 (P0, infra,api) — Two servers concurrent (3100 + 8899) — isolation
- **Q**: Default + env-pointed instance running simultaneously — independent state?
- **A**: `curl -s http://localhost:8899/api/status` & `curl -s http://localhost:3100/api/status` (sibling) — compare workspace_count, queue_depth.
- **R**: Both respond OK; values can differ; no cross-port interference.

### TC-031 (P1, infra) — Log file path from env-pointed config is used
- **Q**: Does the server write to the `logging.file` declared in the custom config?
- **A**: `ls -la /tmp/nano-brain-custom/logs/`
- **R**: Either log file exists OR server.out shows the log path being used (config respected).

### TC-032 (P1, infra) — Container-style DB URL (host.docker.internal) works
- **Q**: Does the env-pointed config with `host.docker.internal:5432` DB URL successfully connect?
- **A**: Check `/api/status` for `pg_status`
- **R**: `pg_status: healthy`.

### TC-033 (P2, infra) — Docker example in README is syntactically valid
- **Q**: Does the Docker example in README parse and would work?
- **A**: `grep -A 6 "Docker example" README.md`
- **R**: Example contains `-e NANO_BRAIN_CONFIG=`, `-v /path`, valid bash.

---

## Performance Dimension

### TC-040 (P2, perf) — Config resolution adds < 5ms to startup
- **Q**: Does `ResolveConfigPath` introduce measurable overhead?
- **A**: `time ./nano-brain config show > /dev/null 2>&1` (3 trials each, env vs flag vs default)
- **R**: All three variants finish in < 1s; spread between variants < 100ms (env var read is negligible).

### TC-041 (P2, perf) — Server startup time unchanged
- **Q**: Does config resolution path affect time-to-listen on port?
- **A**: Time from process start to `"nano-brain starting"` log line.
- **R**: < 2s (rough heuristic; full startup includes DB ping, watcher init, etc.).

---

## Security Dimension

### TC-050 (P1, security) — Env var doesn't bypass YAML schema validation
- **Q**: Crafted malicious YAML via env-pointed file — schema validation still active?
- **A**: Create file with `server: { port: "not-a-number" }` → `NANO_BRAIN_CONFIG=/tmp/malicious.yml ./nano-brain config check 2>&1`
- **R**: Validation error (type mismatch); no silent acceptance.

### TC-051 (P2, security) — Resolved config path is logged (audit trail)
- **Q**: Is there an audit log line showing which config file was used?
- **A**: `grep -iE "config|loaded" /tmp/nano-brain-custom/server.out | head -5`
- **R**: At least one log line mentions the config file path or the loaded config OR `config show` displays it. Production deployments need this for forensics.

### TC-052 (P1, security) — Permission errors on unreadable config
- **Q**: Env points to file with `chmod 000` — graceful?
- **A**: `touch /tmp/locked.yml && chmod 000 /tmp/locked.yml && NANO_BRAIN_CONFIG=/tmp/locked.yml ./nano-brain config show 2>&1 ; chmod 644 /tmp/locked.yml; rm /tmp/locked.yml`
- **R**: Non-zero exit; permission-denied error; no panic.

### TC-053 (P2, security) — Process listing exposure (informational only)
- **Q**: Does `ps -ef` show `NANO_BRAIN_CONFIG=...`?
- **A**: `ps -E -ef | grep nano-brain` (on Linux env vars are visible via /proc/$pid/environ to same user)
- **R**: Env var visible in process environment (documented limitation; not a regression).

---

## Test Case Counts

| Dim | Count | P0 | P1 | P2 |
|---|---|---|---|---|
| API | 4 | 2 | 2 | 0 |
| Data Integrity | 6 | 4 | 2 | 0 |
| Edge Cases | 7 | 0 | 4 | 3 |
| Infrastructure | 4 | 1 | 2 | 1 |
| Performance | 2 | 0 | 0 | 2 |
| Security | 4 | 0 | 2 | 2 |
| **Total** | **27** | **7** | **12** | **8** |

P0 tests are the GO/NO-GO gate. ALL 7 P0 must PASS.
