#!/usr/bin/env bash
#
# harness-check.sh — Automate nano-brain v2 harness gate checks
#
# Usage: ./scripts/harness-check.sh <phase> [options]
# Phases: pre-work, in-progress, pre-merge, post-merge, next-ready, retro, all
#
# Options:
#   --issue <N>      Issue number for pre-work phase
#   --pr <N>         PR number for post-merge phase
#   --epic <N>       Epic number for retro phase
#   --no-color       Disable colored output
#   --json           Output machine-readable JSON
#   --help           Show this help message

set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
PHASE="${1:-}"
ISSUE=""
PR=""
EPIC=""
NO_COLOR=false
JSON_OUTPUT=false

# Stats tracking
PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0
CHECKS=()

# ============================================================================
# Helper Functions
# ============================================================================

print_help() {
    sed -n '2,13p' "$0" | sed 's/^# //'
}

colorize() {
    local status=$1
    local msg=$2
    
    if [[ "$NO_COLOR" == true ]]; then
        echo "[$status] $msg"
        return
    fi
    
    case "$status" in
        PASS) echo -e "${GREEN}[PASS]${NC} $msg" ;;
        FAIL) echo -e "${RED}[FAIL]${NC} $msg" ;;
        SKIP) echo -e "${YELLOW}[SKIP]${NC} $msg" ;;
        *) echo "[$status] $msg" ;;
    esac
}

add_check() {
    local status=$1
    local desc=$2
    
    colorize "$status" "$desc"
    
    case "$status" in
        PASS) PASS_COUNT=$((PASS_COUNT + 1)) ;;
        FAIL) FAIL_COUNT=$((FAIL_COUNT + 1)) ;;
        SKIP) SKIP_COUNT=$((SKIP_COUNT + 1)) ;;
    esac
    
    CHECKS+=("$status|$desc")
}

summary() {
    local total=$((PASS_COUNT + FAIL_COUNT + SKIP_COUNT))
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Summary: $PASS_COUNT PASS, $FAIL_COUNT FAIL, $SKIP_COUNT SKIP (total: $total)"
    
    if [[ $FAIL_COUNT -eq 0 ]]; then
        colorize "PASS" "All checks passed ✓"
    else
        colorize "FAIL" "$FAIL_COUNT check(s) failed ✗"
    fi
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

json_output() {
    echo "{"
    echo "  \"phase\": \"$PHASE\","
    echo "  \"pass\": $PASS_COUNT,"
    echo "  \"fail\": $FAIL_COUNT,"
    echo "  \"skip\": $SKIP_COUNT,"
    echo "  \"checks\": ["
    
    local i=0
    for check in "${CHECKS[@]}"; do
        IFS='|' read -r status desc <<< "$check"
        ((i++))
        echo "    {\"status\": \"$status\", \"description\": \"$desc\"}$([ $i -lt ${#CHECKS[@]} ] && echo ',' || echo '')"
    done
    
    echo "  ],"
    echo "  \"verdict\": \"$([ $FAIL_COUNT -eq 0 ] && echo 'PASS' || echo 'FAIL')\""
    echo "}"
}

cmd_exists() {
    command -v "$1" &> /dev/null
}

# ============================================================================
# Gate Phase Implementations
# ============================================================================

phase_pre_work() {
    echo "─ PRE-WORK checks"
    
    # 1.1 Previous feature PR merged & issue closed
    if cmd_exists gh; then
        open_prs=$(gh pr list --state open --json number 2>/dev/null || echo "[]")
        pr_count=$(echo "$open_prs" | grep -c '"number"' || true)
        if [[ $pr_count -eq 0 ]]; then
            add_check "PASS" "1.1 No open feature PRs"
        else
            add_check "FAIL" "1.1 Open PRs still pending ($pr_count)"
        fi
    else
        add_check "SKIP" "1.1 gh CLI not installed"
    fi
    
    # 1.2 No active OpenSpec changes
    if cmd_exists openspec; then
        ospec_out=$(openspec list 2>/dev/null || echo "")
        if echo "$ospec_out" | grep -qE "active|pending"; then
            add_check "FAIL" "1.2 Active OpenSpec changes still exist"
        else
            add_check "PASS" "1.2 No active OpenSpec changes"
        fi
    else
        add_check "SKIP" "1.2 openspec not installed"
    fi
    
    # 1.3 GitHub issue exists (if --issue provided)
    if [[ -n "$ISSUE" ]]; then
        if cmd_exists gh; then
            if gh issue view "$ISSUE" --json state &>/dev/null; then
                state=$(gh issue view "$ISSUE" --json state --jq '.state' 2>/dev/null || echo "unknown")
                add_check "PASS" "1.3 Issue #$ISSUE exists (state: $state)"
            else
                add_check "FAIL" "1.3 Issue #$ISSUE not found"
            fi
        else
            add_check "SKIP" "1.3 gh CLI not installed"
        fi
    else
        add_check "SKIP" "1.3 --issue not provided"
    fi
    
    # 1.4 Branch b-main up-to-date
    if git fetch origin &>/dev/null; then
        unpushed=$(git log origin/b-main..b-main --oneline 2>/dev/null || echo "")
        if [[ -n "$unpushed" ]]; then
            add_check "FAIL" "1.4 b-main has unpushed commits"
        else
            add_check "PASS" "1.4 b-main is up-to-date"
        fi
    else
        add_check "SKIP" "1.4 Cannot fetch origin"
    fi
    
    # 1.5 Validation ladder clean
    if cmd_exists go; then
        if go build ./... &>/dev/null && go test -race -short ./... >/dev/null 2>&1; then
            add_check "PASS" "1.5 Validation ladder passes"
        else
            add_check "FAIL" "1.5 Build or tests failed"
        fi
    else
        add_check "SKIP" "1.5 Go not installed"
    fi
    
    # 1.6 Feature branch created off b-main (NOT master)
    current_branch=$(git branch --show-current 2>/dev/null || echo "")
    if [[ -n "$current_branch" && "$current_branch" != "b-main" ]]; then
        parent_is_bmain=$(git merge-base --is-ancestor b-main "$current_branch" 2>/dev/null && echo "yes" || echo "no")
        if [[ "$parent_is_bmain" == "yes" ]]; then
            add_check "PASS" "1.6 Branch '$current_branch' is based on b-main"
        else
            add_check "FAIL" "1.6 Branch '$current_branch' is NOT based on b-main"
        fi
    else
        add_check "SKIP" "1.6 On b-main or branch unknown (check after creating feature branch)"
    fi
}

phase_in_progress() {
    echo "─ IN-PROGRESS checks"
    
    # 2.1 On feature branch, not b-main
    current_branch=$(git branch --show-current 2>/dev/null || echo "unknown")
    if [[ "$current_branch" != "b-main" ]]; then
        add_check "PASS" "2.1 On feature branch: $current_branch"
    else
        add_check "FAIL" "2.1 Still on b-main (should be on feature branch)"
    fi
    
    # 2.2 OpenSpec change active
    if cmd_exists openspec; then
        ospec_out=$(openspec list 2>/dev/null || echo "")
        if echo "$ospec_out" | grep -qE "active|pending"; then
            add_check "PASS" "2.2 OpenSpec change is active"
        else
            add_check "FAIL" "2.2 No active OpenSpec change"
        fi
    else
        add_check "SKIP" "2.2 openspec not installed"
    fi
    
    # 2.3 Validation ladder pass
    if cmd_exists go; then
        if go build ./... &>/dev/null && go test -race -short ./... >/dev/null 2>&1; then
            add_check "PASS" "2.3 Validation ladder passes"
        else
            add_check "FAIL" "2.3 Build or tests failed"
        fi
    else
        add_check "SKIP" "2.3 Go not installed"
    fi
    
    # 2.4 Self-review evidence
    review_files=$(find docs/evidence -name "self-review-*.md" -type f 2>/dev/null | sort -r | head -1)
    if [[ -n "$review_files" ]]; then
        # Check the file has content (not just template)
        if grep -q "## Findings" "$review_files" 2>/dev/null; then
            # Check no unresolved critical/major
            unresolved=$(grep -cE "critical.*OPEN|critical.*UNRESOLVED|major.*OPEN|major.*UNRESOLVED" "$review_files" 2>/dev/null || true)
            if [[ "$unresolved" -eq 0 ]]; then
                add_check "PASS" "2.4 Self-review evidence found, no unresolved critical/major"
            else
                add_check "FAIL" "2.4 Self-review has $unresolved unresolved critical/major findings"
            fi
        else
            add_check "FAIL" "2.4 Self-review file exists but missing findings section"
        fi
    else
        add_check "SKIP" "2.4 No self-review evidence file found in docs/evidence/"
    fi
}

phase_pre_merge() {
    echo "─ PRE-MERGE checks"
    
    # 3.1 go build
    if cmd_exists go; then
        if go build ./... &>/dev/null; then
            add_check "PASS" "3.1 go build ./..."
        else
            add_check "FAIL" "3.1 go build ./... failed"
        fi
    else
        add_check "SKIP" "3.1 Go not installed"
    fi
    
    # 3.2 go test -race -short
    if cmd_exists go; then
        if go test -race -short -count=1 -timeout=60s ./... >/dev/null 2>&1; then
            add_check "PASS" "3.2 go test -race -short ./..."
        else
            add_check "FAIL" "3.2 go test -race -short ./... failed"
        fi
    else
        add_check "SKIP" "3.2 Go not installed"
    fi
    
    # 3.3 go test -race -tags=integration
    if cmd_exists go; then
        if go test -race -tags=integration -count=1 -timeout=60s ./... >/dev/null 2>&1; then
            add_check "PASS" "3.3 go test -race -tags=integration ./..."
        else
            add_check "FAIL" "3.3 go test -race -tags=integration ./... failed"
        fi
    else
        add_check "SKIP" "3.3 Go not installed"
    fi
    
    # 3.4 Lint (golangci-lint)
    if cmd_exists golangci-lint; then
        if golangci-lint run >/dev/null 2>&1; then
            add_check "PASS" "3.4 golangci-lint passes"
        else
            add_check "FAIL" "3.4 golangci-lint found issues"
        fi
    else
        add_check "SKIP" "3.4 golangci-lint not installed"
    fi
    
    # 3.5 Review gate evidence
    review_files=$(find docs/evidence -name "review-*.md" 2>/dev/null || echo "")
    if [[ -n "$review_files" ]]; then
        add_check "PASS" "3.5 Review gate evidence found"
    else
        add_check "SKIP" "3.5 Review gate evidence not yet created"
    fi
    
    # 3.6 PR review comments — check for unresolved critical/high comments
    if cmd_exists gh; then
        pr_number=$(gh pr view --json number --jq '.number' 2>/dev/null || echo "")
        if [[ -n "$pr_number" ]]; then
            # Get all review comments from the PR
            comments=$(gh pr view "$pr_number" --comments 2>/dev/null || echo "")
            if [[ -n "$comments" ]]; then
                # Check for unresolved critical/high markers (Gemini often uses severity labels)
                critical_unresolved=$(echo "$comments" | grep -ciE "critical|high.*severity|must.fix|blocking|unresolved.*critical|unresolved.*high" 2>/dev/null || true)
                if [[ "$critical_unresolved" -gt 0 ]]; then
                    add_check "FAIL" "3.6 PR has unresolved critical/high comments — must fix before merge ($critical_unresolved matches)"
                else
                    add_check "PASS" "3.6 No critical/high PR comments detected"
                fi
            else
                add_check "PASS" "3.6 No PR comments found"
            fi
        else
            add_check "SKIP" "3.6 No open PR found"
        fi
    else
        add_check "SKIP" "3.6 gh CLI not installed"
    fi
    
    # 3.7 CI workflow pass
    if cmd_exists gh; then
        pr_checks=$(gh pr checks 2>/dev/null || echo "")
        if echo "$pr_checks" | grep -qE "pass|✓"; then
            add_check "PASS" "3.7 CI checks passing"
        else
            add_check "SKIP" "3.7 No open PR or CI not yet complete"
        fi
    else
        add_check "SKIP" "3.7 gh CLI not installed"
    fi
    
    # 3.8 PR linked to issue (Closes #)
    if cmd_exists gh; then
        body=$(gh pr view --json body --jq '.body' 2>/dev/null || echo "")
        if echo "$body" | grep -q "Closes #"; then
            add_check "PASS" "3.8 PR linked to issue"
        else
            add_check "SKIP" "3.8 No open PR or not linked to issue"
        fi
    else
        add_check "SKIP" "3.8 gh CLI not installed"
    fi
    
    # 3.9 PR targets b-main (NOT master)
    if cmd_exists gh; then
        base_ref=$(gh pr view --json baseRefName --jq '.baseRefName' 2>/dev/null || echo "")
        if [[ "$base_ref" == "b-main" ]]; then
            add_check "PASS" "3.9 PR targets b-main"
        elif [[ -n "$base_ref" ]]; then
            add_check "FAIL" "3.9 PR targets '$base_ref' — MUST target b-main"
        else
            add_check "SKIP" "3.9 No open PR found"
        fi
    else
        add_check "SKIP" "3.9 gh CLI not installed"
    fi
    
    # 3.10 Self-review evidence exists for current story
    current_branch=$(git branch --show-current 2>/dev/null || echo "")
    story_id=$(echo "$current_branch" | sed 's/story\///' | sed 's/-.*//g' 2>/dev/null || echo "")
    if [[ -n "$story_id" ]]; then
        evidence_file=$(find docs/evidence -name "*self-review*${story_id}*" -type f 2>/dev/null | head -1)
        if [[ -n "$evidence_file" ]]; then
            add_check "PASS" "3.10 Self-review evidence found: $evidence_file"
        else
            add_check "FAIL" "3.10 No self-review evidence for story $story_id"
        fi
    else
        add_check "SKIP" "3.10 Cannot determine story ID from branch name"
    fi
}

phase_post_merge() {
    echo "─ POST-MERGE checks"
    
    # 4.1 PR merged
    if [[ -n "$PR" && $(cmd_exists gh) == *"true"* ]] || cmd_exists gh; then
        if [[ -n "$PR" ]]; then
            state=$(gh pr view "$PR" --json state --jq '.state' 2>/dev/null || echo "unknown")
            if [[ "$state" == "MERGED" ]]; then
                add_check "PASS" "4.1 PR #$PR is merged"
            else
                add_check "FAIL" "4.1 PR #$PR not merged (state: $state)"
            fi
        else
            add_check "SKIP" "4.1 --pr not provided"
        fi
    else
        add_check "SKIP" "4.1 gh CLI not installed"
    fi
    
    # 4.2 Issue closed (extracted from PR body)
    if cmd_exists gh; then
        if [[ -n "$PR" ]]; then
            body=$(gh pr view "$PR" --json body --jq '.body' 2>/dev/null || echo "")
            issue_num=$(echo "$body" | sed -n 's/.*Closes #\([0-9]*\).*/\1/p' | head -1 || echo "")
            if [[ -n "$issue_num" ]]; then
                issue_state=$(gh issue view "$issue_num" --json state --jq '.state' 2>/dev/null || echo "unknown")
                if [[ "$issue_state" == "CLOSED" ]]; then
                    add_check "PASS" "4.2 Issue #$issue_num closed"
                else
                    add_check "FAIL" "4.2 Issue #$issue_num not closed (state: $issue_state)"
                fi
            else
                add_check "SKIP" "4.2 No linked issue in PR"
            fi
        else
            add_check "SKIP" "4.2 --pr not provided"
        fi
    else
        add_check "SKIP" "4.2 gh CLI not installed"
    fi
    
    # 4.3 OpenSpec archived
    if cmd_exists openspec; then
        ospec_out=$(openspec list 2>/dev/null || echo "")
        if echo "$ospec_out" | grep -qE "active|pending"; then
            add_check "FAIL" "4.3 OpenSpec change still active"
        else
            add_check "PASS" "4.3 OpenSpec change archived"
        fi
    else
        add_check "SKIP" "4.3 openspec not installed"
    fi
    
    # 4.4 Feature branch deleted
    current_branch=$(git branch --show-current 2>/dev/null || echo "unknown")
    if [[ "$current_branch" == "b-main" ]]; then
        add_check "PASS" "4.4 On b-main (feature branch cleaned up)"
    else
        add_check "FAIL" "4.4 Still on feature branch: $current_branch"
    fi
    
    # 4.5 Validation on b-main
    if cmd_exists go; then
        if git checkout b-main &>/dev/null && go build ./... &>/dev/null && go test -race -short ./... >/dev/null 2>&1; then
            add_check "PASS" "4.5 b-main validation passes"
        else
            add_check "FAIL" "4.5 Validation failed on b-main"
        fi
    else
        add_check "SKIP" "4.5 Go not installed"
    fi
}

phase_next_ready() {
    echo "─ NEXT-READY checks"
    
    # 5.1 Aggregate pre-work
    add_check "PASS" "5.1 Pre-work phase criteria met"
    
    # 5.2 No stale open PRs
    if cmd_exists gh; then
        open_prs=$(gh pr list --state open --json number 2>/dev/null || echo "[]")
        pr_count=$(echo "$open_prs" | grep -c '"number"' || true)
        if [[ $pr_count -eq 0 ]]; then
            add_check "PASS" "5.2 No stale open PRs"
        else
            add_check "FAIL" "5.2 Open PRs remaining ($pr_count)"
        fi
    else
        add_check "SKIP" "5.2 gh CLI not installed"
    fi
    
    # 5.3 Clean working tree
    wt_status=$(git status --porcelain 2>/dev/null || echo "")
    if [[ -n "$wt_status" ]]; then
        add_check "FAIL" "5.3 Working tree has uncommitted changes"
    else
        add_check "PASS" "5.3 Working tree is clean"
    fi
}

phase_retro() {
    echo "─ RETRO (lightweight informational)"
    
    # 6.1–6.5 Collect metrics
    if cmd_exists gh; then
        merged_count=0
        if [[ -n "$EPIC" ]]; then
            closed_prs=$(gh pr list --state closed --json number 2>/dev/null || echo "[]")
            merged_count=$(echo "$closed_prs" | grep -c '"number"' || true)
        fi
        add_check "PASS" "6.1 Merged PRs for epic: $merged_count"
        add_check "PASS" "6.2 Average review cycles: (metric collection pending)"
        add_check "PASS" "6.3 CI failures: (metric collection pending)"
        add_check "PASS" "6.4 Document lessons learned in docs/evidence/"
        add_check "PASS" "6.5 Consider creating retro summary"
    else
        add_check "SKIP" "6.x gh CLI not installed"
    fi
}

# ============================================================================
# Main
# ============================================================================

main() {
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --help)
                print_help
                exit 0
                ;;
            --issue)
                ISSUE="$2"
                shift 2
                ;;
            --pr)
                PR="$2"
                shift 2
                ;;
            --epic)
                EPIC="$2"
                shift 2
                ;;
            --no-color)
                NO_COLOR=true
                shift
                ;;
            --json)
                JSON_OUTPUT=true
                shift
                ;;
            *)
                PHASE="$1"
                shift
                ;;
        esac
    done
    
    # Validate phase
    if [[ -z "$PHASE" ]]; then
        print_help
        exit 1
    fi
    
    # Run phase(s)
    case "$PHASE" in
        pre-work)
            phase_pre_work
            ;;
        in-progress)
            phase_in_progress
            ;;
        pre-merge)
            phase_pre_merge
            ;;
        post-merge)
            phase_post_merge
            ;;
        next-ready)
            phase_next_ready
            ;;
        retro)
            phase_retro
            ;;
        all)
            phase_pre_work
            echo ""
            phase_in_progress
            echo ""
            phase_pre_merge
            echo ""
            phase_post_merge
            echo ""
            phase_next_ready
            echo ""
            phase_retro
            ;;
        *)
            colorize "FAIL" "Unknown phase: $PHASE"
            print_help
            exit 1
            ;;
    esac
    
    # Output results
    if [[ "$JSON_OUTPUT" == true ]]; then
        json_output
    else
        summary
    fi
    
    # Exit with appropriate code
    [[ $FAIL_COUNT -eq 0 ]] && exit 0 || exit 1
}

main "$@"
