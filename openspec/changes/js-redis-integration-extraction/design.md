## Context

The `JSIntegrationExtractor` in `internal/graph/js_integration_extractor.go` handles member expressions via `handleMemberExpression`. It currently checks for:
- HTTP method names (`get`, `post`, `put`, `delete`) → HTTP integration edge
- Publish methods (`publish`, `send`, `emit`) → queue integration edge (but skips when receiver is "redis")
- Consume methods (`subscribe`, `consume`, `listen`, `on`) → queue consumer edge

Redis calls like `redis.get(key)` hit the HTTP method check (`get`) and get classified as HTTP calls — which is incorrect. `redis.set(key, val)` doesn't match any pattern and is silently dropped.

The express-app codebase uses Redis extensively: cache reads/writes, distributed locks, session storage, pub/sub channels.

## Goals / Non-Goals

**Goals:**
- Detect Redis operations from `JSIntegrationExtractor`
- Classify Redis operations as `cache_read`, `cache_write`, `cache_delete`, `cache_lock`, `cache_pubsub`
- Fix the existing bug where `redis.get()` is misclassified as HTTP GET
- Apply same detection to `PythonIntegrationExtractor`

**Non-Goals:**
- Detecting Redis Cluster/Sentinel patterns
- Parsing Redis command arguments for cache key names
- Adding Redis to the Go integration extractor (Go project doesn't use Redis)

## Decisions

### Decision 1: Add Redis method map before HTTP check

**Choice**: Check Redis receiver patterns BEFORE HTTP method names in `handleMemberExpression`.

**Rationale**: The current code matches `redis.get()` as HTTP GET. By checking Redis patterns first, we fix this misclassification. Redis detection is receiver-aware (`redis.get()` → cache, `axios.get()` → HTTP).

**Alternative considered**: Add a special case in HTTP handler — rejected because it's fragile and doesn't scale to other libraries.

### Decision 2: Redis-specific edge metadata

**Choice**: Use metadata `kind: "cache_read"` / `cache_write` / `cache_delete` / `cache_lock` / `cache_pubsub` with `receiver: "redis"`.

**Rationale**: Distinguishes Redis operations from HTTP/queue. Enables future filtering and diagram rendering (e.g., show cache vs HTTP vs queue differently).

### Decision 3: Receiver detection

**Choice**: Match any member expression where the method name is in `jsRedisMethods`. The receiver name is extracted but not validated (any object calling `.get()` with Redis-like patterns is classified as Redis).

**Rationale**: Too restrictive (only `redis.get()`) misses `r.get()`, `cache.get()`, `client.get()`. The method name set is specific enough to avoid false positives.

## Risks / Trade-offs

- **[Risk] False positives** → Mitigation: Redis method names are distinctive (`setEx`, `hget`, `sadd`). Common JS methods like `get`/`set` are already in HTTP map — Redis check runs first but only when receiver is present.
- **[Risk] Overlapping HTTP/Redis for `.get()`** → Mitigation: Redis handler returns early, preventing HTTP handler from also matching.
