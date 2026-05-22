#!/usr/bin/env bash
# verify_install.sh — sanity-check that harness files installed correctly.
# Usage: verify_install.sh [TARGET_DIR]

set -uo pipefail

TARGET="${1:-$PWD}"
cd "$TARGET" 2>/dev/null || { echo "verify: cannot cd $TARGET" >&2; exit 1; }

PASS=0
FAIL=0

check_file() {
  local f="$1"
  if [[ -f "$f" ]]; then
    echo "  ✓ $f"
    PASS=$((PASS+1))
  else
    echo "  ✗ $f (missing)"
    FAIL=$((FAIL+1))
  fi
}

check_dir() {
  local d="$1"
  if [[ -d "$d" ]]; then
    echo "  ✓ $d/"
    PASS=$((PASS+1))
  else
    echo "  ✗ $d/ (missing)"
    FAIL=$((FAIL+1))
  fi
}

check_unrendered() {
  local f="$1"
  if [[ ! -f "$f" ]]; then return; fi
  local unrendered
  unrendered=$(grep -oE '\$\{[A-Z_][A-Z0-9_]*\}' "$f" 2>/dev/null | sort -u | head -5)
  if [[ -n "$unrendered" ]]; then
    echo "  ✗ $f contains unrendered placeholders:" >&2
    echo "$unrendered" | sed 's/^/      /' >&2
    FAIL=$((FAIL+1))
  else
    echo "  ✓ $f (all placeholders rendered)"
    PASS=$((PASS+1))
  fi
}

echo "=== Verify harness install at $TARGET ==="
echo
echo "[1] Required files exist:"
check_file "docs/HARNESS.md"
check_file "docs/FEATURE_INTAKE.md"
check_file "docs/HARNESS_BACKLOG.md"
check_dir  "docs/evidence"
check_file "docs/evidence/README.md"

echo
echo "[2] Placeholders fully rendered:"
check_unrendered "docs/HARNESS.md"
check_unrendered "docs/FEATURE_INTAKE.md"
check_unrendered "docs/HARNESS_BACKLOG.md"
check_unrendered "docs/evidence/README.md"

echo
echo "[3] Story template:"
if [[ -f "docs/templates/story.md" ]]; then
  check_unrendered "docs/templates/story.md"
elif [[ -f "docs/stories/story.md" ]]; then
  check_unrendered "docs/stories/story.md"
else
  echo "  ✗ story template not found at expected paths" >&2
  FAIL=$((FAIL+1))
fi

echo
echo "=== Summary ==="
echo "PASS: $PASS"
echo "FAIL: $FAIL"

if [[ $FAIL -gt 0 ]]; then
  exit 1
fi
exit 0
