#!/usr/bin/env bash
# PreToolUse hook (Bash matcher): safety net for the harness pre-merge gate.
# Blocks `gh pr create` while fast pre-merge checks FAIL (evidence/process
# gates only — HARNESS_FAST=1 skips build/test/lint, which the validation
# ladder covers separately). Respects [HARNESS-OVERRIDE] (R7).
#
# Exit 0 = allow, exit 2 = block (stderr is shown to the agent).
set -uo pipefail

command_text=$(jq -r '.tool_input.command // ""' 2>/dev/null || echo "")

# Only intercept PR creation.
case "$command_text" in
  *"gh pr create"*) ;;
  *) exit 0 ;;
esac

# R7 escape hatch: explicit override in the PR body being created.
if [[ "$command_text" == *"[HARNESS-OVERRIDE]"* ]]; then
  echo "harness: [HARNESS-OVERRIDE] present — pre-merge hook bypassed (R7). Document the override in docs/evidence/." >&2
  exit 0
fi

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || echo "")
checker="$repo_root/scripts/harness-check.sh"
[[ -x "$checker" ]] || exit 0  # no checker in this repo → not our concern

# Runner contract exit codes: 0=PASS, 1=FAIL, 2=SKIP(all), … — only [FAIL]
# lines block; an all-SKIP result (exit 2) must not.
out=$(cd "$repo_root" && HARNESS_FAST=1 "$checker" pre-merge --no-color 2>&1)

if echo "$out" | grep -q '^\[FAIL\]'; then
  {
    echo "harness: pre-merge gate FAILED — fix before creating the PR (or use [HARNESS-OVERRIDE]: <reason> per R7 with evidence)."
    echo "$out" | grep -E '^\[(FAIL|PASS|SKIP)\]' | head -20
  } >&2
  exit 2
fi

exit 0
