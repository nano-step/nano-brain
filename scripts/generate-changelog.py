#!/usr/bin/env python3
"""Parse CHANGELOG.md and generate changelog.html with release versions."""
import re, html, sys

def parse_changelog(md_path):
    with open(md_path) as f:
        text = f.read()

    releases = []
    current = None
    current_category = None

    for line in text.split('\n'):
        stripped = line.strip()

        m = re.match(r'^## \[(.+?)\](?:\s*—\s*(\S+))?', stripped)
        if m:
            if current:
                releases.append(current)
            version = m.group(1)
            date = m.group(2) or ''
            current = {'version': version, 'date': date, 'sections': []}
            current_category = None
            continue

        if current is None:
            continue

        m = re.match(r'^### (.+)', stripped)
        if m:
            current_category = {'title': m.group(1), 'items': []}
            current['sections'].append(current_category)
            continue

        if current_category and stripped.startswith('- '):
            item = stripped[2:]
            item = re.sub(r'\*\*(.+?)\*\*', r'<strong>\1</strong>', html.escape(item))
            item = re.sub(r'`(.+?)`', r'<code>\1</code>', item)
            item = re.sub(r'\(#(\d+)\)', r'<a href="https://github.com/nano-step/nano-brain/pull/\1">#\1</a>', item)
            current_category['items'].append(item)

    if current:
        releases.append(current)

    return releases

def generate_html(releases):
    category_colors = {
        'Added': '#22c55e',
        'Changed': '#f97316',
        'Fixed': '#ef4444',
        'Removed': '#a1a1aa',
        'Security': '#eab308',
        'Internal': '#71717a',
    }

    rows = []
    for r in releases:
        ver = html.escape(r['version'])
        date = html.escape(r['date'])
        badge_class = 'current' if ver == 'Unreleased' else 'release'

        sections_html = ''
        for s in r['sections']:
            color = category_colors.get(s['title'], '#a1a1aa')
            items = ''.join(f'<li>{item}</li>' for item in s['items'])
            sections_html += f'''
            <div class="change-section">
                <h3 style="color:{color}">{html.escape(s['title'])}</h3>
                <ul>{items}</ul>
            </div>'''

        rows.append(f'''
        <div class="release">
            <div class="release-header">
                <span class="version">{ver}</span>
                <span class="badge {badge_class}">{ver}</span>
                <span class="date">{date}</span>
            </div>
            {sections_html}
        </div>''')

    versions_list = ''.join(
        f'<a href="#{html.escape(r["version"])}">{html.escape(r["version"])}</a>'
        for r in releases[:15]
    )
    entries = '\n'.join(rows)

    return f'''<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>nano-brain — Changelog</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
<style>
:root{{--bg:#09090b;--surface:#18181b;--surface-2:#27272a;--border:#3f3f46;--text:#fafafa;--text-2:#a1a1aa;--accent:#22c55e}}
*{{margin:0;padding:0;box-sizing:border-box}}
body{{font-family:'Inter',sans-serif;background:var(--bg);color:var(--text);padding:100px 24px 80px}}
.container{{max-width:800px;margin:0 auto}}
h1{{font-size:40px;font-weight:700;letter-spacing:-1px;margin-bottom:8px}}
.subtitle{{color:var(--text-2);margin-bottom:32px}}
.toc{{display:flex;flex-wrap:wrap;gap:8px;margin-bottom:48px;padding:24px;background:var(--surface);border:1px solid var(--border);border-radius:12px}}
.toc a{{padding:6px 12px;background:var(--bg);border:1px solid var(--border);border-radius:6px;text-decoration:none;color:var(--text-2);font-family:'JetBrains Mono',monospace;font-size:12px;transition:all .2s}}
.toc a:hover{{border-color:var(--accent);color:var(--accent)}}
.release{{margin-bottom:48px;padding:32px;background:var(--surface);border:1px solid var(--border);border-radius:16px}}
.release-header{{display:flex;align-items:center;gap:12px;margin-bottom:24px;padding-bottom:16px;border-bottom:1px solid var(--border)}}
.version{{font-size:24px;font-weight:700;font-family:'JetBrains Mono',monospace}}
.badge{{padding:4px 12px;border-radius:100px;font-size:11px;font-weight:500}}
.badge.current{{background:rgba(34,197,94,.15);color:var(--accent)}}
.badge.release{{background:rgba(161,161,170,.15);color:var(--text-2)}}
.date{{font-size:14px;color:var(--text-2);margin-left:auto}}
.change-section{{margin-bottom:20px}}
.change-section:last-child{{margin-bottom:0}}
.change-section h3{{font-size:14px;font-weight:600;margin-bottom:8px}}
.change-section ul{{list-style:none}}
.change-section li{{padding:6px 0;font-size:14px;color:var(--text-2);line-height:1.6;border-bottom:1px solid rgba(63,63,70,.3)}}
.change-section li:last-child{{border-bottom:none}}
.change-section li strong{{color:var(--text);font-weight:500}}
.change-section li code{{font-family:'JetBrains Mono',monospace;font-size:12px;padding:2px 6px;background:var(--bg);border-radius:4px;color:var(--accent)}}
.change-section li a{{color:var(--accent);text-decoration:none}}
.change-section li a:hover{{text-decoration:underline}}
</style>
</head>
<body>
<div class="container">
<h1>Changelog</h1>
<p class="subtitle">All notable changes to nano-brain.</p>
<div class="toc">{versions_list}</div>
{entries}
</div>
</body>
</html>'''

if __name__ == '__main__':
    md = sys.argv[1] if len(sys.argv) > 1 else 'CHANGELOG.md'
    releases = parse_changelog(md)
    print(generate_html(releases))
