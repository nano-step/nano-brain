# smoke:e2e Evidence — #397 Code Symbol Summarization (PR1)

## Test Environment
- Server: `./bin/nano-brain` built from feat/397-code-symbol-summarization
- Port: 3199
- Database: nanobrain_dev (host.docker.internal:5432)
- Config: code_summarization.enabled = false (default)

## Test 1: Server starts and health endpoint responds
```
curl -sf http://localhost:3199/health
{"status":"ok","ready":true,"version":"dev","uptime_s":2,"workspace_count":45}
```
**Result: PASS**

## Test 2: POST /api/v1/code/summarize returns 400 when feature disabled
```
curl -s -o /tmp/resp.json -w "%{http_code}" -X POST http://localhost:3199/api/v1/code/summarize \
  -H "Content-Type: application/json" \
  -d '{"workspace":"7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f"}'

HTTP 400
{"error":"http_error","message":"code summarization is disabled"}
```
**Result: PASS** — Correct status code and error message

## Test 3: Response format validation
```python
import json
d = json.load(open('/tmp/resp.json'))
assert 'error' in d  # PASS
```
**Result: PASS**

## Summary
| Test | Expected | Actual | Verdict |
|------|----------|--------|---------|
| Health endpoint | 200 + status:ok | 200 + status:ok | ✅ PASS |
| Disabled feature | 400 + error message | 400 + error message | ✅ PASS |
| Response format | Has error field | Has error field | ✅ PASS |
