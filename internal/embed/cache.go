package embed

import (
	"container/list"
	"sync"
)

// DefaultQueryCacheSize bounds the query-embedding cache (see QueryCache).
// Agents commonly re-issue the same memory_query text (retries, cursor
// pagination, mode=debugging fan-out), so a modest working set avoids
// unbounded memory growth on long-lived server processes while still
// eliminating the redundant embedding-provider round-trip for repeats.
const DefaultQueryCacheSize = 512

// QueryCache is a bounded, thread-safe LRU cache for query embeddings.
//
// It exists solely to avoid a redundant embedding-provider (Ollama/VoyageAI)
// round-trip when the same query text is embedded more than once on the
// hybrid search hot path (see issue #539 Finding 4). It must NOT be used to
// cache document/chunk embeddings — those are already persisted in the
// database via internal/embed/queue.go, and caching them here would just
// duplicate that storage in memory.
type QueryCache struct {
	mu       sync.Mutex
	capacity int
	ll       *list.List
	items    map[string]*list.Element
}

type queryCacheEntry struct {
	key   string
	value []float32
}

// NewQueryCache creates a QueryCache bounded to capacity entries. A
// non-positive capacity falls back to DefaultQueryCacheSize.
func NewQueryCache(capacity int) *QueryCache {
	if capacity <= 0 {
		capacity = DefaultQueryCacheSize
	}
	return &QueryCache{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[string]*list.Element),
	}
}

// Get returns the cached embedding for key, if present, promoting the entry
// to most-recently-used.
func (c *QueryCache) Get(key string) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*queryCacheEntry).value, true
}

// Put stores value under key, evicting the least-recently-used entry if the
// cache is at capacity.
func (c *QueryCache) Put(key string, value []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		el.Value.(*queryCacheEntry).value = value
		c.ll.MoveToFront(el)
		return
	}
	el := c.ll.PushFront(&queryCacheEntry{key: key, value: value})
	c.items[key] = el
	if c.ll.Len() > c.capacity {
		oldest := c.ll.Back()
		if oldest != nil {
			c.ll.Remove(oldest)
			delete(c.items, oldest.Value.(*queryCacheEntry).key)
		}
	}
}

// Len returns the current number of cached entries (for tests/diagnostics).
func (c *QueryCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}
