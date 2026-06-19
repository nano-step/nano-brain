## 1. Redis Method Detection

- [x] 1.1 Add `jsRedisMethods` map to `js_integration_extractor.go` with Redis-specific methods: `get`, `set`, `setEx`, `del`, `expire`, `ttl`, `publish`, `subscribe`, `lpush`, `rpush`, `sadd`, `srem`, `incr`, `decr`, `hget`, `hset`
- [x] 1.2 Add `jsRedisReadMethods`, `jsRedisWriteMethods`, `jsRedisDeleteMethods`, `jsRedisPubSubMethods` sub-maps for metadata classification
- [x] 1.3 Add `handleRedisCall` method that emits Redis integration edges with correct metadata (`cache_read`, `cache_write`, `cache_delete`, `cache_pubsub`, `cache_lock`)

## 2. Integration with Existing Extractor

- [x] 2.1 Modify `handleMemberExpression` to check Redis receiver patterns BEFORE HTTP method names (fix `redis.get()` misclassification)
- [ ] 2.2 Add `handleIdentifier` support for bare `get("key")` → detect if receiver is Redis-typed (via variable name matching)
- [x] 2.3 Ensure Redis handler returns early to prevent HTTP handler from also matching

## 3. Python Redis Detection

- [ ] 3.1 Check if `PythonIntegrationExtractor` exists and if Python Redis usage is present in any workspace
- [ ] 3.2 If yes, add Redis detection to `python_integration_extractor.go` with same metadata pattern

## 4. Testing & Verification

- [x] 4.1 Add unit tests for Redis edge extraction (get, set, setEx, del, publish) in `js_integration_extractor_test.go`
- [x] 4.2 Add test for `redis.get()` is NOT classified as HTTP GET
- [x] 4.3 Run `go test -race -short ./...`
- [ ] 4.4 Index zengamingx workspace and verify Redis edges appear in flow diagrams
- [ ] 4.5 Run `go test -race -tags=integration ./...`
