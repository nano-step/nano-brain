#!/usr/bin/env bash
# smoke-ui.sh — Web UI asset smoke test (HARNESS.md smoke:ui layer, issue #285)
#
# Builds the dev binary, starts it on port 3199 in serve_only mode, then
# verifies /ui/ and all referenced assets serve with correct MIME types and
# sufficient body sizes. Catches the class of bugs from #275/#277/#278/#281.
#
# Usage: ./scripts/smoke-ui.sh
# Exit: 0 on PASS, non-zero on FAIL
# Evidence: append stdout to docs/evidence/<change-slug>/smoke-ui-output.log

set -euo pipefail

PORT=3199
BUILD_DIR=/tmp/nano-brain-smoke
BINARY="${BUILD_DIR}/nano-brain"
HEALTH_TIMEOUT=15
MIN_JS_SIZE=1024
MIN_CSS_SIZE=100

SERVER_PID=""
FAIL_COUNT=0

cleanup() {
    if [[ -n "${SERVER_PID:-}" ]] && kill -0 "${SERVER_PID}" 2>/dev/null; then
        kill "${SERVER_PID}" 2>/dev/null || true
        wait "${SERVER_PID}" 2>/dev/null || true
        echo "Server stopped (PID=${SERVER_PID})"
    fi
}
trap cleanup EXIT INT TERM

log_header() { echo "=== smoke:ui run $(date -u +%Y-%m-%dT%H:%M:%SZ) ==="; }
log_pass()   { echo "PASS: $*"; }
log_fail()   { echo "FAIL: $*"; FAIL_COUNT=$((FAIL_COUNT + 1)); }
log_info()   { echo "INFO: $*"; }

build_binary() {
    mkdir -p "${BUILD_DIR}"
    if go build -o "${BINARY}" ./cmd/nano-brain 2>&1; then
        local size
        size=$(stat -c %s "${BINARY}" 2>/dev/null || stat -f %z "${BINARY}")
        log_info "Binary built: ${BINARY} (size=${size} bytes)"
    else
        log_fail "go build ./cmd/nano-brain failed"
        finalize
        exit 1
    fi
}
start_server() {
    local cfg_path="${BUILD_DIR}/config.yml"
    local db_url="${NANO_BRAIN_DATABASE_URL:-postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev}"
    cat > "${cfg_path}" <<EOF
server:
    host: 0.0.0.0
    port: ${PORT}
    serve_only: true
database:
    url: ${db_url}
embedding:
    provider: ""
harvester:
    opencode:
        db_root: ""
        db_path: ""
        session_dir: ""
    claudecode:
        enabled: false
summarization:
    enabled: false
logging:
    level: warn
EOF

    NANO_BRAIN_CONFIG="${cfg_path}" \
    NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1 \
        "${BINARY}" serve --unsafe-no-auth --serve-only \
        > "${BUILD_DIR}/server.log" 2>&1 &
    SERVER_PID=$!
    log_info "Server started on port ${PORT} (PID=${SERVER_PID})"
}
wait_for_health() {
    local i
    for i in $(seq 1 ${HEALTH_TIMEOUT}); do
        if ! kill -0 "${SERVER_PID}" 2>/dev/null; then
            log_fail "Server process (PID=${SERVER_PID}) died before /health became ready"
            log_info "Server log tail:"
            tail -20 "${BUILD_DIR}/server.log" || true
            finalize
            exit 1
        fi
        if curl -sf "http://localhost:${PORT}/health" > /dev/null 2>&1; then
            local status
            status=$(curl -sf "http://localhost:${PORT}/health")
            log_pass "/health → 200 OK ${status}"
            return 0
        fi
        sleep 1
    done
    log_fail "/health did not return 200 after ${HEALTH_TIMEOUT}s"
    log_info "Server log tail:"
    tail -20 "${BUILD_DIR}/server.log" || true
    finalize
    exit 1
}
check_ui_root() {
    UI_HTML_BODY=""
    local tmp status content_type body
    tmp=$(mktemp)
    local meta
    meta=$(curl -so "${tmp}" -w "%{http_code} %{content_type}" "http://localhost:${PORT}/ui/")
    status="${meta%% *}"
    content_type="${meta#* }"
    body=$(cat "${tmp}")
    rm -f "${tmp}"

    if [[ "${status}" != "200" ]]; then
        log_fail "/ui/ HTTP status=${status} (expected 200)"
        return
    fi
    if [[ -z "${body}" ]]; then
        log_fail "/ui/ returned empty body"
        return
    fi
    if echo "${content_type}" | grep -qi '^text/html'; then
        log_pass "/ui/ Content-Type=${content_type}"
    else
        log_fail "/ui/ wrong Content-Type=${content_type} (expected text/html)"
        return
    fi
    if echo "${body}" | grep -qi '<!DOCTYPE html>'; then
        log_pass "/ui/ body has <!DOCTYPE html>"
    else
        log_fail "/ui/ body missing <!DOCTYPE html> (not HTML?)"
        return
    fi
    if echo "${body}" | grep -q '<script'; then
        log_pass "/ui/ HTML contains <script> tag"
    else
        log_fail "/ui/ HTML missing <script> tag"
    fi
    UI_HTML_BODY="${body}"
}
check_assets() {
    local urls js_count css_count
    urls=$(echo "${UI_HTML_BODY}" | grep -oE '/ui/assets/[A-Za-z0-9_.-]+\.(js|css)' | sort -u)
    if [[ -z "${urls}" ]]; then
        log_fail "no /ui/assets/* references found in /ui/ HTML"
        return
    fi
    js_count=0
    css_count=0
    while IFS= read -r url; do
        check_one_asset "${url}" || true
        case "${url}" in
            *.js)  js_count=$((js_count + 1)) ;;
            *.css) css_count=$((css_count + 1)) ;;
        esac
    done <<< "${urls}"
    log_info "Asset summary: ${js_count} JS, ${css_count} CSS"
    if [[ ${js_count} -eq 0 ]]; then
        log_fail "no JS assets found in /ui/ HTML"
    fi
}

check_one_asset() {
    local url="$1"
    local full="http://localhost:${PORT}${url}"
    local tmp status content_type size min_size expected_ct body_head meta
    tmp=$(mktemp)
    meta=$(curl -so "${tmp}" -w "%{http_code} %{content_type} %{size_download}" "${full}")
    status="${meta%% *}"
    rest="${meta#* }"
    content_type="${rest%% *}"
    size="${rest##* }"
    body_head=$(head -c 100 "${tmp}")
    rm -f "${tmp}"

    case "${url}" in
        *.js)
            min_size=${MIN_JS_SIZE}
            expected_ct="application/javascript"
            ;;
        *.css)
            min_size=${MIN_CSS_SIZE}
            expected_ct="text/css"
            ;;
        *)
            min_size=0
            expected_ct=""
            ;;
    esac

    if [[ "${status}" != "200" ]]; then
        log_fail "${url} status=${status} (expected 200)"
        return 1
    fi
    if [[ -n "${expected_ct}" ]] && ! echo "${content_type}" | grep -qi "${expected_ct}"; then
        log_fail "${url} Content-Type=${content_type} (expected ${expected_ct})"
        return 1
    fi
    if echo "${body_head}" | grep -qi '<!DOCTYPE html>'; then
        log_fail "${url} body starts with <!DOCTYPE html> — server fell back to index.html (#275 regression)"
        return 1
    fi
    if [[ "${size}" -lt "${min_size}" ]]; then
        log_fail "${url} body size=${size} bytes (expected > ${min_size}); likely HTML fallback (#275 regression)"
        return 1
    fi
    log_pass "${url} → 200 ${content_type} size=${size}"
}
finalize() {
    if [[ ${FAIL_COUNT} -eq 0 ]]; then
        echo "=== smoke:ui PASS ==="
        exit 0
    else
        echo "=== smoke:ui FAIL: ${FAIL_COUNT} check(s) failed ==="
        exit 1
    fi
}

main() {
    log_header
    build_binary
    start_server
    wait_for_health
    check_ui_root
    check_assets
    finalize
}

main "$@"
