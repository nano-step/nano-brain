# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**All project context lives in [AGENTS.md](AGENTS.md).** Read that file for architecture, conventions, testing, and agent-oriented design principles.

## Autonomous Delivery Protocol (DEFAULT MODE)

When Tâm gives an implementation/bugfix directive — "fix …", "giải quyết …", "làm cái này/cái kia", or any request to change behavior (not a question) — drive it to **DONE** through the full GSD pipeline. Do not stop at each gate for approval; Tâm invokes rarely on purpose. Decide and proceed.

**Flow per request:**
1. **Locate/create the phase.** Existing backlog/roadmap phase → use it. New work → add a phase first (`/gsd-phase` add, or `/gsd-capture --backlog`).
2. **Run autonomously:** `/gsd-autonomous --only <N>` → discuss→plan→execute, auto-answered. In discuss, **spawn subagents** (`Explore`/scout) to find the root cause, form your own recommendation, and lock the decisions yourself. `workflow.auto_advance: true` is set so phases self-advance.
3. **Independent review:** after execute, spawn a **separate** reviewer agent (`gsd-code-reviewer` or `oh-my-claudecode:code-reviewer`) — the author never self-approves in the same context (R88). Apply the fixes it finds.
4. **Test with evidence:** `go test -race -short ./...` (+ `-tags=integration` when relevant) against **nanobrain_test / :3199** — never the dev DB / :3100, never broad-kill processes. Paste the real output.
5. **Ship:** `/gsd-ship` → branch from `origin/master`, open the PR.
6. **PR comments:** address review comments autonomously, re-test, push.

**Stop to ask Tâm ONLY when** a decision is genuinely his: irreversible, a business/product choice, or a real blocker. Everything else — pick the sensible default, note it in passing, keep going.

**Evidence is mandatory at every claim.** Test output, reviewer report, verification report. No "should work" — show it ran. If tests fail, say so with the output.

Commits and PRs: author `kokorolx`, **no AI footers** (no `Co-Authored-By`, no 🤖).

## Quick Reference

```bash
CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain   # Build
go test -race -short ./...                                 # Unit tests
go test -race -tags=integration ./...                      # Integration tests
sqlc generate                                              # SQL codegen
make generate-openapi                                      # OpenAPI spec regeneration
```
