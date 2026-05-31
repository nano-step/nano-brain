# Harness Backlog

<!-- generated-by: harness-init v0.1.0 -->

Use this file when an agent discovers a missing harness capability but should
not change the operating model immediately.

## Template

```md
## Missing Harness Capability

### Title

Short name.

### Discovered While

Task or story that exposed the gap.

### Current Pain

What was hard, repeated, ambiguous, or unsafe?

### Suggested Improvement

What should be added or changed?

### Risk

Tiny, normal, or high-risk.

### Status

proposed | accepted | implemented | rejected
```

## Items

---

## Missing Harness Capability

### Title

Self-review before push: struct field population check

### Discovered While

Symbol extraction feature (Pillar 1, commit `40c87c6`). REST handler and MCP tool both defined fields (`kind`, `language`, `signature`) in response structs but populated only `name` and `source_path`. The other fields silently returned empty strings. Caught during post-commit self-review, not before push.

### Current Pain

Implementation was considered "done" after `go build + go test` passed. No step verified that the response payload actually contained the correct data shape — only that the code compiled and unit tests passed. The bug only appeared during manual code reading.

### Suggested Improvement

Add a mandatory self-review checklist item to the validation ladder for `user-feature` change type:

```text
self-review:response-shape   (user-feature only)
  For each new REST endpoint and MCP tool:
  1. Read the response struct definition.
  2. Read the mapping loop that populates it.
  3. Verify every declared field is explicitly assigned (no zero-value gaps).
  4. If a field is populated from a secondary source (e.g. JSONB metadata), verify
     the unmarshal path exists and is exercised.
  This check runs BEFORE push, not after. It takes < 2 minutes and catches
  "struct has fields but loop doesn't fill them" bugs that tests won't catch.
```

Add to HARNESS.md validation ladder under `validate:quick`.

### Risk

Tiny

### Status

implemented — added `self-review:response-shape` and `self-review:staged-files` to validation ladder in HARNESS.md.

---

## Missing Harness Capability

### Title

Gitignore gate: block rogue files before commit

### Discovered While

Symbol extraction commit (`40c87c6`) accidentally included `.opencode/worktree-sessions.json`, `.opencode/.repo-id`, `.opencode/worktrees/...` (embedded git submodule), `docs/AGENTS_SNIPPET.md`, `docs/SKILL.md` (empty files), and `package-lock.json`. These were staged by `git add -A` without review.

### Current Pain

`git add -A` is convenient but indiscriminate. Worktree metadata, OpenCode internal files, and empty scaffolding files ended up in the PR diff, adding noise and a spurious embedded-git-repo warning from git.

### Suggested Improvement

1. Add the following to `.gitignore`:
   ```
   .opencode/worktrees/
   .opencode/worktree-sessions.json
   .opencode/.repo-id
   package-lock.json
   ```
2. Add a pre-push gate step: `git diff --cached --name-only` — agent reads the list and explicitly confirms no `.opencode/` metadata, no `package-lock.json`, no empty doc scaffolds before committing.
3. Never use `git add -A` without running `git status` first and reading the staged file list.

### Risk

Tiny

### Status

implemented — added `self-review:staged-files` gate to validation ladder in HARNESS.md. `.gitignore` already updated in commit `afb0d2f`.

---

## Missing Harness Capability

### Title

Extractor init errors must be logged, never silently ignored

### Discovered While

Symbol extraction wiring in `main.go` used `goE, _ := symbol.NewGoExtractor()` — errors from all 4 extractor constructors were silently dropped with `_`. If any extractor fails (e.g. bad tree-sitter grammar init), the registry receives a nil extractor. Calling `Extract` on nil would panic at runtime. Project constraint: "mọi action đều cần có log."

### Current Pain

Silent `_` on constructor errors violates the logging constraint and creates a latent nil-panic risk. The pattern is easy to miss during review because the code compiles and tests pass.

### Suggested Improvement

Add to Forbidden Practices (#14):

> **`_ = err` on constructor calls in `main.go` or any startup path.** Use `log.Warn` + skip the nil value, or `log.Fatal` if the component is critical. The `_` discard is only permitted in deferred cleanup (e.g. `defer f.Close()`).

Concrete pattern for optional extractors:
```go
goE, err := symbol.NewGoExtractor()
if err != nil {
    logger.Warn().Err(err).Msg("go extractor init failed, skipping")
}
// Pass only non-nil extractors to registry
```

### Risk

Tiny

### Status

implemented — added Forbidden Practice #14 to HARNESS.md. Constructor error logging already applied in `main.go` commit `afb0d2f`.

---

## Live rescan of db_root on each harvest tick

### Problem

`buildOpenCodeHarvesters` runs once at startup. New per-project OpenCode DBs
added after the daemon starts are not discovered until restart.

### Proposed solution

On each `Runner.Run` tick (or a dedicated rescan ticker), call
`ScanOpenCodeDBRoot` again and diff against the current harvester set.
Add new harvesters; remove stale ones (workspace deregistered).

### Risk

Normal — touches `Runner` internals and liveness semantics. Defer until
a user reports needing it.

### Status

deferred — startup-only discovery chosen for v1 (simpler, lower risk).
Inline TODO comment in `buildOpenCodeHarvesters` references this entry.
Tracking: opencode-multi-db-discovery (#199), Task 5.

---

## Harness Capability — IMPLEMENTED (May 2026)

### Title

Explicit & enforced harness rules pass

### Discovered While

Comparison with `hoangnb24/harness-experimental` and `harness-benchmark`
revealed 89 rules in our harness; only 54% were both explicit AND enforced.
21 were ambiguous but enforced; 15 were explicit but orphaned; 5 were both
ambiguous and unenforced (exploitable); 11 retro-gate checks were placeholders.

### Current Pain

- Agents could not predict gate behavior because rules were vague
  ("substantive comment", "fix if cheap", "stale issue")
- Self-review evidence had no required structure → reviewer had to guess
- Retro Gate ⑥ returned PASS without computing anything ("metric collection pending")
- No closed-set vocabulary → terms drifted across docs

### Applied Improvement

W1.1 — Fixed 5 ambiguous+unenforced rules with explicit conditions:
- R2: PR closes exactly 1 issue (gate 3.8)
- R7: `[HARNESS-OVERRIDE]: <reason>` literal string mechanism (gate 3.6)
- R31: Agent-triaged Gemini verdict vocabulary (closed set) (gate 3.6)
- R56: Dropped 15-min effort threshold; verdict-based decisions only
- R89: Measurable skip-issue conditions (git diff HEAD~1 HEAD) (gate 1.3)

W1.2 — Replaced 11 retro-gate placeholders with real metric computation:
- 6.1 Merged PRs for epic (from gh pr list --search)
- 6.2 Avg PR cycles (commits per PR; FAIL if > 2.5)
- 6.3 CI failures on b-main (FAIL if > 5)
- 6.4 Retro file exists with min 200 words
- 6.5 Retro contains required sections (Metrics, Patterns, Root Cause, Proposed Changes)

W2 — Added gate checks for 5 high-value orphans:
- 2.4 upgraded to enforce TRACE_SPEC Tier 2 required sections
- 3.5 upgraded to verify literal `Review Verdict: PASS`
- 3.11 new — max 3 PR commits (R29)
- 3.12 new — smoke:e2e evidence with curl/HTTP body (R19, R20)
- 4.3 upgraded — archive blocked unless Review Verdict: PASS exists (R28)

W2 — Demoted 5 unautomatable rules to HUMAN-ONLY section in HARNESS.md
(R14, R26, R30, R84, R87) — explicit-ly marked, not silently orphaned.

W3 — Standardized self-review evidence format (TRACE_SPEC.md Tier 2) and
added YAML frontmatter to docs/templates/story.md for machine-readable
validation tracking.

W4 — Created supporting docs:
- docs/GLOSSARY.md (closed-set vocabulary, 30+ terms)
- docs/CONTEXT_RULES.md (phase × lane retrieval, token budgets)
- docs/TRACE_SPEC.md (3-tier trace specification)
- docs/decisions/ (ADR folder + README + template)
- Formalized Two-Output Model (product delta + harness delta) in HARNESS.md

### Status

implemented — May 30, 2026. Net effect: rule traceability via R-IDs in gate
FAIL messages, ~76% of rules now automated (was 54%), retro gate computes
real metrics, vocabulary closed-set defined. Token budget for tasks reduced
~10-15% via CONTEXT_RULES.md phase × lane reading rules.

Tracking: in-session harness update (no GitHub issue per R89 — this work
was the harness rule update itself; the agent's task IS the rule fix).
