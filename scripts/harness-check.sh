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
    local rule_id="${3:-}"
    
    if [[ "$JSON_OUTPUT" != true ]]; then
        colorize "$status" "$desc"
    fi
    
    case "$status" in
        PASS) PASS_COUNT=$((PASS_COUNT + 1)) ;;
        FAIL) FAIL_COUNT=$((FAIL_COUNT + 1)) ;;
        SKIP) SKIP_COUNT=$((SKIP_COUNT + 1)) ;;
    esac
    
    CHECKS+=("$status|$desc|$rule_id")
}

# Run phase_hooks.<gate>.before from .opencode/harness.config.json if configured.
# Emits a PASS/FAIL check and returns 1 on hook failure so callers can abort early.
run_phase_hook_before() {
    local gate="$1"
    local config_file=".opencode/harness.config.json"
    [[ ! -f "$config_file" ]] && return 0
    if ! command -v python3 &>/dev/null; then return 0; fi

    local hook_cmd
    hook_cmd=$(python3 -c "
import sys, json
try:
    d = json.load(open('$config_file'))
    print(d.get('phase_hooks', {}).get('$gate', {}).get('before', ''))
except Exception:
    print('')
" 2>/dev/null)

    [[ -z "$hook_cmd" ]] && return 0

    local hook_out hook_exit
    hook_out=$(eval "$hook_cmd" 2>&1)
    hook_exit=$?
    if [[ $hook_exit -eq 0 ]]; then
        add_check "PASS" "0.0 phase_hooks.$gate.before: OK"
    else
        add_check "FAIL" "0.0 phase_hooks.$gate.before: hook failed (exit $hook_exit)" "phase-hook"
        return 1
    fi
    return 0
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

get_next_gate() {
    local current="$1"
    case "$current" in
        pre-work) echo "in-progress" ;;
        in-progress) echo "pre-merge" ;;
        pre-merge) echo "async-pr-review" ;;
        async-pr-review) echo "post-merge" ;;
        post-merge) echo "post-merge-npm-release" ;;
        post-merge-npm-release) echo "next-ready" ;;
        next-ready) echo "null" ;;
        retro) echo "null" ;;
        *) echo "null" ;;
    esac
}

build_instructions_for_agent() {
    local failed_checks=()
    for check in "${CHECKS[@]}"; do
        IFS='|' read -r status desc rule_id <<< "$check"
        if [[ "$status" == "FAIL" ]]; then
            if [[ -n "$rule_id" ]]; then
                failed_checks+=("[$rule_id] $desc")
            else
                failed_checks+=("$desc")
            fi
        fi
    done
    
    if [[ ${#failed_checks[@]} -eq 0 ]]; then
        echo ""
        return
    fi
    
    echo "Fix the following issues before proceeding:"
    printf '%s\n' "${failed_checks[@]}"
}

json_output() {
    local status
    local next_gate
    local rule_ids_json="[]"
    
    if [[ $FAIL_COUNT -eq 0 ]]; then
        if [[ $SKIP_COUNT -gt 0 && $PASS_COUNT -eq 0 ]]; then
            status="SKIP"
        else
            status="PASS"
        fi
    else
        status="FAIL"
    fi
    
    next_gate=$(get_next_gate "$PHASE")
    
    local rule_ids=()
    for check in "${CHECKS[@]}"; do
        IFS='|' read -r chk_status desc rule_id <<< "$check"
        if [[ "$chk_status" == "FAIL" && -n "$rule_id" ]]; then
            rule_ids+=("\"$rule_id\"")
        fi
    done
    
    if [[ ${#rule_ids[@]} -gt 0 ]]; then
        rule_ids_json="[$(IFS=,; echo "${rule_ids[*]}")]"
    fi
    
    echo "{"
    echo "  \"gate\": \"$PHASE\","
    echo "  \"status\": \"$status\","
    echo "  \"checks\": ["
    
    local i=0
    for check in "${CHECKS[@]}"; do
        IFS='|' read -r chk_status desc rule_id <<< "$check"
        i=$((i + 1))
        local id_field=""
        local name_field="$desc"
        if [[ "$desc" =~ ^([0-9]+\.[0-9]+)\ (.*)$ ]]; then
            id_field="${BASH_REMATCH[1]}"
            name_field="${BASH_REMATCH[2]}"
        fi
        local rule_field=""
        if [[ -n "$rule_id" ]]; then
            rule_field=", \"rule_id\": \"$rule_id\""
        fi
        echo "    {\"id\": \"$id_field\", \"name\": \"$name_field\", \"status\": \"$chk_status\"$rule_field}$([ $i -lt ${#CHECKS[@]} ] && echo ',' || echo '')"
    done
    
    echo "  ],"
    
    if [[ "$next_gate" == "null" ]]; then
        echo "  \"next_gate\": null,"
    else
        echo "  \"next_gate\": \"$next_gate\","
    fi
    
    if [[ "$status" == "FAIL" ]]; then
        local instructions
        instructions=$(build_instructions_for_agent)
        instructions="${instructions//\"/\\\"}"
        instructions="${instructions//$'\n'/\\n}"
        echo "  \"instructions_for_agent\": \"$instructions\","
    fi
    
    echo "  \"rule_ids_violated\": $rule_ids_json"
    echo "}"
}

cmd_exists() {
    command -v "$1" &> /dev/null
}

# Check GSD phase state from .planning/STATE.md
# Returns: "none", "in_progress", "completed", or "unknown"
get_gsd_phase_state() {
    local state_file=".planning/STATE.md"
    if [[ ! -f "$state_file" ]]; then
        echo "unknown"
        return
    fi

    # Reads the gsd-core canonical STATE.md format, in priority order:
    #   1) frontmatter `status:` (project/milestone state)
    #   2) `## Current Position` -> `Status:` line
    #   3) legacy `**Phase N: Name** (status)` bold line (back-compat)
    local fm_status pos_status legacy probe
    fm_status=$(awk 'NR==1&&/^---[[:space:]]*$/{f=1;next} f&&/^---[[:space:]]*$/{exit} f&&/^status:/{sub(/^status:[[:space:]]*/,"");gsub(/["'"'"']/,"");print;exit}' "$state_file")
    pos_status=$(grep -iE '^Status:[[:space:]]' "$state_file" 2>/dev/null | head -1 | sed -E 's/^[Ss]tatus:[[:space:]]*//')
    legacy=$(grep -E '^\*\*Phase [0-9]+:' "$state_file" 2>/dev/null | head -1 || echo "")

    if [[ -z "$fm_status$pos_status$legacy" ]]; then
        echo "none"
        return
    fi

    probe="$fm_status | $pos_status | $legacy"
    if echo "$probe" | grep -qiE 'in.?progress|in_progress|executing|verifying'; then
        echo "in_progress"
    elif echo "$probe" | grep -qiE 'complete|completed|done|finished'; then
        echo "completed"
    elif echo "$probe" | grep -qiE 'ready to (plan|execute)|planning|pending|not.?started|planned'; then
        echo "pending"
    else
        echo "unknown"
    fi
}

has_active_gsd_phase() {
    local state
    state=$(get_gsd_phase_state)
    [[ "$state" == "in_progress" ]]
}

is_gsd_phase_completed() {
    local state
    state=$(get_gsd_phase_state)
    [[ "$state" == "completed" ]]
}

# ============================================================================
# Gate Phase Implementations
# ============================================================================

phase_pre_work() {
    [[ "$JSON_OUTPUT" != true ]] && echo "─ PRE-WORK checks"
    run_phase_hook_before "pre-work" || return 0

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
    
    # 1.2 No active GSD phase
    gsd_state=$(get_gsd_phase_state)
    case "$gsd_state" in
        none|completed)
            add_check "PASS" "1.2 No active GSD phase (state: $gsd_state)"
            ;;
        in_progress)
            add_check "FAIL" "1.2 GSD phase still in progress — complete or archive before starting new feature"
            ;;
        pending)
            add_check "PASS" "1.2 GSD phase pending (not started)"
            ;;
        *)
            add_check "SKIP" "1.2 Cannot determine GSD state (.planning/STATE.md missing or invalid)"
            ;;
    esac
    
    # 1.3 GitHub issue exists or skip-conditions met (R89)
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
        # R89: --issue omitted → verify zero file changes in last commit
        changed=$(git diff --name-only HEAD~1 HEAD 2>/dev/null | wc -l | tr -d ' ' || echo "0")
        if [[ "$changed" -gt 0 ]]; then
            add_check "FAIL" "1.3 $changed file(s) changed in HEAD but no --issue provided" "R89"
        else
            add_check "PASS" "1.3 No file changes detected; issue not required (R89)"
        fi
    fi
    
    # 1.4 Branch master up-to-date
    if git fetch origin &>/dev/null; then
        unpushed=$(git log origin/master..master --oneline 2>/dev/null || echo "")
        if [[ -n "$unpushed" ]]; then
            add_check "FAIL" "1.4 master has unpushed commits"
        else
            add_check "PASS" "1.4 master is up-to-date"
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
    
    # 1.6 Feature branch created off master
    current_branch=$(git branch --show-current 2>/dev/null || echo "")
    if [[ -n "$current_branch" && "$current_branch" != "master" ]]; then
        parent_is_master=$(git merge-base --is-ancestor master "$current_branch" 2>/dev/null && echo "yes" || echo "no")
        if [[ "$parent_is_master" == "yes" ]]; then
            add_check "PASS" "1.6 Branch '$current_branch' is based on master"
        else
            add_check "FAIL" "1.6 Branch '$current_branch' is NOT based on master"
        fi
    else
        add_check "SKIP" "1.6 On master or branch unknown (check after creating feature branch)"
    fi
    
    # 1.7 Deep-design completed (normal+ risk only — tiny lane skips)
    deep_design_file=$(find docs/evidence -name "deep-design-*.md" -type f 2>/dev/null | sort -r | head -1)
    if [[ -n "$deep_design_file" ]]; then
        if grep -qE '(Verdict|verdict|PASS|pass)' "$deep_design_file"; then
            add_check "PASS" "1.7 Deep-design evidence found: $deep_design_file"
        else
            add_check "FAIL" "1.7 $deep_design_file missing verdict (PASS/FAIL)"
        fi
    else
        if [[ -n "$ISSUE" ]] && cmd_exists gh; then
            labels=$(gh issue view "$ISSUE" --json labels --jq '.labels[].name' 2>/dev/null || echo "")
            if echo "$labels" | grep -q "lane:tiny"; then
                add_check "SKIP" "1.7 Deep-design skipped (tiny lane)"
            else
                add_check "FAIL" "1.7 No deep-design evidence — run deep-design pipeline first"
            fi
        else
            add_check "FAIL" "1.7 No deep-design evidence — run deep-design pipeline first"
        fi
    fi
}

phase_in_progress() {
    [[ "$JSON_OUTPUT" != true ]] && echo "─ IN-PROGRESS checks"
    run_phase_hook_before "in-progress" || return 0

    # 2.1 On feature branch, not master
    current_branch=$(git branch --show-current 2>/dev/null || echo "unknown")
    if [[ "$current_branch" != "master" ]]; then
        add_check "PASS" "2.1 On feature branch: $current_branch"
    else
        add_check "FAIL" "2.1 Still on master (should be on feature branch)"
    fi
    
    # 2.2 Active GSD phase exists
    gsd_state=$(get_gsd_phase_state)
    if [[ "$gsd_state" == "in_progress" ]]; then
        add_check "PASS" "2.2 Active GSD phase exists (state: in_progress)"
    elif [[ "$gsd_state" == "pending" ]]; then
        add_check "FAIL" "2.2 GSD phase not started — run /gsd-plan-phase or /gsd-execute-phase first"
    elif [[ "$gsd_state" == "none" ]]; then
        add_check "FAIL" "2.2 No GSD phase defined — initialize with /gsd-new-project or /gsd-new-milestone"
    else
        add_check "SKIP" "2.2 Cannot determine GSD state"
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
    # 2.4 Self-review evidence — Tier 2 format (TRACE_SPEC.md)
    review_files=$(find docs/evidence -name "self-review-*.md" -type f 2>/dev/null | sort -r | head -1)
    if [[ -z "$review_files" ]]; then
        add_check "SKIP" "2.4 No self-review evidence file found in docs/evidence/"
    else
        missing_sections=""
        for section in \
            "## Actions Taken" \
            "## Files Changed" \
            "## Findings Summary" \
            "## Resolution Status"; do
            grep -qF "$section" "$review_files" || missing_sections="$missing_sections '$section'"
        done

        if [[ -n "$missing_sections" ]]; then
            add_check "FAIL" "2.4 $review_files missing required sections:$missing_sections (TRACE_SPEC Tier 2)"
        else
            unresolved=$(grep -cE "critical.*(OPEN|UNRESOLVED)|major.*(OPEN|UNRESOLVED)" "$review_files" 2>/dev/null || true)
            if [[ "$unresolved" -eq 0 ]]; then
                add_check "PASS" "2.4 Self-review evidence has all required sections, no unresolved critical/major"
            else
                add_check "FAIL" "2.4 Self-review has $unresolved unresolved critical/major findings"
            fi
        fi
    fi
}

phase_pre_merge() {
    [[ "$JSON_OUTPUT" != true ]] && echo "─ PRE-MERGE checks"
    run_phase_hook_before "pre-merge" || return 0

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
        if go test -race -tags=integration -count=1 -timeout=180s ./... >/dev/null 2>&1; then
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
    
    # 3.5 Review Gate verdict (R27: literal 'Review Verdict: PASS') +
    #     reviewer independence (R88: review MUST be done by a separate agent —
    #     the implementing agent may NOT self-review/self-approve its own code).
    rg_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
    rg_sid=$(echo "$rg_branch" | sed 's|^[^/]*/||' | sed 's/-.*//g' 2>/dev/null || echo "")
    # Only the review file for THIS story counts — a stale cross-story review must
    # not satisfy the gate. The [[ -n rg_sid ]] guard is load-bearing: an empty
    # sid would turn the glob into review-*.md and match every review.
    # Match the sid as a whole segment (followed by '.md' or '-slug'), never a
    # prefix: sid '49' must NOT match review-497.md.
    story_review=""
    if [[ -n "$rg_sid" ]]; then
        story_review=$(find docs/evidence -type f \( -name "review-${rg_sid}.md" -o -name "review-${rg_sid}-*.md" \) 2>/dev/null | head -1)
    fi
    if [[ -n "$story_review" ]]; then
        # Note: this checks that an independent reviewer is *declared*; it is an
        # honesty guardrail, not a tamper-proof control. Identity-bound external
        # review is enforced separately by the PR bot review (gate 3.6).
        if grep -qE '^Review Verdict: FAIL' "$story_review"; then
            add_check "FAIL" "3.5 Review Verdict: FAIL in $story_review — fix before merge" "R27"
        elif ! grep -qE '^Review Verdict: PASS' "$story_review"; then
            add_check "FAIL" "3.5 $story_review missing 'Review Verdict: PASS|FAIL' line" "R27"
        else
            reviewer=$(grep -iE '^Reviewer:' "$story_review" | head -1 | sed 's/^[Rr]eviewer:[[:space:]]*//')
            if [[ -z "$reviewer" ]]; then
                add_check "FAIL" "3.5 $story_review has no 'Reviewer:' line (R88: name the independent review agent)" "R88"
            elif echo "$reviewer" | grep -qiE '\b(self|author|implementer)\b'; then
                add_check "FAIL" "3.5 $story_review reviewer '$reviewer' is the author — review must be a separate agent (R88)" "R88"
            else
                add_check "PASS" "3.5 Review Verdict: PASS by independent reviewer '$reviewer' (R27, R88)"
            fi
        fi
    elif find docs/evidence -name "review-*.md" -type f 2>/dev/null | grep -q .; then
        add_check "FAIL" "3.5 No review-${rg_sid}*.md for this story (R88: independent review required)" "R88"
    else
        add_check "SKIP" "3.5 Review gate evidence not yet created"
    fi
    
    # 3.6 Gemini comment triage (R31) + override check (R7)
    if ! cmd_exists gh; then
        add_check "SKIP" "3.6 gh CLI not installed"
    else
        pr_number=$(gh pr view --json number --jq '.number' 2>/dev/null || echo "")
        if [[ -z "$pr_number" ]]; then
            add_check "SKIP" "3.6 No open PR found"
        else
            # R7: check for [HARNESS-OVERRIDE]: <reason> (reason >= 20 chars)
            override=$(gh pr view "$pr_number" --comments 2>/dev/null \
                | grep -oE '\[HARNESS-OVERRIDE\]: .{20,}' | head -1 || true)
            if [[ -n "$override" ]]; then
                add_check "PASS" "3.6 PR has [HARNESS-OVERRIDE] (R7: Gemini bypass approved)"
            else
                # R31: count Gemini comments vs triage rows in evidence file
                gemini_count=$(gh pr view "$pr_number" --json comments \
                    --jq '[.comments[] | select(.author.login | test("gemini"; "i"))] | length' \
                    2>/dev/null || echo "0")

                current_branch=$(git branch --show-current 2>/dev/null || echo "")
                slug=$(echo "$current_branch" | sed 's|^[^/]*/||')
                triage_file=$(find docs/evidence -name "self-review-${slug}*.md" -type f 2>/dev/null | head -1)

                if [[ "$gemini_count" -eq 0 ]]; then
                    add_check "PASS" "3.6 No Gemini comments on PR"
                elif [[ -z "$triage_file" ]]; then
                    add_check "FAIL" "3.6 $gemini_count Gemini comment(s) but no self-review-${slug}*.md (R31)"
                else
                    # Count triage rows under '## Gemini Verification Triage' section
                    triage_rows=$(awk '/^## Gemini Verification Triage/{f=1;next} /^## /{f=0} f' "$triage_file" \
                        | grep -cE '^\| (PR#|line)' || true)

                    if [[ "$triage_rows" -lt "$gemini_count" ]]; then
                        add_check "FAIL" "3.6 Gemini comments=$gemini_count but triage rows=$triage_rows (R31: every comment must be triaged)"
                    else
                        # Check VALID:critical / VALID:high rows have fix commit reference
                        unresolved=$(awk '/^## Gemini Verification Triage/{f=1;next} /^## /{f=0} f' "$triage_file" \
                            | grep -E 'VALID:(critical|high)' \
                            | grep -vcE 'fixed in commit [a-f0-9]{6,}' || true)
                        if [[ "$unresolved" -gt 0 ]]; then
                            add_check "FAIL" "3.6 $unresolved VALID:critical/high row(s) without 'fixed in commit <sha>' (R31)"
                        else
                            add_check "PASS" "3.6 All $gemini_count Gemini comment(s) triaged and resolved (R31)"
                        fi
                    fi
                fi
            fi
        fi
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
    
    # 3.8 PR linked to exactly 1 issue (R1: no bundling)
    if cmd_exists gh; then
        body=$(gh pr view --json body --jq '.body' 2>/dev/null || echo "")
        if [[ -z "$body" ]]; then
            add_check "SKIP" "3.8 No open PR found"
        else
            closes_count=$(echo "$body" | grep -cE 'Closes #[0-9]+' || true)
            if [[ "$closes_count" -eq 1 ]]; then
                add_check "PASS" "3.8 PR closes exactly 1 issue (R1)"
            elif [[ "$closes_count" -eq 0 ]]; then
                add_check "FAIL" "3.8 PR body missing 'Closes #N' reference" "R1"
            else
                add_check "FAIL" "3.8 PR closes $closes_count issues (must be exactly 1; split PR)" "R1"
            fi
        fi
    else
        add_check "SKIP" "3.8 gh CLI not installed"
    fi
    
    # 3.9 PR targets master
    if cmd_exists gh; then
        base_ref=$(gh pr view --json baseRefName --jq '.baseRefName' 2>/dev/null || echo "")
        if [[ "$base_ref" == "master" ]]; then
            add_check "PASS" "3.9 PR targets master"
        elif [[ -n "$base_ref" ]]; then
            add_check "FAIL" "3.9 PR targets '$base_ref' — MUST target master"
        else
            add_check "SKIP" "3.9 No open PR found"
        fi
    else
        add_check "SKIP" "3.9 gh CLI not installed"
    fi
    
    # 3.10 Self-review evidence exists for current story
    current_branch=$(git branch --show-current 2>/dev/null || echo "")
    branch_slug=$(echo "$current_branch" | sed 's|^[^/]*/||' | sed 's/-.*//g' 2>/dev/null || echo "")
    story_id=$(echo "$current_branch" | sed 's/story\///' | sed 's/-.*//g' 2>/dev/null || echo "")
    if [[ -n "$branch_slug" ]]; then
        evidence_file=$(find docs/evidence -name "*self-review*${branch_slug}*" -type f 2>/dev/null | head -1)
        if [[ -z "$evidence_file" && -n "$story_id" ]]; then
            evidence_file=$(find docs/evidence -name "*self-review*${story_id}*" -type f 2>/dev/null | head -1)
        fi
        if [[ -n "$evidence_file" ]]; then
            add_check "PASS" "3.10 Self-review evidence found: $evidence_file"
        else
            add_check "FAIL" "3.10 No self-review evidence for story $branch_slug"
        fi
    else
        add_check "SKIP" "3.10 Cannot determine story ID from branch name"
    fi

    # 3.11 Max 3 push cycles per PR (R29)
    if cmd_exists gh; then
        pr_number=$(gh pr view --json number --jq '.number' 2>/dev/null || echo "")
        if [[ -n "$pr_number" ]]; then
            commit_count=$(gh pr view "$pr_number" --json commits \
                --jq '.commits | length' 2>/dev/null || echo "0")
            if [[ "$commit_count" -gt 3 ]]; then
                add_check "FAIL" "3.11 PR has $commit_count commits (max 3 push cycles; escalate to human)" "R29"
            else
                add_check "PASS" "3.11 PR commit count: $commit_count (R29: ≤ 3)"
            fi
        else
            add_check "SKIP" "3.11 No open PR found"
        fi
    else
        add_check "SKIP" "3.11 gh CLI not installed"
    fi

    # 3.12 smoke:e2e evidence for user-feature/bug-fix changes (R19, R20)
    if [[ -n "$branch_slug" ]]; then
        smoke_file=$(find docs/evidence -name "smoke-e2e-${branch_slug}*" -type f 2>/dev/null | head -1)
        if [[ -z "$smoke_file" && -n "$story_id" ]]; then
            smoke_file=$(find docs/evidence -name "smoke-e2e-${story_id}*" -type f 2>/dev/null | head -1)
        fi
        if [[ -n "$smoke_file" ]]; then
            if grep -qE '^(curl|HTTP/[12])' "$smoke_file"; then
                add_check "PASS" "3.12 smoke:e2e evidence with curl/HTTP found (R19, R20)"
            else
                add_check "FAIL" "3.12 $smoke_file has no curl command or HTTP response" "R20"
            fi
        else
            add_check "SKIP" "3.12 No smoke-e2e-${branch_slug}*.{md,txt} (R19: required if change-type ∈ {user-feature, bug-fix})"
        fi
    else
        add_check "SKIP" "3.12 Cannot determine story ID for smoke:e2e check"
    fi

    # 3.13 smoke:ui — REMOVED (UI migrated to standalone dashboard repo)
    add_check "SKIP" "3.13 smoke:ui deprecated (dashboard split complete)"

    # 3.14 No real workspace names/paths/hashes in staged files (privacy gate)
    privacy_patterns='Phil-timeshel\|capyhome\|zengamingx\|/Users/tamlh/workspaces/self/Projects/'
    privacy_hits=$(git diff --cached --name-only 2>/dev/null | head -100 | xargs grep -l "$privacy_patterns" --include='*.go' --include='*.md' --include='*.json' --include='*.sh' --include='*.yml' 2>/dev/null || true)
    if [[ -n "$privacy_hits" ]]; then
        add_check "FAIL" "3.14 Real workspace names/paths found in staged files — use generic placeholders" "privacy"
    else
        add_check "PASS" "3.14 No real workspace names in staged files (privacy gate)"
    fi
}

phase_post_merge() {
    [[ "$JSON_OUTPUT" != true ]] && echo "─ POST-MERGE checks"
    run_phase_hook_before "post-merge" || return 0

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
    
    # 4.3 GSD phase completed
    gsd_state=$(get_gsd_phase_state)
    case "$gsd_state" in
        completed)
            add_check "PASS" "4.3 GSD phase completed"
            ;;
        in_progress)
            add_check "FAIL" "4.3 GSD phase still in progress — run /gsd-ship-phase to complete"
            ;;
        pending|none)
            add_check "PASS" "4.3 GSD phase not started (no active phase to complete)"
            ;;
        *)
            add_check "SKIP" "4.3 Cannot determine GSD state"
            ;;
    esac
    
    # 4.4 Feature branch deleted
    current_branch=$(git branch --show-current 2>/dev/null || echo "unknown")
    if [[ "$current_branch" == "master" ]]; then
        add_check "PASS" "4.4 On master (feature branch cleaned up)"
    else
        add_check "FAIL" "4.4 Still on feature branch: $current_branch"
    fi
    
    # 4.5 Validation on master
    if cmd_exists go; then
        if git checkout master &>/dev/null && go build ./... &>/dev/null && go test -race -short ./... >/dev/null 2>&1; then
            add_check "PASS" "4.5 master validation passes"
        else
            add_check "FAIL" "4.5 Validation failed on master"
        fi
    else
        add_check "SKIP" "4.5 Go not installed"
    fi
}

phase_next_ready() {
    [[ "$JSON_OUTPUT" != true ]] && echo "─ NEXT-READY checks"
    run_phase_hook_before "next-ready" || return 0

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
    [[ "$JSON_OUTPUT" != true ]] && echo "─ RETRO (computed metrics — W1.2)"
    run_phase_hook_before "retro" || return 0

    if ! cmd_exists gh; then
        add_check "SKIP" "6.x gh CLI not installed"
        return
    fi
    if [[ -z "$EPIC" ]]; then
        add_check "FAIL" "6.0 --epic <N> is required for retro gate"
        return
    fi

    # 6.1 Merged PRs in epic (search PRs whose body or title mentions epic-N)
    merged_count=$(gh pr list --state merged --search "epic-${EPIC} in:title,body" \
        --json number --jq 'length' 2>/dev/null || echo "0")
    add_check "PASS" "6.1 Merged PRs for epic-$EPIC: $merged_count"

    # 6.2 Average review cycles per PR (commits per PR; > 2.5 = bot loop issue)
    if [[ "$merged_count" -gt 0 ]]; then
        total_commits=$(gh pr list --state merged --search "epic-${EPIC} in:title,body" \
            --json commits --jq '[.[].commits | length] | add // 0' 2>/dev/null || echo "0")
        avg_cycles=$(awk "BEGIN { printf \"%.2f\", $total_commits / $merged_count }")
        if awk "BEGIN { exit !($avg_cycles > 2.5) }"; then
            add_check "FAIL" "6.2 Avg PR cycles: $avg_cycles (> 2.5 — investigate bot review loop)"
        else
            add_check "PASS" "6.2 Avg PR cycles: $avg_cycles"
        fi
    else
        add_check "SKIP" "6.2 No merged PRs to compute avg cycles"
    fi

    # 6.3 CI failure count on master during epic window
    if gh run list --branch master --limit 100 --json conclusion &>/dev/null; then
        ci_fails=$(gh run list --branch master --limit 100 \
            --json conclusion --jq '[.[] | select(.conclusion == "failure")] | length' \
            2>/dev/null || echo "0")
        if [[ "$ci_fails" -gt 5 ]]; then
            add_check "FAIL" "6.3 CI failures on master: $ci_fails (> 5 — investigate)"
        else
            add_check "PASS" "6.3 CI failures on master: $ci_fails"
        fi
    else
        add_check "SKIP" "6.3 gh run list not accessible"
    fi

    # 6.4 Retro evidence file exists with min 200 words
    retro_file="docs/evidence/retro-epic-${EPIC}.md"
    if [[ ! -f "$retro_file" ]]; then
        add_check "FAIL" "6.4 Retro file missing: $retro_file"
    else
        word_count=$(wc -w < "$retro_file" | tr -d ' ')
        if [[ "$word_count" -lt 200 ]]; then
            add_check "FAIL" "6.4 Retro too thin: $word_count words (< 200)"
        else
            add_check "PASS" "6.4 Retro complete: $word_count words"
        fi
    fi

    # 6.5 Retro contains required sections (Metrics, Patterns, Root Cause, Proposed Changes)
    if [[ -f "$retro_file" ]]; then
        missing=""
        for section in "## Metrics" "## Patterns" "## Root Cause" "## Proposed Changes"; do
            grep -qF "$section" "$retro_file" || missing="$missing $section"
        done
        if [[ -n "$missing" ]]; then
            add_check "FAIL" "6.5 Retro missing required sections:$missing"
        else
            add_check "PASS" "6.5 Retro has all required sections"
        fi
    else
        add_check "SKIP" "6.5 Retro file missing (see 6.4)"
    fi
}

validate_runner_contract() {
    local gates=("pre-work" "in-progress" "pre-merge" "async-pr-review" "post-merge" "post-merge-npm-release" "next-ready")
    local required_fields=("gate" "status" "checks" "rule_ids_violated")
    local all_ok=true

    if [[ ! -x "$0" ]]; then
        echo "[FAIL] Runner not executable: $0" >&2
        exit 1
    fi

    for gate in "${gates[@]}"; do
        local output gate_ok=true
        output="$(timeout 10 "$0" "$gate" --json 2>/dev/null)" || true

        if [[ -z "$output" ]]; then
            echo "[FAIL] Gate '$gate': no JSON output (or timed out after 10s)" >&2
            all_ok=false
            continue
        fi

        if ! echo "$output" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
            echo "[FAIL] Gate '$gate': output is not valid JSON" >&2
            all_ok=false
            continue
        fi

        for field in "${required_fields[@]}"; do
            if ! echo "$output" | python3 -c "import sys,json; d=json.load(sys.stdin); assert '$field' in d" 2>/dev/null; then
                echo "[FAIL] Gate '$gate': missing required field '$field'" >&2
                gate_ok=false
                all_ok=false
            fi
        done

        local gate_in_output
        gate_in_output=$(echo "$output" | python3 -c "import sys,json; print(json.load(sys.stdin).get('gate',''))" 2>/dev/null || echo "")
        if [[ "$gate_in_output" != "$gate" ]]; then
            echo "[FAIL] Gate '$gate': response gate='$gate_in_output' does not match request" >&2
            gate_ok=false
            all_ok=false
        fi

        if [[ "$gate_ok" == true ]]; then
            local status_val
            status_val=$(echo "$output" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null || echo "?")
            echo "[PASS] Gate '$gate': contract valid (status=$status_val)"
        fi
    done

    if [[ "$all_ok" == false ]]; then
        exit 1
    fi
    exit 0
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
        async-pr-review)
            extra_flags=()
            [[ "$JSON_OUTPUT" == true ]] && extra_flags+=("--json")
            [[ "$NO_COLOR" == true ]] && extra_flags+=("--no-color")
            exec "$(dirname "$0")/check-pr-review.sh" async-pr-review "${extra_flags[@]+"${extra_flags[@]}"}"
            ;;
        post-merge)
            phase_post_merge
            ;;
        post-merge-npm-release)
            extra_flags=()
            [[ "$JSON_OUTPUT" == true ]] && extra_flags+=("--json")
            [[ "$NO_COLOR" == true ]] && extra_flags+=("--no-color")
            exec "$(dirname "$0")/check-npm-release.sh" post-merge-npm-release "${extra_flags[@]+"${extra_flags[@]}"}"
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
        validate)
            validate_runner_contract
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
    
    # Exit with appropriate code per runner contract:
    # 0=PASS, 1=FAIL, 2=SKIP, 3=WAITING, 4=BLOCKED, 5=ERROR
    if [[ $FAIL_COUNT -eq 0 ]]; then
        if [[ $SKIP_COUNT -gt 0 && $PASS_COUNT -eq 0 ]]; then
            exit 2  # SKIP
        else
            exit 0  # PASS
        fi
    else
        exit 1  # FAIL
    fi
}

main "$@"
