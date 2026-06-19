## ADDED Requirements

### Requirement: Detect Redis cache read operations

The `JSIntegrationExtractor` SHALL detect `redis.get(key)` style calls and emit an integration edge with metadata `kind: "cache_read"`.

#### Scenario: Redis get with string key
- **WHEN** source code contains `redis.get("mykey")`
- **THEN** the extractor SHALL emit an edge with target `Redis GET` and metadata `kind: "cache_read"`

#### Scenario: Redis get with template string key
- **WHEN** source code contains `` redis.get(`caskets730:${botId}`) ``
- **THEN** the extractor SHALL emit an edge with target `Redis GET` and metadata `kind: "cache_read"`, key `caskets730:${botId}`

### Requirement: Detect Redis cache write operations

The `JSIntegrationExtractor` SHALL detect `redis.set(key, val)`, `redis.setEx(key, ttl, val)` calls and emit integration edges with metadata `kind: "cache_write"`.

#### Scenario: Redis setEx with TTL
- **WHEN** source code contains `redis.setEx(key, ttl, JSON.stringify(data))`
- **THEN** the extractor SHALL emit an edge with target `Redis SETEX` and metadata `kind: "cache_write"`

#### Scenario: Redis set with NX option
- **WHEN** source code contains `redis.set("lockTrade:" + assetId, 1, { NX: true, EX: 900 })`
- **THEN** the extractor SHALL emit an edge with target `Redis SET` and metadata `kind: "cache_lock"`

### Requirement: Detect Redis delete operations

The `JSIntegrationExtractor` SHALL detect `redis.del(key)` and emit integration edges with metadata `kind: "cache_delete"`.

#### Scenario: Redis del
- **WHEN** source code contains `redis.del("lockTrade:" + assetId)`
- **THEN** the extractor SHALL emit an edge with target `Redis DEL` and metadata `kind: "cache_delete"`

### Requirement: Detect Redis pub/sub operations

The `JSIntegrationExtractor` SHALL detect `redis.publish(channel, msg)` and emit integration edges with metadata `kind: "cache_pubsub"`.

#### Scenario: Redis publish
- **WHEN** source code contains `redis.publish("trade-events", JSON.stringify(data))`
- **THEN** the extractor SHALL emit an edge with target `Redis PUBLISH` and metadata `kind: "cache_pubsub"`

### Requirement: Do not misclassify Redis as HTTP

The `JSIntegrationExtractor` SHALL NOT classify `redis.get()` as an HTTP GET request. Redis method detection SHALL run before HTTP method detection.

#### Scenario: redis.get is not HTTP
- **WHEN** source code contains `redis.get("key")`
- **THEN** the extractor SHALL emit a `cache_read` edge, NOT an HTTP GET edge
