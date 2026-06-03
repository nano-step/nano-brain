#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNNER="$SCRIPT_DIR/../check-npm-release.sh"
PASS=0
FAIL=0

check() {
    local desc="$1" result="$2"
    if [[ "$result" == "true" ]]; then
        echo "[PASS] $desc"
        PASS=$((PASS + 1))
    else
        echo "[FAIL] $desc"
        FAIL=$((FAIL + 1))
    fi
}

run_with_stubs() {
    local stub_dir="$1"
    shift
    PATH="$stub_dir:$PATH" "$RUNNER" "$@"
}

make_gh_stub() {
    local stub_dir="$1"
    local tag="$2"
    local workflow_status="$3"
    local gh_stub="$stub_dir/gh"
    cat > "$gh_stub" <<STUB
#!/usr/bin/env bash
case "\$*" in
  *"tagName"*) echo "$tag" ;;
  *"release.yml"*"status"*) echo "$workflow_status" ;;
  *) exit 0 ;;
esac
STUB
    chmod +x "$gh_stub"
}

make_npm_stub() {
    local stub_dir="$1"
    local version="$2"
    local npm_stub="$stub_dir/npm"
    cat > "$npm_stub" <<STUB
#!/usr/bin/env bash
echo "$version"
STUB
    chmod +x "$npm_stub"
}

make_gh_no_tag_stub() {
    local stub_dir="$1"
    local gh_stub="$stub_dir/gh"
    cat > "$gh_stub" <<STUB
#!/usr/bin/env bash
echo ""
STUB
    chmod +x "$gh_stub"
}

make_gh_no_gh_stub() {
    local stub_dir="$1"
    cat > "$stub_dir/gh" <<STUB
#!/usr/bin/env bash
exit 127
STUB
}

# ============================================================================
# Scenario 1: matching versions → PASS
# ============================================================================
echo ""
echo "── Scenario 1: matching versions (PASS)"
TMP1=$(mktemp -d)
make_gh_stub "$TMP1" "v2026.5.1.1" "completed"
make_npm_stub "$TMP1" "2026.5.1.1"

out=$(run_with_stubs "$TMP1" post-merge-npm-release --json 2>/dev/null || true)
status=$(echo "$out" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null || echo "PARSE_ERROR")
checks_len=$(echo "$out" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['checks']))" 2>/dev/null || echo "0")

check "Scenario 1: status=PASS" "$([[ "$status" == "PASS" ]] && echo true || echo false)"
check "Scenario 1: 3 checks emitted" "$([[ "$checks_len" == "3" ]] && echo true || echo false)"
rm -rf "$TMP1"

# ============================================================================
# Scenario 2: workflow still in_progress → WAITING
# ============================================================================
echo ""
echo "── Scenario 2: workflow in_progress (WAITING)"
TMP2=$(mktemp -d)
make_gh_stub "$TMP2" "v2026.5.1.2" "in_progress"
make_npm_stub "$TMP2" "2026.5.1.1"

out=$(run_with_stubs "$TMP2" post-merge-npm-release --json 2>/dev/null || true)
status=$(echo "$out" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null || echo "PARSE_ERROR")
has_wait=$(echo "$out" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if 'wait_seconds' in d else 'no')" 2>/dev/null || echo "no")

check "Scenario 2: status=WAITING" "$([[ "$status" == "WAITING" ]] && echo true || echo false)"
check "Scenario 2: wait_seconds present" "$([[ "$has_wait" == "yes" ]] && echo true || echo false)"
rm -rf "$TMP2"

# ============================================================================
# Scenario 3: npm version mismatch → FAIL
# ============================================================================
echo ""
echo "── Scenario 3: npm version mismatch (FAIL)"
TMP3=$(mktemp -d)
make_gh_stub "$TMP3" "v2026.5.1.3" "completed"
make_npm_stub "$TMP3" "2026.5.1.1"

out=$(run_with_stubs "$TMP3" post-merge-npm-release --json 2>/dev/null || true)
status=$(echo "$out" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null || echo "PARSE_ERROR")
has_instructions=$(echo "$out" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if d.get('instructions_for_agent') else 'no')" 2>/dev/null || echo "no")

check "Scenario 3: status=FAIL" "$([[ "$status" == "FAIL" ]] && echo true || echo false)"
check "Scenario 3: instructions_for_agent present" "$([[ "$has_instructions" == "yes" ]] && echo true || echo false)"
rm -rf "$TMP3"

# ============================================================================
# Scenario 4: no recent tag → SKIP
# ============================================================================
echo ""
echo "── Scenario 4: no recent GH tag (SKIP)"
TMP4=$(mktemp -d)
make_gh_no_tag_stub "$TMP4"
make_npm_stub "$TMP4" "2026.5.1.1"

out=$(run_with_stubs "$TMP4" post-merge-npm-release --json 2>/dev/null || true)
status=$(echo "$out" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null || echo "PARSE_ERROR")

check "Scenario 4: status=SKIP" "$([[ "$status" == "SKIP" ]] && echo true || echo false)"
rm -rf "$TMP4"

# ============================================================================
# Scenario 5: JSON shape contract — all scenarios emit valid JSON with required fields
# ============================================================================
echo ""
echo "── Scenario 5: JSON shape contract"
TMP5=$(mktemp -d)
make_gh_stub "$TMP5" "v2026.5.1.1" "completed"
make_npm_stub "$TMP5" "2026.5.1.1"

out=$(run_with_stubs "$TMP5" post-merge-npm-release --json 2>/dev/null || true)

for field in gate status checks rule_ids_violated; do
    has=$(echo "$out" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if '$field' in d else 'no')" 2>/dev/null || echo "no")
    check "Scenario 5: field '$field' present" "$([[ "$has" == "yes" ]] && echo true || echo false)"
done

gate_val=$(echo "$out" | python3 -c "import sys,json; print(json.load(sys.stdin)['gate'])" 2>/dev/null || echo "")
check "Scenario 5: gate field = 'post-merge-npm-release'" "$([[ "$gate_val" == "post-merge-npm-release" ]] && echo true || echo false)"
rm -rf "$TMP5"

# ============================================================================
# Summary
# ============================================================================
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Results: $PASS PASS, $FAIL FAIL"
if [[ $FAIL -gt 0 ]]; then
    exit 1
fi
exit 0
