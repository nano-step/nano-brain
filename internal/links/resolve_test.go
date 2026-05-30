package links

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

type fakeResolverQueries struct {
	existsCalls    atomic.Int64
	titleCalls     atomic.Int64
	existsResults  map[string]bool
	titleResults   map[string][]uuid.UUID
}

func (f *fakeResolverQueries) ExistsDocByID(_ context.Context, arg ExistsDocByIDParams) (bool, error) {
	f.existsCalls.Add(1)
	key := arg.WorkspaceHash + ":" + arg.ID.String()
	return f.existsResults[key], nil
}

func (f *fakeResolverQueries) ListDocIDsByTitle(_ context.Context, arg ListDocIDsByTitleParams) ([]uuid.UUID, error) {
	f.titleCalls.Add(1)
	key := arg.WorkspaceHash + ":" + arg.Lower
	return f.titleResults[key], nil
}

func TestResolver(t *testing.T) {
	ctx := context.Background()
	ws := "ws1"
	idA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	idB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	t.Run("resolve_id_hit", func(t *testing.T) {
		fq := &fakeResolverQueries{
			existsResults: map[string]bool{ws + ":" + idA.String(): true},
		}
		r := NewResolver(fq)
		ok, err := r.ResolveID(ctx, ws, idA)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("expected true")
		}
		if fq.existsCalls.Load() != 1 {
			t.Errorf("existsCalls = %d, want 1", fq.existsCalls.Load())
		}
	})

	t.Run("resolve_id_miss", func(t *testing.T) {
		fq := &fakeResolverQueries{
			existsResults: map[string]bool{},
		}
		r := NewResolver(fq)
		ok, err := r.ResolveID(ctx, ws, idA)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Error("expected false")
		}
		if fq.existsCalls.Load() != 1 {
			t.Errorf("existsCalls = %d, want 1", fq.existsCalls.Load())
		}
	})

	t.Run("resolve_title_case_insensitive", func(t *testing.T) {
		fq := &fakeResolverQueries{
			titleResults: map[string][]uuid.UUID{ws + ":Architecture Overview": {idA}},
		}
		r := NewResolver(fq)
		// First call with original casing — hits the fake, populates cache.
		ids, err := r.ResolveTitle(ctx, ws, "Architecture Overview")
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) != 1 || ids[0] != idA {
			t.Errorf("ids = %v, want [%s]", ids, idA)
		}
		// Second call with DIFFERENT casing — should hit cache (lowered key matches).
		ids2, err := r.ResolveTitle(ctx, ws, "ARCHITECTURE overview")
		if err != nil {
			t.Fatal(err)
		}
		if len(ids2) != 1 || ids2[0] != idA {
			t.Errorf("ids2 = %v, want [%s]", ids2, idA)
		}
		if fq.titleCalls.Load() != 1 {
			t.Errorf("titleCalls = %d, want 1 (different casing should hit cache)", fq.titleCalls.Load())
		}
	})

	t.Run("resolve_title_ambiguous", func(t *testing.T) {
		fq := &fakeResolverQueries{
			titleResults: map[string][]uuid.UUID{ws + ":Foo": {idA, idB}},
		}
		r := NewResolver(fq)
		ids, err := r.ResolveTitle(ctx, ws, "Foo")
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) != 2 {
			t.Errorf("len(ids) = %d, want 2", len(ids))
		}
	})

	t.Run("cache_hit_no_extra_query", func(t *testing.T) {
		fq := &fakeResolverQueries{
			titleResults: map[string][]uuid.UUID{ws + ":bar": {idA}},
		}
		r := NewResolver(fq)
		_, _ = r.ResolveTitle(ctx, ws, "Bar")
		_, _ = r.ResolveTitle(ctx, ws, "Bar")
		if fq.titleCalls.Load() != 1 {
			t.Errorf("titleCalls = %d, want 1 (second call should be cached)", fq.titleCalls.Load())
		}
	})

	t.Run("flush_clears_cache", func(t *testing.T) {
		fq := &fakeResolverQueries{
			titleResults: map[string][]uuid.UUID{ws + ":baz": {idA}},
		}
		r := NewResolver(fq)
		_, _ = r.ResolveTitle(ctx, ws, "Baz")
		r.FlushWorkspace(ws)
		_, _ = r.ResolveTitle(ctx, ws, "Baz")
		if fq.titleCalls.Load() != 2 {
			t.Errorf("titleCalls = %d, want 2 (cache should be flushed)", fq.titleCalls.Load())
		}
	})
}
