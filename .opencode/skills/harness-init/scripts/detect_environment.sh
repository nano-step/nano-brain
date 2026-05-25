#!/usr/bin/env bash
# detect_environment.sh — emit JSON describing the project's stack and tooling.
# Usage: detect_environment.sh [TARGET_DIR]   (default: current dir)
# Output: single-line JSON object to stdout. Errors to stderr.

set -uo pipefail

TARGET="${1:-$PWD}"
cd "$TARGET" 2>/dev/null || { echo "detect: cannot cd $TARGET" >&2; exit 1; }

json_escape() {
  python3 -c 'import json,sys;print(json.dumps(sys.stdin.read().rstrip()))' 2>/dev/null \
    || sed 's/\\/\\\\/g; s/"/\\"/g; s/$/\\n/' | tr -d '\n' | sed 's/^/"/; s/$/"/'
}

has_cmd() { command -v "$1" >/dev/null 2>&1; }
file_exists() { [[ -e "$1" ]]; }

# --- cwd + project name --------------------------------------------------
CWD="$(pwd)"
PROJECT_NAME=""
if file_exists package.json && has_cmd python3; then
  PROJECT_NAME="$(python3 -c 'import json,sys;print(json.load(open("package.json")).get("name",""))' 2>/dev/null)"
fi
if [[ -z "$PROJECT_NAME" ]] && file_exists pyproject.toml; then
  PROJECT_NAME="$(grep -E '^name\s*=' pyproject.toml 2>/dev/null | head -1 | sed -E 's/^name\s*=\s*"([^"]+)".*/\1/')"
fi
if [[ -z "$PROJECT_NAME" ]] && file_exists Cargo.toml; then
  PROJECT_NAME="$(grep -E '^name\s*=' Cargo.toml 2>/dev/null | head -1 | sed -E 's/^name\s*=\s*"([^"]+)".*/\1/')"
fi
[[ -z "$PROJECT_NAME" ]] && PROJECT_NAME="$(basename "$CWD")"

# --- git ----------------------------------------------------------------
GIT_REPO="false"
GIT_REMOTE=""
GIT_DEFAULT_BRANCH=""
GIT_OWNER_REPO=""
if [[ -d .git ]] || git rev-parse --git-dir >/dev/null 2>&1; then
  GIT_REPO="true"
  GIT_REMOTE="$(git remote get-url origin 2>/dev/null || echo "")"
  GIT_DEFAULT_BRANCH="$(git symbolic-ref --short HEAD 2>/dev/null \
    || git rev-parse --abbrev-ref HEAD 2>/dev/null \
    || echo "main")"
  if [[ -n "$GIT_REMOTE" ]]; then
    GIT_OWNER_REPO="$(echo "$GIT_REMOTE" \
      | sed -E 's#\.git$##' \
      | sed -E 's#^.*github\.com[^:/]*[:/]+##' \
      | grep -E '^[^/]+/[^/]+$' || true)"
  fi
fi

# --- gh CLI -------------------------------------------------------------
GH_INSTALLED="false"
GH_AUTHED="false"
GH_ACTIVE_ACCOUNT=""
GH_ACCOUNTS_JSON="[]"
if has_cmd gh; then
  GH_INSTALLED="true"
  if gh auth status >/dev/null 2>&1; then
    GH_AUTHED="true"
    GH_ACTIVE_ACCOUNT="$(gh auth status 2>&1 | grep -E 'Active account: true' -B1 \
      | grep -E 'Logged in to .* account ' \
      | sed -E 's/.*account ([^ ]+).*/\1/' | head -1 || true)"
    GH_ACCOUNTS_JSON="$(gh auth status 2>&1 | grep -E 'Logged in to .* account ' \
      | sed -E 's/.*account ([^ ]+).*/\1/' \
      | python3 -c 'import sys,json;print(json.dumps([l.strip() for l in sys.stdin if l.strip()]))' 2>/dev/null || echo "[]")"
  fi
fi

# --- openspec ----------------------------------------------------------
OPENSPEC_INSTALLED="false"
OPENSPEC_INITIALIZED="false"
has_cmd openspec && OPENSPEC_INSTALLED="true"
file_exists openspec/config.yaml && OPENSPEC_INITIALIZED="true"

# --- MCP issue trackers (best-effort by env hints) ----------------------
MCP_TRACKERS_JSON='[]'
detected_trackers=()
[[ -n "${LINEAR_API_KEY:-}" ]] && detected_trackers+=("linear")
[[ -n "${JIRA_API_TOKEN:-}" ]] && detected_trackers+=("jira")
[[ "$GH_AUTHED" == "true" ]] && detected_trackers+=("github")
# opencode-mcp config probe
if file_exists "$HOME/.config/opencode/opencode.json"; then
  grep -qi 'linear' "$HOME/.config/opencode/opencode.json" 2>/dev/null && \
    [[ ! " ${detected_trackers[*]} " =~ " linear " ]] && detected_trackers+=("linear")
  grep -qi 'jira' "$HOME/.config/opencode/opencode.json" 2>/dev/null && \
    [[ ! " ${detected_trackers[*]} " =~ " jira " ]] && detected_trackers+=("jira")
fi
if [[ ${#detected_trackers[@]} -gt 0 ]]; then
  MCP_TRACKERS_JSON="$(printf '%s\n' "${detected_trackers[@]}" \
    | python3 -c 'import sys,json;print(json.dumps([l.strip() for l in sys.stdin if l.strip()]))' 2>/dev/null \
    || echo "[]")"
fi

# --- stack signals -----------------------------------------------------
STACK_SIGNALS=()
file_exists pyproject.toml && STACK_SIGNALS+=("python")
file_exists requirements.txt && STACK_SIGNALS+=("python")
file_exists package.json && STACK_SIGNALS+=("typescript-or-javascript")
file_exists tsconfig.json && STACK_SIGNALS+=("typescript")
file_exists go.mod && STACK_SIGNALS+=("go")
file_exists Cargo.toml && STACK_SIGNALS+=("rust")
file_exists Gemfile && STACK_SIGNALS+=("ruby")
file_exists composer.json && STACK_SIGNALS+=("php")
file_exists deno.json && STACK_SIGNALS+=("deno")
# dedupe
if [[ ${#STACK_SIGNALS[@]} -gt 0 ]]; then
  STACK_JSON="$(printf '%s\n' "${STACK_SIGNALS[@]}" | sort -u \
    | python3 -c 'import sys,json;print(json.dumps([l.strip() for l in sys.stdin if l.strip()]))' 2>/dev/null \
    || echo "[]")"
else
  STACK_JSON="[]"
fi

# --- test runners -----------------------------------------------------
RUNNERS=()
file_exists pytest.ini && RUNNERS+=("pytest")
file_exists pyproject.toml && grep -q "\[tool\.pytest" pyproject.toml 2>/dev/null && RUNNERS+=("pytest")
file_exists vitest.config.ts && RUNNERS+=("vitest")
file_exists vitest.config.js && RUNNERS+=("vitest")
file_exists jest.config.js && RUNNERS+=("jest")
file_exists jest.config.ts && RUNNERS+=("jest")
file_exists playwright.config.ts && RUNNERS+=("playwright")
file_exists playwright.config.js && RUNNERS+=("playwright")
file_exists cypress.config.ts && RUNNERS+=("cypress")
file_exists cypress.config.js && RUNNERS+=("cypress")
[[ -d tests ]] && RUNNERS+=("pytest")  # heuristic
if [[ ${#RUNNERS[@]} -gt 0 ]]; then
  RUNNERS_JSON="$(printf '%s\n' "${RUNNERS[@]}" | sort -u \
    | python3 -c 'import sys,json;print(json.dumps([l.strip() for l in sys.stdin if l.strip()]))' 2>/dev/null \
    || echo "[]")"
else
  RUNNERS_JSON="[]"
fi

# --- linters / type-check -----------------------------------------------
LINTERS=()
{ file_exists .ruff.toml || file_exists ruff.toml; } && LINTERS+=("ruff")
file_exists pyproject.toml && grep -q "\[tool\.ruff" pyproject.toml 2>/dev/null && LINTERS+=("ruff")
file_exists pyproject.toml && grep -q "\[tool\.mypy" pyproject.toml 2>/dev/null && LINTERS+=("mypy")
file_exists mypy.ini && LINTERS+=("mypy")
file_exists .importlinter && LINTERS+=("lint-imports")
file_exists .eslintrc.json && LINTERS+=("eslint")
file_exists .eslintrc.js && LINTERS+=("eslint")
file_exists eslint.config.js && LINTERS+=("eslint")
file_exists eslint.config.mjs && LINTERS+=("eslint")
file_exists tsconfig.json && LINTERS+=("tsc")
file_exists .biome.json && LINTERS+=("biome")
file_exists biome.json && LINTERS+=("biome")
if [[ ${#LINTERS[@]} -gt 0 ]]; then
  LINTERS_JSON="$(printf '%s\n' "${LINTERS[@]}" | sort -u \
    | python3 -c 'import sys,json;print(json.dumps([l.strip() for l in sys.stdin if l.strip()]))' 2>/dev/null \
    || echo "[]")"
else
  LINTERS_JSON="[]"
fi

# --- existing harness files -------------------------------------------
EX_HARNESS_MD="false"
EX_FEATURE_INTAKE="false"
EX_STORY_TEMPLATE="false"
EX_HARNESS_BACKLOG="false"
EX_AGENTS_MD="false"
EX_EVIDENCE_DIR="false"
[[ -f docs/HARNESS.md ]] && EX_HARNESS_MD="true"
[[ -f docs/FEATURE_INTAKE.md ]] && EX_FEATURE_INTAKE="true"
[[ -f docs/templates/story.md ]] && EX_STORY_TEMPLATE="true"
[[ -f docs/HARNESS_BACKLOG.md ]] && EX_HARNESS_BACKLOG="true"
[[ -f AGENTS.md ]] && EX_AGENTS_MD="true"
[[ -d docs/evidence ]] && EX_EVIDENCE_DIR="true"

# --- container vs host detection ---------------------------------------
IN_CONTAINER="false"
if [[ -f /.dockerenv ]] || grep -qE '(docker|kubepod|lxc)' /proc/1/cgroup 2>/dev/null; then
  IN_CONTAINER="true"
fi

# --- emit JSON ---------------------------------------------------------
python3 <<PYEOF
import json
out = {
    "cwd": ${CWD@Q},
    "project_name": ${PROJECT_NAME@Q},
    "in_container": ${IN_CONTAINER@Q} == "true",
    "git": {
        "is_repo": ${GIT_REPO@Q} == "true",
        "remote": ${GIT_REMOTE@Q},
        "owner_repo": ${GIT_OWNER_REPO@Q},
        "default_branch": ${GIT_DEFAULT_BRANCH@Q},
    },
    "gh": {
        "installed": ${GH_INSTALLED@Q} == "true",
        "authed": ${GH_AUTHED@Q} == "true",
        "active_account": ${GH_ACTIVE_ACCOUNT@Q},
        "accounts": json.loads('${GH_ACCOUNTS_JSON}'),
    },
    "openspec": {
        "installed": ${OPENSPEC_INSTALLED@Q} == "true",
        "initialized": ${OPENSPEC_INITIALIZED@Q} == "true",
    },
    "mcp_issue_trackers": json.loads('${MCP_TRACKERS_JSON}'),
    "stack_signals": json.loads('${STACK_JSON}'),
    "test_runners": json.loads('${RUNNERS_JSON}'),
    "linters": json.loads('${LINTERS_JSON}'),
    "existing_files": {
        "harness_md": ${EX_HARNESS_MD@Q} == "true",
        "feature_intake": ${EX_FEATURE_INTAKE@Q} == "true",
        "story_template": ${EX_STORY_TEMPLATE@Q} == "true",
        "harness_backlog": ${EX_HARNESS_BACKLOG@Q} == "true",
        "agents_md": ${EX_AGENTS_MD@Q} == "true",
        "evidence_dir": ${EX_EVIDENCE_DIR@Q} == "true",
    },
}
print(json.dumps(out, indent=2))
PYEOF
