## Why

The `JSIntegrationExtractor` does not detect Redis operations (`redis.get()`, `redis.set()`, `redis.del()`, `redis.setEx()`, `redis.publish()`, `redis.expire()`). These are heavily used throughout the express-app codebase (cache reads, trade locks, session storage, pub/sub) but never appear in flow diagrams or sequence diagrams.

Current detection only covers: HTTP calls (axios/fetch), queue publish (emit/send), queue consume (subscribe/on). Redis is the most critical missing integration — it's the cache layer, distributed lock mechanism, and pub/sub backbone.

## What Changes

- Add `jsRedisMethods` map to `JSIntegrationExtractor` covering: `get`, `set`, `setEx`, `del`, `expire`, `ttl`, `publish`, `subscribe`, `lpush`, `rpush`, `sadd`, `srem`, `incr`, `decr`, `hget`, `hset`
- Add Redis-specific edge metadata (`kind: "cache_read"`, `kind: "cache_write"`, `kind: "cache_delete"`, `kind: "cache_pubsub"`)
- Detect receiver pattern: `<redisInstance>.<method>(key)` → `Redis <method>`
- Add new edge kind `cache` (or reuse `integration`) with metadata distinguishing read/write/pubsub
- Apply same detection to `PythonIntegrationExtractor` if Python Redis usage exists

## Capabilities

### New Capabilities

- `redis-integration-extraction`: Detect and extract Redis operations from JS/TS/Python source code as graph edges

### Modified Capabilities

_(none — this is additive)_

## Impact

- **Files**: `internal/graph/js_integration_extractor.go` (add Redis method map + handler), possibly `internal/graph/python_integration_extractor.go`
- **API**: No contract change — new edges appear in flow/sequence diagrams automatically
- **Data**: Re-index required to populate Redis edges for existing workspaces
- **Breaking**: No — Redis detection is opt-in via new method map
