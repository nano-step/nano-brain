#!/usr/bin/env bash
#
# check-npm-release.sh — post-merge-npm-release gate checker
#
# Verifies that the latest GitHub release tag has been published to npm.
#
# Usage: ./scripts/check-npm-release.sh post-merge-npm-release [--json] [--feature=<id>]
#
# Exit codes (runner contract):
#   0=PASS, 1=FAIL, 2=SKIP, 3=WAITING, 5=ERROR

set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
GATE="post-merge-npm-release"
PHASE="${1:-}"
JSON_OUTPUT=false
FEATURE_ID=""

# Stats tracking
PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0
CHECKS=()

# Gate-level state (set during gate logic)
GATE_STATUS=""       # PASS|FAIL|SKIP|WAITING
WAIT_SECONDS=""
INSTRUCTIONS=""

# ============================================================================
# Helper Functions
# ============================================================================

colorize() {
    local status=$1
    local msg=$2
    case "$status" in
        PASS) echo -e "${GREEN}[PASS]${NC} $msg" ;;
        FAIL) echo -e "${RED}[FAIL]${NC} $msg" ;;
        SKIP) echo -e "${YELLOW}[SKIP]${NC} $msg" ;;
        WAITING) echo -e "${YELLOW}[WAITING]${NC} $msg" ;;
        *) echo "[$status] $msg" ;;
    esac
}

add_check() {
    local status=$1
    local desc=$2

    if [[ "$JSON_OUTPUT" != true ]]; then
        colorize "$status" "$desc"
    fi

    case "$status" in
        PASS) PASS_COUNT=$((PASS_COUNT + 1)) ;;
        FAIL) FAIL_COUNT=$((FAIL_COUNT + 1)) ;;
        SKIP) SKIP_COUNT=$((SKIP_COUNT + 1)) ;;
    esac

    CHECKS+=("$status|$desc")
}

cmd_exists() {
    command -v "$1" &>/dev/null
}

emit_json() {
    local status="$1"
    local next_gate="null"

    if [[ "$status" == "PASS" ]]; then
        next_gate="\"next-ready\""
    fi

    printf '{\n'
    printf '  "gate": "%s",\n' "$GATE"
    printf '  "status": "%s",\n' "$status"
    printf '  "checks": [\n'

    local i=0
    local total=${#CHECKS[@]}
    for check in "${CHECKS[@]}"; do
        IFS='|' read -r chk_status desc <<< "$check"
        i=$((i + 1))
        local id_field=""
        local name_field="$desc"
        if [[ "$desc" =~ ^([0-9]+\.[0-9]+)\ (.*)$ ]]; then
            id_field="${BASH_REMATCH[1]}"
            name_field="${BASH_REMATCH[2]}"
        fi
        local comma=""
        [[ $i -lt $total ]] && comma=","
        printf '    {"id": "%s", "name": "%s", "status": "%s"}%s\n' \
            "$id_field" "$name_field" "$chk_status" "$comma"
    done

    printf '  ],\n'
    printf '  "next_gate": %s,\n' "$next_gate"

    if [[ -n "$WAIT_SECONDS" ]]; then
        printf '  "wait_seconds": %s,\n' "$WAIT_SECONDS"
    fi

    printf '  "rule_ids_violated": []'

    if [[ -n "$INSTRUCTIONS" ]]; then
        local escaped="${INSTRUCTIONS//\"/\\\"}"
        escaped="${escaped//$'\n'/\\n}"
        printf ',\n  "instructions_for_agent": "%s"' "$escaped"
    fi

    printf '\n}\n'
}

# ============================================================================
# Gate Logic
# ============================================================================

run_gate() {
    [[ "$JSON_OUTPUT" != true ]] && echo "─ POST-MERGE-NPM-RELEASE checks"

    # --- Check 1.1: GH release tag exists ---
    local gh_tag=""
    if ! cmd_exists gh; then
        add_check "SKIP" "1.1 GH release tag exists"
        add_check "SKIP" "1.2 release.yml not in-progress"
        add_check "SKIP" "1.3 npm version matches GH tag"
        GATE_STATUS="SKIP"
        return
    fi

    gh_tag=$(gh release list --limit 1 --json tagName --jq '.[0].tagName' 2>/dev/null || true)

    if [[ -z "$gh_tag" ]]; then
        add_check "SKIP" "1.1 GH release tag exists"
        add_check "SKIP" "1.2 release.yml not in-progress"
        add_check "SKIP" "1.3 npm version matches GH tag"
        GATE_STATUS="SKIP"
        return
    fi

    add_check "PASS" "1.1 GH release tag exists"

    # --- Check 1.2: release.yml not in-progress ---
    local workflow_status=""
    workflow_status=$(gh run list --workflow=release.yml --limit 1 --json status --jq '.[0].status' 2>/dev/null || true)

    if [[ "$workflow_status" == "in_progress" ]]; then
        add_check "FAIL" "1.2 release.yml not in-progress"
        add_check "SKIP" "1.3 npm version matches GH tag"
        GATE_STATUS="WAITING"
        WAIT_SECONDS=60
        INSTRUCTIONS="release.yml still running — check with: gh run list --workflow=release.yml"
        return
    fi

    add_check "PASS" "1.2 release.yml not in-progress"

    # --- Check 1.3: npm version matches GH tag ---
    local npm_version=""
    npm_version=$(npm view @nano-step/nano-brain version 2>/dev/null || true)

    # Strip leading 'v' from tag for comparison
    local tag_version="${gh_tag#v}"

    if [[ -z "$npm_version" ]]; then
        add_check "FAIL" "1.3 npm version matches GH tag"
        GATE_STATUS="FAIL"
        INSTRUCTIONS="Could not fetch npm version for @nano-step/nano-brain. The release workflow may not have run yet or npm publish may have failed.

Check release workflow status:
  gh run list --workflow=release.yml

If the workflow failed, check the logs:
  gh run view --log-failed

To re-trigger: re-push the tag or fix the release workflow and push a new tag.
Expected npm version: $tag_version (from GH tag: $gh_tag)"
        return
    fi

    if [[ "$tag_version" == "$npm_version" ]]; then
        add_check "PASS" "1.3 npm version matches GH tag"
        GATE_STATUS="PASS"
    else
        add_check "FAIL" "1.3 npm version matches GH tag"
        GATE_STATUS="FAIL"
        INSTRUCTIONS="npm version ($npm_version) does not match GH tag ($gh_tag / $tag_version).

The release workflow may still be running or may have failed.

Steps to diagnose:
1. Check recent workflow runs:
     gh run list --workflow=release.yml
2. View logs for the latest run:
     gh run view --log-failed
3. If in_progress: wait ~2 min and re-run this gate.
4. If failed: check the release.yml logs, fix the issue, then either:
   - Re-push the existing tag: git push origin $gh_tag
   - Or push a new commit to master (auto-tag will create a new tag)"
    fi
}

# ============================================================================
# Main
# ============================================================================

main() {
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --json)
                JSON_OUTPUT=true
                shift
                ;;
            --feature=*)
                FEATURE_ID="${1#--feature=}"
                shift
                ;;
            post-merge-npm-release)
                # Phase name — consumed, already set as GATE
                shift
                ;;
            --help)
                sed -n '2,10p' "$0" | sed 's/^# //'
                exit 0
                ;;
            *)
                shift
                ;;
        esac
    done

    run_gate

    # Resolve final status if not already set
    if [[ -z "$GATE_STATUS" ]]; then
        if [[ $FAIL_COUNT -eq 0 ]]; then
            if [[ $SKIP_COUNT -gt 0 && $PASS_COUNT -eq 0 ]]; then
                GATE_STATUS="SKIP"
            else
                GATE_STATUS="PASS"
            fi
        else
            GATE_STATUS="FAIL"
        fi
    fi

    if [[ "$JSON_OUTPUT" == true ]]; then
        emit_json "$GATE_STATUS"
    else
        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        colorize "$GATE_STATUS" "Gate: $GATE — $GATE_STATUS"
        if [[ -n "$INSTRUCTIONS" ]]; then
            echo ""
            echo "$INSTRUCTIONS"
        fi
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    fi

    # Exit codes: 0=PASS, 1=FAIL, 2=SKIP, 3=WAITING, 5=ERROR
    case "$GATE_STATUS" in
        PASS)    exit 0 ;;
        FAIL)    exit 1 ;;
        SKIP)    exit 2 ;;
        WAITING) exit 3 ;;
        *)       exit 5 ;;
    esac
}

main "$@"
