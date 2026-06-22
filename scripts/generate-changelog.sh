#!/usr/bin/env bash
# Auto-generate changelog.html from git log with conventional commits.
# Usage: bash scripts/generate-changelog.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
OUTPUT="$ROOT_DIR/changelog.html"

# Collect commits by category
declare -A FEAT FIX DOCS BENCH OTHER

while IFS='|' read -r hash subject date; do
  # Parse conventional commit prefix
  prefix="${subject%%:*}"
  body="${subject#*: }"
  
  case "$prefix" in
    feat) FEAT["$hash|$body|$date"]=1 ;;
    fix) FIX["$hash|$body|$date"]=1 ;;
    docs|bench) DOCS["$hash|$body|$date"]=1 ;;
    *) OTHER["$hash|$body|$date"]=1 ;;
  esac
done < <(git -C "$ROOT_DIR" log --format='%h|%s|%ad' --date=short -100 --no-merges)

# Build HTML
cat > "$OUTPUT" << 'HEADER'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>nano-brain — Changelog</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
    <style>
        :root{--bg:#09090b;--surface:#18181b;--border:#3f3f46;--text:#fafafa;--text-2:#a1a1aa;--accent:#22c55e;--orange:#f97316}
        *{margin:0;padding:0;box-sizing:border-box}
        body{font-family:'Inter',sans-serif;background:var(--bg);color:var(--text);padding:120px 24px 80px}
        .container{max-width:800px;margin:0 auto}
        h1{font-size:40px;font-weight:700;letter-spacing:-1px;margin-bottom:8px}
        .subtitle{color:var(--text-2);margin-bottom:48px}
        .category{margin-bottom:48px}
        .category h2{font-size:20px;font-weight:600;margin-bottom:16px;display:flex;align-items:center;gap:8px}
        .category h2 .dot{width:10px;height:10px;border-radius:50%;display:inline-block}
        .dot-feat{background:var(--accent)}
        .dot-fix{background:var(--orange)}
        .dot-docs{background:#a1a1aa}
        .dot-other{background:#71717a}
        .commit{padding:16px 0;border-bottom:1px solid var(--border)}
        .commit:last-child{border-bottom:none}
        .commit-hash{font-family:'JetBrains Mono',monospace;font-size:13px;color:var(--accent);margin-bottom:4px}
        .commit-msg{font-size:15px;line-height:1.5}
        .commit-date{font-size:12px;color:var(--text-2);margin-top:4px}
        .empty{color:var(--text-2);font-style:italic}
    </style>
</head>
<body>
<div class="container">
    <h1>Changelog</h1>
    <p class="subtitle">Auto-generated from git commits.</p>
HEADER

# Write categories
write_category() {
  local title="$1" dot_class="$2"
  shift 2
  local items=("$@")
  
  if [ ${#items[@]} -eq 0 ]; then
    return
  fi
  
  echo "<div class=\"category\">" >> "$OUTPUT"
  echo "  <h2><span class=\"dot $dot_class\"></span> $title</h2>" >> "$OUTPUT"
  
  for item in "${items[@]}"; do
    IFS='|' read -r hash body date <<< "$item"
    echo "  <div class=\"commit\">" >> "$OUTPUT"
    echo "    <div class=\"commit-hash\">$hash</div>" >> "$OUTPUT"
    echo "    <div class=\"commit-msg\">$body</div>" >> "$OUTPUT"
    echo "    <div class=\"commit-date\">$date</div>" >> "$OUTPUT"
    echo "  </div>" >> "$OUTPUT"
  done
  
  echo "</div>" >> "$OUTPUT"
}

# Collect arrays
FEAT_ITEMS=()
FIX_ITEMS=()
DOCS_ITEMS=()
OTHER_ITEMS=()

for key in "${!FEAT[@]}"; do FEAT_ITEMS+=("$key"); done
for key in "${!FIX[@]}"; do FIX_ITEMS+=("$key"); done
for key in "${!DOCS[@]}"; do DOCS_ITEMS+=("$key"); done
for key in "${!OTHER[@]}"; do OTHER_ITEMS+=("$key"); done

write_category "Features" "dot-feat" "${FEAT_ITEMS[@]}"
write_category "Bug Fixes" "dot-fix" "${FIX_ITEMS[@]}"
write_category "Documentation & Benchmarks" "dot-docs" "${DOCS_ITEMS[@]}"
write_category "Other" "dot-other" "${OTHER_ITEMS[@]}"

cat >> "$OUTPUT" << 'FOOTER'
</div>
</body>
</html>
FOOTER

echo "Generated: $OUTPUT"
