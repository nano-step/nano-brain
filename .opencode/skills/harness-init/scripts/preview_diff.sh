#!/usr/bin/env bash
# preview_diff.sh — render one template and diff against existing target file.
# Usage: preview_diff.sh --template <name> --target <path> --config <answers.yaml>
# Output: unified diff to stdout. Empty if no existing target. Non-zero if differs.

set -uo pipefail

SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TEMPLATE=""
TARGET=""
CONFIG=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --template) TEMPLATE="$2"; shift 2 ;;
    --target)   TARGET="$2";   shift 2 ;;
    --config)   CONFIG="$2";   shift 2 ;;
    -h|--help)
      grep '^#' "$0" | head -4
      exit 0 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$TEMPLATE" || -z "$TARGET" || -z "$CONFIG" ]]; then
  echo "Usage: preview_diff.sh --template <name> --target <path> --config <answers.yaml>" >&2
  exit 1
fi

TMPL_PATH="$SKILL_DIR/templates/$TEMPLATE"
if [[ ! -f "$TMPL_PATH" ]]; then
  echo "Template not found: $TMPL_PATH" >&2
  exit 1
fi

if [[ ! -f "$CONFIG" ]]; then
  echo "Config not found: $CONFIG" >&2
  exit 1
fi

RENDERED="$(mktemp -t harness-render-XXXXXX)"
trap 'rm -f "$RENDERED"' EXIT

python3 - "$TMPL_PATH" "$CONFIG" "$RENDERED" <<'PYEOF'
import sys, importlib.util
tmpl_path, cfg_path, out_path = sys.argv[1], sys.argv[2], sys.argv[3]

spec_path = __file__
import os
script_dir = os.path.dirname(os.path.abspath(spec_path)) if "__file__" in dir() else "."
install_py = os.path.join(os.path.dirname(tmpl_path).replace("/templates", "/scripts"), "install.py")

spec = importlib.util.spec_from_file_location("install_mod", install_py)
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)

from pathlib import Path
cfg = mod.parse_yaml(Path(cfg_path))
vars = mod.build_vars(cfg)
with open(tmpl_path) as f:
    tmpl = f.read()
rendered = mod.substitute(tmpl, vars)
with open(out_path, "w") as f:
    f.write(rendered)
PYEOF

if [[ ! -f "$TARGET" ]]; then
  echo "[preview_diff] target does not exist — would be a fresh create."
  echo "Rendered size: $(wc -l < "$RENDERED") lines"
  exit 0
fi

if diff -q "$TARGET" "$RENDERED" >/dev/null 2>&1; then
  echo "[preview_diff] no changes — rendered output matches target."
  exit 0
fi

echo "=== diff $TARGET <-> rendered($TEMPLATE) ==="
diff -u "$TARGET" "$RENDERED" || true
