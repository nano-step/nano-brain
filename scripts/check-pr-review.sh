#!/usr/bin/env bash
#
# check-pr-review.sh — async-pr-review gate checker
#
# Verifies that the open PR on the current branch has received bot review
# (e.g. Gemini), unresolved findings have been triaged, and CI is green.
#
# Usage: ./scripts/check-pr-review.sh async-pr-review [--json] [--feature=<id>]
#
# Exit codes (runner contract):
#   0=PASS, 1=FAIL, 2=SKIP, 3=WAITING, 4=BLOCKED, 5=ERROR

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

GATE="async-pr-review"
JSON_OUTPUT=false
FEATURE_ID=""

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0
CHECKS=()

GATE_STATUS=""
WAIT_SECONDS=""
INSTRUCTIONS=""
NEXT_GATE_OVERRIDE=""

PR_NUMBER=""
GEMINI_COUNT=0
GEMINI_SOAK_SECONDS=180

colorize() {
    local status=$1
    local msg=$2
    case "$status" in
        PASS) echo -e "${GREEN}[PASS]${NC} $msg" ;;
        FAIL) echo -e "${RED}[FAIL]${NC} $msg" ;;
        SKIP) echo -e "${YELLOW}[SKIP]${NC} $msg" ;;
        WAITING) echo -e "${YELLOW}[WAITING]${NC} $msg" ;;
        BLOCKED) echo -e "${RED}[BLOCKED]${NC} $msg" ;;
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
    if [[ -n "$NEXT_GATE_OVERRIDE" ]]; then
        next_gate="\"$NEXT_GATE_OVERRIDE\""
    elif [[ "$status" == "PASS" ]]; then
        next_gate="\"post-merge\""
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
        if [[ "$desc" =~ ^([0-9]+\.[0-9]+(\.[0-9]+)?)\ (.*)$ ]]; then
            id_field="${BASH_REMATCH[1]}"
            name_field="${BASH_REMATCH[3]}"
        fi
        local comma=""
        [[ $i -lt $total ]] && comma=","
        local escaped_name="${name_field//\"/\\\"}"
        printf '    {"id": "%s", "name": "%s", "status": "%s"}%s\n' \
            "$id_field" "$escaped_name" "$chk_status" "$comma"
    done

    printf '  ],\n'
    printf '  "next_gate": %s,\n' "$next_gate"

    if [[ -n "$WAIT_SECONDS" ]]; then
        printf '  "wait_seconds": %s,\n' "$WAIT_SECONDS"
    fi

    printf '  "rule_ids_violated": []'

    if [[ -n "$INSTRUCTIONS" ]]; then
        local escaped="${INSTRUCTIONS//\\/\\\\}"
        escaped="${escaped//\"/\\\"}"
        escaped="${escaped//$'\n'/\\n}"
        printf ',\n  "instructions_for_agent": "%s"' "$escaped"
    fi

    printf '\n}\n'
}

# Gate checks
check_pr_state() {
    if ! cmd_exists gh; then
        add_check "SKIP" "3.5.1 gh CLI not available"
        GATE_STATUS="SKIP"
        return 2
    fi

    local pr_info
    pr_info=$(gh pr view --json state,isDraft,number \
        --jq '"\(.state) \(.isDraft) \(.number)"' 2>/dev/null) || pr_info=""

    if [[ -z "$pr_info" ]]; then
        add_check "FAIL" "3.5.1 No open PR found on current branch"
        GATE_STATUS="FAIL"
        INSTRUCTIONS="No PR found on current branch. Push the branch and open a PR:
  gh pr create --base master --fill"
        return 1
    fi

    local state is_draft pr_num
    read -r state is_draft pr_num <<< "$pr_info" || true

    PR_NUMBER="$pr_num"

    if [[ "$state" == "MERGED" ]]; then
        add_check "PASS" "3.5.1 PR #$pr_num already MERGED — skipping review gate"
        GATE_STATUS="PASS"
        NEXT_GATE_OVERRIDE="post-merge"
        return 2
    fi

    if [[ "$state" == "CLOSED" ]]; then
        add_check "FAIL" "3.5.1 PR #$pr_num is CLOSED (not merged)"
        GATE_STATUS="FAIL"
        INSTRUCTIONS="PR was closed without merge. Reopen with 'gh pr reopen $pr_num' or create a new PR."
        return 1
    fi

    if [[ "$is_draft" == "true" ]]; then
        add_check "FAIL" "3.5.1 PR #$pr_num is DRAFT"
        GATE_STATUS="BLOCKED"
        INSTRUCTIONS="PR is in DRAFT state. Mark ready for review:
  gh pr ready $pr_num"
        return 1
    fi

    add_check "PASS" "3.5.1 PR #$pr_num exists and is OPEN"
    return 0
}
check_merge_conflicts() {
    local mergeable
    mergeable=$(gh pr view "$PR_NUMBER" --json mergeable --jq '.mergeable' 2>/dev/null || echo "UNKNOWN")

    case "$mergeable" in
        MERGEABLE)
            add_check "PASS" "3.5.2 No merge conflicts"
            return 0
            ;;
        CONFLICTING)
            add_check "FAIL" "3.5.2 PR has merge conflicts"
            GATE_STATUS="BLOCKED"
            INSTRUCTIONS="PR has merge conflicts with master. Rebase:
  git fetch origin && git rebase origin/master
Resolve conflicts, force-push the branch, then re-run this gate."
            return 1
            ;;
        UNKNOWN|"")
            add_check "FAIL" "3.5.2 Mergeable state UNKNOWN (GitHub still computing)"
            GATE_STATUS="WAITING"
            WAIT_SECONDS=30
            return 1
            ;;
        *)
            add_check "SKIP" "3.5.2 Mergeable state: $mergeable (unhandled)"
            return 0
            ;;
    esac
}
check_gemini_review() {
    local review_count
    review_count=$(gh pr view "$PR_NUMBER" --json reviews \
        --jq '[.reviews[] | select(.author.login | test("gemini|copilot|coderabbit";"i"))] | length' \
        2>/dev/null || echo "0")

    if [[ "$review_count" -gt 0 ]]; then
        add_check "PASS" "3.5.3 Bot review posted ($review_count review event(s))"
        GEMINI_COUNT="$review_count"
        return 0
    fi

    local last_commit_at now_epoch last_commit_epoch age_seconds
    last_commit_at=$(gh pr view "$PR_NUMBER" --json commits --jq '.commits[-1].committedDate' 2>/dev/null || echo "")

    if [[ -n "$last_commit_at" ]]; then
        now_epoch=$(date +%s)
        last_commit_epoch=$(date -d "$last_commit_at" +%s 2>/dev/null || date -j -f "%Y-%m-%dT%H:%M:%SZ" "$last_commit_at" +%s 2>/dev/null || echo "$now_epoch")
        age_seconds=$((now_epoch - last_commit_epoch))

        if [[ "$age_seconds" -ge "$GEMINI_SOAK_SECONDS" ]]; then
            add_check "PASS" "3.5.3 No bot review after ${age_seconds}s since last push (clean / no bot configured)"
            GEMINI_COUNT=0
            return 0
        fi

        local remaining=$((GEMINI_SOAK_SECONDS - age_seconds))
        add_check "FAIL" "3.5.3 Waiting for bot review (${age_seconds}s since last push, soak ${GEMINI_SOAK_SECONDS}s)"
        GATE_STATUS="WAITING"
        WAIT_SECONDS=$(( remaining < 30 ? 30 : remaining ))
        return 1
    fi

    add_check "FAIL" "3.5.3 Could not determine last commit time; waiting once"
    GATE_STATUS="WAITING"
    WAIT_SECONDS=30
    return 1
}
check_findings_resolved() {
    if [[ "$GEMINI_COUNT" -eq 0 ]]; then
        add_check "PASS" "3.5.4 No bot findings to resolve"
        return 0
    fi

    local override
    override=$(gh pr view "$PR_NUMBER" --json comments \
        --jq '[.comments[] | select(.body | test("\\[HARNESS-OVERRIDE\\]"))] | length' \
        2>/dev/null || echo "0")
    if [[ "$override" -gt 0 ]]; then
        add_check "PASS" "3.5.4 Bot findings overridden via [HARNESS-OVERRIDE] comment"
        return 0
    fi

    local current_branch slug
    current_branch=$(git branch --show-current 2>/dev/null || echo "")
    slug="${current_branch#*/}"

    local triage_file=""
    if [[ -d docs/evidence ]]; then
        triage_file=$(find docs/evidence -type f \( -name "self-review*${slug}*.md" -o -name "*${slug}*review*.md" -o -name "review-*${slug}*.md" \) 2>/dev/null | head -1)
    fi

    if [[ -z "$triage_file" ]]; then
        add_check "FAIL" "3.5.4 ${GEMINI_COUNT} bot review(s) but no triage file"
        GATE_STATUS="FAIL"
        INSTRUCTIONS="Bot posted ${GEMINI_COUNT} review event(s) but no triage file found under docs/evidence/.
Run the code-review skill to generate a triage table for PR #${PR_NUMBER}:
  Identify each comment as VALID:{critical|high|medium|low} or INVALID:<reason>
  For VALID:critical/high, fix and append 'fixed in commit <sha>' to the row.
Then re-run this gate."
        return 1
    fi

    local unresolved=0
    if grep -qiE 'VALID:(critical|high)' "$triage_file" 2>/dev/null; then
        unresolved=$(awk '
            BEGIN { in_triage=0; count=0 }
            /^##+[[:space:]]+(Gemini|Bot|Triage)/ { in_triage=1; next }
            /^##[[:space:]]/ && in_triage { in_triage=0 }
            in_triage && /^\|/ {
                line = tolower($0)
                if (line ~ /valid:(critical|high)/) {
                    if (line !~ /fixed in commit [a-f0-9]{6,}/ && line !~ /resolved in commit [a-f0-9]{6,}/) count++
                }
            }
            END { print count }
        ' "$triage_file")
    fi

    if [[ "$unresolved" -gt 0 ]]; then
        add_check "FAIL" "3.5.4 ${unresolved} VALID:critical/high finding(s) unresolved in ${triage_file}"
        GATE_STATUS="FAIL"
        INSTRUCTIONS="${unresolved} VALID:critical/high finding(s) in ${triage_file} lack 'fixed in commit <sha>'.
Read each unresolved row → fix the code → commit → append the commit SHA to the row → push.
Then re-run this gate."
        return 1
    fi

    add_check "PASS" "3.5.4 All bot findings resolved or overridden (triage: ${triage_file})"
    return 0
}
check_ci_status() {
    local ci_info
    ci_info=$(gh pr view "$PR_NUMBER" --json statusCheckRollup --jq '
        if .statusCheckRollup == null then "0 0 0"
        else
          (.statusCheckRollup | length) as $total |
          ([.statusCheckRollup[]? | select(.conclusion == "FAILURE" or .conclusion == "CANCELLED" or .conclusion == "TIMED_OUT" or .conclusion == "ACTION_REQUIRED")] | length) as $fail |
          ([.statusCheckRollup[]? | select(.status == "QUEUED" or .status == "IN_PROGRESS" or .status == "PENDING" or .status == "WAITING")] | length) as $pending |
          "\($total) \($fail) \($pending)"
        end
    ' 2>/dev/null || echo "0 0 0")

    local total fail_count pending_count
    read -r total fail_count pending_count <<< "$ci_info" || true

    if [[ "$total" -eq 0 ]]; then
        add_check "SKIP" "3.5.5 No CI checks configured on PR"
        return 0
    fi

    if [[ "$fail_count" -gt 0 ]]; then
        add_check "FAIL" "3.5.5 ${fail_count}/${total} CI check(s) failed"
        GATE_STATUS="FAIL"
        INSTRUCTIONS="${fail_count} CI check(s) failed on PR #${PR_NUMBER}. Inspect:
  gh pr checks ${PR_NUMBER}
  gh run view --log-failed
Fix the failing checks, push, then re-run this gate."
        return 1
    fi

    if [[ "$pending_count" -gt 0 ]]; then
        add_check "FAIL" "3.5.5 ${pending_count}/${total} CI check(s) in progress"
        GATE_STATUS="WAITING"
        WAIT_SECONDS=30
        return 1
    fi

    add_check "PASS" "3.5.5 All ${total} CI check(s) passing"
    return 0
}

run_gate() {
    [[ "$JSON_OUTPUT" != true ]] && echo "─ ASYNC-PR-REVIEW checks"

    check_pr_state || true
    if [[ -n "$GATE_STATUS" ]]; then return; fi

    check_merge_conflicts || true
    if [[ -n "$GATE_STATUS" && "$GATE_STATUS" != "" ]]; then return; fi

    check_gemini_review || true
    if [[ -n "$GATE_STATUS" && "$GATE_STATUS" != "" ]]; then return; fi

    check_findings_resolved || true
    if [[ -n "$GATE_STATUS" && "$GATE_STATUS" != "" ]]; then return; fi

    check_ci_status || true
    if [[ -n "$GATE_STATUS" && "$GATE_STATUS" != "" ]]; then return; fi

    GATE_STATUS="PASS"
}

main() {
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
            async-pr-review)
                shift
                ;;
            --help)
                sed -n '2,12p' "$0" | sed 's/^# //; s/^#$//'
                exit 0
                ;;
            *)
                shift
                ;;
        esac
    done

    run_gate

    if [[ -z "$GATE_STATUS" ]]; then
        if [[ $FAIL_COUNT -eq 0 ]]; then
            GATE_STATUS="PASS"
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
            echo -e "$INSTRUCTIONS"
        fi
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    fi

    case "$GATE_STATUS" in
        PASS)    exit 0 ;;
        FAIL)    exit 1 ;;
        SKIP)    exit 2 ;;
        WAITING) exit 3 ;;
        BLOCKED) exit 4 ;;
        *)       exit 5 ;;
    esac
}

main "$@"
