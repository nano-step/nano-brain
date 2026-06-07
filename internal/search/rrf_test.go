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

func TestRRFMerge_StableTiebreaker(t *testing.T) {
	// Two chunks with mathematically identical RRF scores: both at rank 0 in their respective lists.
	// With k=60, score = 1.0 / (60 + 0 + 1) = 1/61 ≈ 0.016393...
	// Chunk A: "uuid-a-zzz" (sorts first lexicographically)
	// Chunk B: "uuid-b-aaa" (sorts second lexicographically)
	// Expected: tied scores broken by ID, so "uuid-a-zzz" should be first.
	bm25 := []Result{{ID: "uuid-b-aaa"}}
	vec := []Result{{ID: "uuid-a-zzz"}}
	k := 60.0

	got := RRFMerge(bm25, vec, k)
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}

	expectedScore := 1.0 / (k + 0 + 1)
	if !approxEqual(got[0].Score, expectedScore, 1e-9) {
		t.Errorf("first result score = %f, want %f", got[0].Score, expectedScore)
	}
	if !approxEqual(got[1].Score, expectedScore, 1e-9) {
		t.Errorf("second result score = %f, want %f", got[1].Score, expectedScore)
	}

	// Tiebreaker: smaller ID wins
	if got[0].ID != "uuid-a-zzz" {
		t.Errorf("tiebreaker: expected first ID='uuid-a-zzz', got '%s'", got[0].ID)
	}
	if got[1].ID != "uuid-b-aaa" {
		t.Errorf("tiebreaker: expected second ID='uuid-b-aaa', got '%s'", got[1].ID)
	}

	// Determinism check: run 10 times with same input, verify identical order
	for iter := 0; iter < 10; iter++ {
		result := RRFMerge(bm25, vec, k)
		if len(result) != 2 {
			t.Fatalf("iteration %d: expected 2 results, got %d", iter, len(result))
		}
		if result[0].ID != "uuid-a-zzz" || result[1].ID != "uuid-b-aaa" {
			t.Errorf("iteration %d: order changed; got %s, %s", iter, result[0].ID, result[1].ID)
		}
	}
}

func TestRRFMerge_PaginationConsistency(t *testing.T) {
	// Build a list with 12 results, 3 of which have tied scores (to test tiebreaker across pagination boundary).
	// Input: mix of scores such that we can slice [0:5] and [5:10] consistently.
	bm25 := []Result{
		{ID: "chunk-score-1-rank0"},
		{ID: "chunk-score-1-rank1"},
		{ID: "chunk-score-2-rank0"},
		{ID: "chunk-score-2-rank1"},
		{ID: "chunk-score-3-rank0"},
		{ID: "chunk-score-3-rank1"},
	}
	vec := []Result{
		{ID: "chunk-score-4-rank0"},
		{ID: "chunk-score-4-rank1"},
		{ID: "chunk-score-tied-vec0"}, // Will have tied score with some bm25
		{ID: "chunk-score-tied-vec1"},
		{ID: "chunk-score-tied-vec2"},
	}
	k := 60.0

	// Merge once, get full sorted result
	full := RRFMerge(bm25, vec, k)

	// Slice as if paginating: page 1 = [0:5], page 2 = [5:10]
	page1 := full[0:5]
	page2 := full[5:10]

	// Collect all IDs from both pages
	pageIDs := make(map[string]int)
	for i, r := range page1 {
		pageIDs[r.ID] = i
	}
	for i, r := range page2 {
		if _, exists := pageIDs[r.ID]; exists {
			t.Errorf("duplicate result across pages: %s", r.ID)
		}
		pageIDs[r.ID] = 5 + i
	}

	// Verify union of pages == full result (no skips, no duplicates)
	if len(pageIDs) != 10 {
		t.Errorf("pagination union: expected 10 unique results, got %d", len(pageIDs))
	}

	// Re-merge same input and verify page slices are identical
	full2 := RRFMerge(bm25, vec, k)
	for i := 0; i < len(full); i++ {
		if full[i].ID != full2[i].ID || !approxEqual(full[i].Score, full2[i].Score, 1e-12) {
			t.Errorf("re-merge changed result at index %d: %s vs %s", i, full[i].ID, full2[i].ID)
		}
	}
}

func approxEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestComputeRRFK_HighOverlap(t *testing.T) {
	// High overlap -> should return k smaller than baseK
	bm25 := []Result{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "e"}}
	vector := []Result{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "f"}, {ID: "g"}}
	baseK := 60.0

	k := ComputeRRFK(bm25, vector, baseK)
	if k >= baseK {
		t.Errorf("High overlap should produce k < baseK (%.2f), got %.2f", baseK, k)
	}
	if k < baseK*0.5 {
		t.Errorf("k should be >= baseK*0.5 (%.2f), got %.2f", baseK*0.5, k)
	}
}

func TestComputeRRFK_LowOverlap(t *testing.T) {
	// Low overlap -> should return k larger than baseK
	bm25 := []Result{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "e"}}
	vector := []Result{{ID: "f"}, {ID: "g"}, {ID: "h"}, {ID: "i"}, {ID: "j"}}
	baseK := 60.0

	k := ComputeRRFK(bm25, vector, baseK)
	if k <= baseK {
		t.Errorf("Low overlap should produce k > baseK (%.2f), got %.2f", baseK, k)
	}
	if k > baseK*2.0 {
		t.Errorf("k should be <= baseK*2.0 (%.2f), got %.2f", baseK*2.0, k)
	}
}

func TestComputeRRFK_SmallLists(t *testing.T) {
	// Lists too small -> should return baseK
	bm25 := []Result{{ID: "a"}, {ID: "b"}}
	vector := []Result{{ID: "c"}, {ID: "d"}}
	baseK := 60.0

	k := ComputeRRFK(bm25, vector, baseK)
	if !approxEqual(k, baseK, 1e-9) {
		t.Errorf("Small lists should return baseK (%.2f), got %.2f", baseK, k)
	}
}

func TestComputeRRFK_EmptyVector(t *testing.T) {
	// Empty vector list -> should return baseK
	bm25 := []Result{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "e"}}
	vector := []Result{}
	baseK := 60.0

	k := ComputeRRFK(bm25, vector, baseK)
	if !approxEqual(k, baseK, 1e-9) {
		t.Errorf("Empty vector list should return baseK (%.2f), got %.2f", baseK, k)
	}
}

func TestComputeRRFK_EmptyBM25(t *testing.T) {
	// Empty BM25 list -> should return baseK
	bm25 := []Result{}
	vector := []Result{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "e"}}
	baseK := 60.0

	k := ComputeRRFK(bm25, vector, baseK)
	if !approxEqual(k, baseK, 1e-9) {
		t.Errorf("Empty BM25 list should return baseK (%.2f), got %.2f", baseK, k)
	}
}

func TestComputeRRFK_BothEmpty(t *testing.T) {
	// Both lists empty -> should return baseK
	bm25 := []Result{}
	vector := []Result{}
	baseK := 60.0

	k := ComputeRRFK(bm25, vector, baseK)
	if !approxEqual(k, baseK, 1e-9) {
		t.Errorf("Both empty lists should return baseK (%.2f), got %.2f", baseK, k)
	}
}

func TestDynamicRRFMerge_Basic(t *testing.T) {
	bm25 := []Result{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	vector := []Result{{ID: "d"}, {ID: "e"}, {ID: "f"}}
	baseK := 60.0

	results := DynamicRRFMerge(bm25, vector, baseK)
	if len(results) != 6 {
		t.Errorf("Expected 6 results, got %d", len(results))
	}

	for i := 0; i < len(results)-1; i++ {
		if results[i].Score < results[i+1].Score {
			t.Error("Results not sorted descending")
		}
	}
}
