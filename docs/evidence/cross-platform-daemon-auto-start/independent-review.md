# Independent review — cross-platform daemon auto-start (#615)

Two review rounds by an independent reviewer subagent (R88 — the author never
self-approves in the same context).

## Round 1 — REQUEST-CHANGES

Findings: 0 CRITICAL, 7 MAJOR, 7 MINOR. No credential serialization, no
injection/escaping holes, atomic writes sound, retry pool verified no-leak.

All 7 MAJOR fixed in commit `cbecd05`:

| # | Finding | Fix |
|---|---|---|
| 1 | systemd `start` is a no-op on an active unit → update never reloaded the new binary | `register` uses `systemctl --user restart` |
| 2 | unregistered status left `supervisor_state=""`, missing remediation | `inactive` + "run 'nano-brain service install'" |
| 3 | status probed the interactive env endpoint, not the pinned config | sidecar marker `~/.nano-brain/service/config-path` |
| 4 | rollback restored the file but left a running service stopped | best-effort re-register of the restored definition |
| 5 | relative `NANO_BRAIN_BIN` pinned raw (launchd/systemd spawn from /) | `filepath.Abs` after validation |
| 6 | service log dir never created → launchd refused to start the job | `MkdirAll(logDir, 0700)` in launchd register |
| 7 | absent-state error matching too fragile for uninstall | match "No such process" (launchd) / "not loaded" (systemd) |

Minor fixes: npm wrapper signal re-raise, `next_retry` log accuracy, Windows
doc honesty, 0600 definition perms, managed recovery before the TTY gate.

## Round 2 — APPROVE-WITH-NITS

All 7 MAJOR re-verified against the working tree (table above, ✅ each).
Remaining items were MINOR/NIT; 4 of them closed in `f2b7e19`:

- status keeps the install remediation when the probe also errors
- uninstall removes the marker on the already-absent path
- rollback surfaces a failed restore-write
- unit tests for the launchd/systemd absent-state unregister branches

Verdict chain: REQUEST-CHANGES → APPROVE-WITH-NITS → APPROVE (nits closed).

Verification battery (reviewer + author, all green):
- `go test -race -short ./...` — PASS
- `node --test npm/run.test.js npm/postinstall.test.js` — 30 pass
- integration `TestNewPoolWithRetrySurvivesPostgreSQLDelay` against
  `nanobrain_test` — PASS
- macOS native smoke (isolated HOME/config, port 3199): install → launchd
  active → /health ready → status contract → kill -9 auto-restart → update →
  uninstall — PASS
