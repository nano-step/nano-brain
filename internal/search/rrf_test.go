package search

import (
	"math"
	"testing"
)

func TestRRFMerge_BasicFusion(t *testing.T) {
	bm25 := []Result{{ID: "a"}, {ID: "b"}}
	vec := []Result{{ID: "c"}, {ID: "d"}}
	k := 60.0

	got := RRFMerge(bm25, vec, k)
	if len(got) != 4 {
		t.Fatalf("expected 4 results, got %d", len(got))
	}

	wantFirst := 1.0 / (k + 0 + 1)
	if !approxEqual(got[0].Score, wantFirst, 1e-9) {
		t.Errorf("rank-0 score = %f, want %f", got[0].Score, wantFirst)
	}
	if got[0].Score < got[1].Score {
		t.Error("results not sorted descending")
	}
}

func TestRRFMerge_Deduplication(t *testing.T) {
	bm25 := []Result{{ID: "dup", Title: "bm25-title"}}
	vec := []Result{{ID: "dup", Title: "vec-title"}}
	k := 60.0

	got := RRFMerge(bm25, vec, k)
	if len(got) != 1 {
		t.Fatalf("expected 1 deduplicated result, got %d", len(got))
	}

	wantScore := 2 * (1.0 / (k + 0 + 1))
	if !approxEqual(got[0].Score, wantScore, 1e-9) {
		t.Errorf("dedup score = %f, want %f (sum of both)", got[0].Score, wantScore)
	}
}

func TestRRFMerge_SingleList(t *testing.T) {
	bm25 := []Result{{ID: "a"}, {ID: "b"}}
	got := RRFMerge(bm25, nil, 60)
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
}

func TestRRFMerge_EmptyBoth(t *testing.T) {
	got := RRFMerge(nil, nil, 60)
	if len(got) != 0 {
		t.Fatalf("expected 0 results, got %d", len(got))
	}
}

func TestRRFMerge_CustomK(t *testing.T) {
	list := []Result{{ID: "x"}}
	k30 := RRFMerge(list, nil, 30)
	k60 := RRFMerge(list, nil, 60)

	if approxEqual(k30[0].Score, k60[0].Score, 1e-12) {
		t.Error("k=30 and k=60 should produce different scores")
	}
	if k30[0].Score <= k60[0].Score {
		t.Error("smaller k should produce higher scores")
	}
}

func approxEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}
