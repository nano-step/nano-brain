---
name: "Harness+GSD: Autonomous Fix/Feature"
description: Drive a bugfix or feature request through the full Harness + GSD pipeline autonomously, per CLAUDE.md's Autonomous Delivery Protocol and docs/HARNESS.md
category: Workflow
tags: [workflow, harness, gsd, autonomous]
argument-hint: <description of the bug or change to make>
---

Drive a bugfix or change request through the full Harness + GSD pipeline, without stopping for approval at each step unless a decision genuinely requires the user.

**Input**: The text after `/harness-gsd` is the description of the bug/change to make. If no input is provided, ask what to fix (AskUserQuestion or an open question) before starting — do not guess.

**Source of truth — read these before running, do NOT copy their contents into the output:**
- `CLAUDE.md` § Autonomous Delivery Protocol — autonomy rules, when to stop and ask the user, mandatory evidence, commit/PR conventions.
- `docs/HARNESS.md` — the full pipeline: issue → lane → propose → deep-design → spec → implement → validate → user-flow test → review gate → PR+bot loop → ship.
- `.planning/ROADMAP.md` and `.planning/STATE.md` — check which phases already exist to avoid duplicating one.

This file is only a condensed pointer to the two files above. If they change, defer to their latest content — do not treat the steps below as authoritative over them.

**Steps**
1. Re-read `CLAUDE.md` and `docs/HARNESS.md` (rules may have changed since the last run).
2. Locate/create the phase: check `.planning/phases/` + `ROADMAP.md` for an existing phase matching the description; create a new one if none exists.
3. Classify the lane per Feature Intake (tiny / normal / high-risk):
   - tiny → patch directly + validate, no need to run the full GSD chain.
   - normal/high-risk → run `/gsd-discuss-phase → /gsd-deep-design → /gsd-plan-phase → /gsd-execute-phase → /gsd-verify-work → /gsd-ship-phase`, auto-answering the discuss/plan/execute prompts, without stopping for approval mid-way.
4. After execute: spawn an **independent** reviewer (not the same agent that wrote the code) — the author never self-approves.
5. Test with real evidence: `go test -race -short ./...` (+ `-tags=integration` when relevant) against `nanobrain_test`/`:3199` — never the dev DB `:3100`, never broad-kill processes. Paste the real output, never assume "should pass".
6. Ship: branch from `origin/master`, open a PR (`Closes #N` if an issue is linked), author `kokorolx`, no AI footers (no `Co-Authored-By`, no bot emoji).
7. If there are review/bot comments on the PR: read them, fix, re-test, push again until they pass.

**Guardrails**
- Only stop to ask the user when a decision is genuinely irreversible or a product/business choice — otherwise pick the sensible default and note it in the final summary.
- Never report "should work" without real output attached.

**Output**
Final summary: related phase/issue, what was changed, test evidence (real output), PR link.
