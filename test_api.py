#!/usr/bin/env python3
"""
nano-brain HTTP API test suite
Tests all existing endpoints at http://localhost:3100
"""
import urllib.request
import json
import time
import sys

BASE = 'http://localhost:3100'
results = []

def req(name, method, path, data=None, timeout=15):
    t0 = time.time()
    try:
        if data:
            r = urllib.request.Request(
                BASE + path, method=method,
                data=json.dumps(data).encode(),
                headers={'Content-Type': 'application/json'}
            )
        else:
            r = urllib.request.Request(BASE + path, method=method)
        with urllib.request.urlopen(r, timeout=timeout) as resp:
            body = resp.read().decode()
            ms = round((time.time() - t0) * 1000)
            results.append(('PASS', ms, name, body[:120]))
            print(f'  PASS {ms:5d}ms  {name}')
            return True, ms, body
    except urllib.error.HTTPError as e:
        ms = round((time.time() - t0) * 1000)
        body = e.read().decode()[:120]
        results.append(('FAIL', ms, name, f'HTTP{e.code}: {body}'))
        print(f'  FAIL {ms:5d}ms  {name}  |  HTTP{e.code}: {body}')
        return False, ms, body
    except Exception as e:
        ms = round((time.time() - t0) * 1000)
        results.append(('FAIL', ms, name, str(e)[:120]))
        print(f'  FAIL {ms:5d}ms  {name}  |  {str(e)[:120]}')
        return False, ms, str(e)


print("=" * 65)
print("  nano-brain HTTP API Test Suite")
print(f"  Target: {BASE}")
print("=" * 65)

# ─── Write a doc first so we have a real docId for connections test ──
_ok, _ms, _body = req('POST /api/write (setup)', 'POST', '/api/write',
    {'content': 'Setup doc for connections test', 'tags': 'test'})
# Extract the path from write response and use it as docId
_written_path = ''
try:
    _written_path = json.loads(_body).get('path', '')
except Exception:
    pass

# ─── GET endpoints ────────────────────────────────────────────────
print("\n[GET endpoints]")
req('GET /health',                       'GET', '/health')
req('GET /api/status',                   'GET', '/api/status')
req('GET /api/v1/status',                'GET', '/api/v1/status')
req('GET /api/v1/workspaces',            'GET', '/api/v1/workspaces')
req('GET /api/v1/tags',                  'GET', '/api/v1/tags')
req('GET /api/v1/graph/entities',        'GET', '/api/v1/graph/entities')
req('GET /api/v1/graph/stats',           'GET', '/api/v1/graph/stats')
req('GET /api/v1/graph/symbols',         'GET', '/api/v1/graph/symbols')
req('GET /api/v1/graph/flows',           'GET', '/api/v1/graph/flows')
req('GET /api/v1/graph/connections',     'GET', '/api/v1/graph/connections')
req('GET /api/v1/graph/infrastructure',  'GET', '/api/v1/graph/infrastructure')
req('GET /api/v1/telemetry',             'GET', '/api/v1/telemetry')
# Use the written path as docId (URL-encoded); fallback to a dummy path
import urllib.parse
_doc_param = urllib.parse.quote(_written_path, safe='') if _written_path else 'nonexistent'
req('GET /api/v1/connections?docId=<path>', 'GET', f'/api/v1/connections?docId={_doc_param}')
req('GET /api/v1/search?q=test',         'GET', '/api/v1/search?q=test&limit=3')

# ─── POST endpoints ───────────────────────────────────────────────
print("\n[POST endpoints]")
req('POST /api/write',         'POST', '/api/write',
    {'content': 'Test memory entry for API validation', 'tags': 'test,api'})

req('POST /api/query (hybrid)', 'POST', '/api/query',
    {'query': 'test memory', 'limit': 3})

req('POST /api/search (FTS)',   'POST', '/api/search',
    {'query': 'test memory', 'limit': 3})

# vsearch needs embedding model; give it a longer timeout
req('POST /api/vsearch',        'POST', '/api/vsearch',
    {'query': 'test memory', 'limit': 3}, timeout=20)

# ─── Summary ──────────────────────────────────────────────────────
passed = [r for r in results if r[0] == 'PASS']
failed = [r for r in results if r[0] == 'FAIL']
total  = len(results)

print("\n" + "=" * 65)
print(f"  RESULTS: {len(passed)} PASS  |  {len(failed)} FAIL  |  {total} total")
print("=" * 65)

if failed:
    print("\n[Failed endpoints]")
    for r in failed:
        print(f"  {r[2]}  ->  {r[3]}")

slow = [r for r in passed if r[1] > 3000]
if slow:
    print("\n[Slow endpoints >3s — expected for vsearch if embedding model busy]")
    for r in slow:
        print(f"  {r[1]}ms  {r[2]}")

if failed:
    sys.exit(1)
