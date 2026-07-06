package embed

import "testing"

func TestQueryCache_GetMiss(t *testing.T) {
	c := NewQueryCache(4)
	if _, ok := c.Get("nope"); ok {
		t.Error("expected miss on empty cache")
	}
}

func TestQueryCache_PutThenGet(t *testing.T) {
	c := NewQueryCache(4)
	want := []float32{0.1, 0.2, 0.3}
	c.Put("q", want)

	got, ok := c.Get("q")
	if !ok {
		t.Fatal("expected hit after Put")
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %f, want %f", i, got[i], want[i])
		}
	}
}

func TestQueryCache_EvictsLeastRecentlyUsed(t *testing.T) {
	c := NewQueryCache(2)
	c.Put("a", []float32{1})
	c.Put("b", []float32{2})
	c.Put("c", []float32{3}) // evicts "a" (least recently used)

	if _, ok := c.Get("a"); ok {
		t.Error("expected \"a\" to be evicted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Error("expected \"b\" to still be cached")
	}
	if _, ok := c.Get("c"); !ok {
		t.Error("expected \"c\" to still be cached")
	}
	if got := c.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
}

func TestQueryCache_GetPromotesToMostRecentlyUsed(t *testing.T) {
	c := NewQueryCache(2)
	c.Put("a", []float32{1})
	c.Put("b", []float32{2})
	c.Get("a")               // promote "a"
	c.Put("c", []float32{3}) // should evict "b", not "a"

	if _, ok := c.Get("a"); !ok {
		t.Error("expected \"a\" to survive eviction after being promoted")
	}
	if _, ok := c.Get("b"); ok {
		t.Error("expected \"b\" to be evicted")
	}
}

func TestQueryCache_PutOverwritesExistingKey(t *testing.T) {
	c := NewQueryCache(4)
	c.Put("q", []float32{1})
	c.Put("q", []float32{2})

	got, ok := c.Get("q")
	if !ok {
		t.Fatal("expected hit")
	}
	if len(got) != 1 || got[0] != 2 {
		t.Errorf("got = %v, want [2]", got)
	}
	if got := c.Len(); got != 1 {
		t.Errorf("Len() = %d, want 1 (overwrite should not grow the cache)", got)
	}
}

func TestNewQueryCache_NonPositiveCapacityUsesDefault(t *testing.T) {
	c := NewQueryCache(0)
	if c.capacity != DefaultQueryCacheSize {
		t.Errorf("capacity = %d, want default %d", c.capacity, DefaultQueryCacheSize)
	}
}
