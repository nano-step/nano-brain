#!/usr/bin/env bash
# setup_labels.sh — create harness labels on a GitHub repo.
# Usage: setup_labels.sh <owner/repo> [--dry-run]

set -uo pipefail

REPO="${1:-}"
DRY_RUN="false"
[[ "${2:-}" == "--dry-run" ]] && DRY_RUN="true"

if [[ -z "$REPO" ]]; then
  echo "Usage: setup_labels.sh <owner/repo> [--dry-run]" >&2
  exit 1
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "ERROR: gh CLI not installed" >&2
  exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
  echo "ERROR: gh not authenticated. Run 'gh auth login' first." >&2
  exit 1
fi

create_label() {
  local name="$1" color="$2" desc="$3"
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[dry-run] gh label create '$name' --repo $REPO --color $color --description '$desc'"
    return
  fi
  if gh label create "$name" --repo "$REPO" --color "$color" --description "$desc" --force >/dev/null 2>&1; then
    echo "✓ $name"
  else
    echo "✗ $name (failed)" >&2
  fi
}

echo "=== Creating harness labels on $REPO ==="

create_label "lane:tiny"       "c2e0c6" "Tiny lane (0-1 risk flags, patch direct)"
create_label "lane:normal"     "fbca04" "Normal lane (2-3 risk flags, proposal required)"
create_label "lane:high-risk"  "b60205" "High-risk lane (4+ flags or hard gate)"

create_label "change-type:user-feature"      "0e8a16" "User-visible feature/behavior"
create_label "change-type:bug-fix"           "d93f0b" "Fix to user-facing behavior"
create_label "change-type:infrastructure"    "5319e7" "Migrations, config, env, deploy"
create_label "change-type:refactor"          "bfd4f2" "Internal cleanup, no behavior change"
create_label "change-type:docs"              "0075ca" "Documentation only"
create_label "change-type:dependency-bump"   "fef2c0" "Library version updates"

create_label "status:proposal"     "ededed" "Proposal in progress"
create_label "status:in-progress"  "1d76db" "Implementation underway"
create_label "status:in-review"    "e99695" "Review Gate / PR Bot Review"
create_label "status:blocked"      "000000" "Waiting on human / external"

echo
echo "=== Done ==="
echo "Verify: gh label list --repo $REPO"
