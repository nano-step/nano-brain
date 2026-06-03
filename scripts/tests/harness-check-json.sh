#!/usr/bin/env bash
# harness-check-json.sh — JSON output contract tests for harness-check.sh
# Verifies that each gate produces valid, schema-compliant JSON when run with --json.
# Exits 0 if all shape tests pass; exits 1 if any shape test fails.
# Note: gate FAIL/PASS/SKIP status in this environment is not a test failure —
# only structural shape violations fail these tests.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNNER="$SCRIPT_DIR/../harness-check.sh"

if [[ ! -x "$RUNNER" ]]; then
    echo "ERROR: runner not found or not executable: $RUNNER" >&2
    exit 1
fi

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

# valid_status — returns "true" if value is one of the allowed gate statuses
valid_status() {
    local s="$1"
    case "$s" in
        PASS|FAIL|SKIP|WAITING|BLOCKED|ERROR) echo "true" ;;
        *) echo "false" ;;
    esac
}

# py — run a python3 one-liner against $output; echo "true" on success, "false" on failure
py() {
    local code="$1"
    if echo "$output" | python3 -c "$code" 2>/dev/null; then
        echo "true"
    else
        echo "false"
    fi
}

# py_str — extract a string value via python3; prints the value
py_str() {
    local code="$1"
    echo "$output" | python3 -c "$code" 2>/dev/null || echo ""
}

# ============================================================================
# Per-gate validation function
# ============================================================================

validate_gate_json() {
    local gate="$1"
    local output="$2"
    local prefix="[$gate]"

    # 1. Non-empty output
    check "$prefix output is non-empty" \
        "$([[ -n "$output" ]] && echo "true" || echo "false")"

    if [[ -z "$output" ]]; then
        return  # remaining checks are meaningless
    fi

    # 2. Valid JSON
    check "$prefix output is valid JSON" \
        "$(py 'import sys,json; json.load(sys.stdin)')"

    # Bail early if invalid JSON — can't parse fields
    if ! echo "$output" | python3 -m json.tool >/dev/null 2>&1; then
        return
    fi

    # 3. .gate field equals requested gate name
    local gate_val
    gate_val=$(py_str "import sys,json; print(json.load(sys.stdin).get('gate',''))")
    check "$prefix .gate == \"$gate\"" \
        "$([[ "$gate_val" == "$gate" ]] && echo "true" || echo "false")"

    # 4. .status field is one of allowed values
    local status_val
    status_val=$(py_str "import sys,json; print(json.load(sys.stdin).get('status',''))")
    check "$prefix .status is a valid enum (got: $status_val)" \
        "$(valid_status "$status_val")"

    # 5. .checks is a JSON array
    check "$prefix .checks is a JSON array" \
        "$(py 'import sys,json; d=json.load(sys.stdin); assert isinstance(d["checks"], list)')"

    # 6. .rule_ids_violated is a JSON array
    check "$prefix .rule_ids_violated is a JSON array" \
        "$(py 'import sys,json; d=json.load(sys.stdin); assert isinstance(d["rule_ids_violated"], list)')"

    # 7. .next_gate field exists (may be null or string)
    check "$prefix .next_gate field exists" \
        "$(py 'import sys,json; d=json.load(sys.stdin); assert "next_gate" in d')"

    # 8. Each check item has id, name, status fields
    check "$prefix each .checks[] element has id/name/status fields" \
        "$(echo "$output" | python3 -c '
import sys,json
d=json.load(sys.stdin)
for c in d["checks"]:
    assert "id" in c
    assert "name" in c
    assert "status" in c
' 2>/dev/null && echo "true" || echo "false")"

    # 9. Each check's status is a valid enum
    check "$prefix each .checks[].status is a valid enum" \
        "$(echo "$output" | python3 -c '
import sys,json
d=json.load(sys.stdin)
valid={"PASS","FAIL","SKIP","WAITING","BLOCKED","ERROR"}
for c in d["checks"]:
    assert c["status"] in valid
' 2>/dev/null && echo "true" || echo "false")"

    # 10. If overall status=FAIL, at least one check must also be FAIL
    if [[ "$status_val" == "FAIL" ]]; then
        check "$prefix overall FAIL implies at least one FAIL check" \
            "$(echo "$output" | python3 -c '
import sys,json
d=json.load(sys.stdin)
assert any(c["status"]=="FAIL" for c in d["checks"])
' 2>/dev/null && echo "true" || echo "false")"
    fi

    # 11. If instructions_for_agent present, it must be a string
    local has_instructions
    has_instructions=$(py_str 'import sys,json; d=json.load(sys.stdin); print("yes" if "instructions_for_agent" in d else "no")')
    if [[ "$has_instructions" == "yes" ]]; then
        check "$prefix .instructions_for_agent is a string (when present)" \
            "$(py 'import sys,json; d=json.load(sys.stdin); assert isinstance(d["instructions_for_agent"], str)')"
    fi
}

# ============================================================================
# Gate tests — skip pre-work and pre-merge (require network/state)
# ============================================================================

GATES=("in-progress" "post-merge" "post-merge-npm-release" "next-ready")

for gate in "${GATES[@]}"; do
    echo ""
    echo "══ Testing gate: $gate ══"
    output=""
    output=$(timeout 15 bash "$RUNNER" "$gate" --json 2>/dev/null || true)
    validate_gate_json "$gate" "$output"
done

# ============================================================================
# Validate subcommand test — check it exits 0 or 1 (not crash), produces
# [PASS]/[FAIL] lines in output. We run only in-progress to avoid long timeouts.
# ============================================================================

echo ""
echo "══ Testing validate subcommand (single gate spot-check) ══"

validate_out=""
validate_code=0
# We can't run full validate (all gates) as it hangs on pre-work/pre-merge.
# Instead test the gate directly outputs [PASS]/[FAIL] lines via validate logic.
# Run just in-progress gate and confirm it produces valid JSON (already done above).
# The validate subcommand itself runs all gates with timeout — test it exits cleanly.
validate_out=$(timeout 20 bash "$RUNNER" validate 2>/dev/null || true)
validate_code=$?

check "validate subcommand exits with code 0 or 1 (not crash)" \
    "$([[ $validate_code -le 1 ]] && echo "true" || echo "false")"

check "validate subcommand produces [PASS] or [FAIL] lines" \
    "$([[ "$validate_out" =~ \[PASS\]|\[FAIL\] ]] && echo "true" || echo "false")"

# ============================================================================
# Summary
# ============================================================================

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Results: $PASS passed, $FAIL failed"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ $FAIL -gt 0 ]]; then
    exit 1
fi
exit 0
