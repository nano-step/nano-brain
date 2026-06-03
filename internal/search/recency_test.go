package search

import (
	"math"
	"testing"
	"time"
)

func TestRecencyBoost_NewerHigher(t *testing.T) {
	now := time.Now()
	results := []Result{
		{ID: "old", Score: 0.5, UpdatedAt: now.Add(-200 * 24 * time.Hour)},
		{ID: "new", Score: 0.5, UpdatedAt: now.Add(-10 * 24 * time.Hour)},
	}

	boosted := ApplyRecencyBoost(results, 0.3, 180, now)
	if boosted[0].ID != "new" {
		t.Errorf("newer doc should rank first, got %s", boosted[0].ID)
	}
	if boosted[0].Score <= boosted[1].Score {
		t.Error("newer doc should have higher score")
	}
}

func TestRecencyBoost_ZeroWeight(t *testing.T) {
	now := time.Now()
	results := []Result{
		{ID: "a", Score: 0.8, UpdatedAt: now.Add(-100 * 24 * time.Hour)},
		{ID: "b", Score: 0.5, UpdatedAt: now},
	}

	boosted := ApplyRecencyBoost(results, 0, 180, now)
	if boosted[0].ID != "a" || !approxEqual(boosted[0].Score, 0.8, 1e-9) {
		t.Error("zero weight should preserve original scores and order")
	}
}

func TestRecencyBoost_TodayDoc(t *testing.T) {
	now := time.Now()
	results := []Result{
		{ID: "today", Score: 1.0, UpdatedAt: now},
	}

	boosted := ApplyRecencyBoost(results, 0.5, 180, now)
	want := 0.5*1.0 + 0.5*1.0 // multiplier = exp(0) = 1.0
	if !approxEqual(boosted[0].Score, want, 1e-9) {
		t.Errorf("today doc score = %f, want %f", boosted[0].Score, want)
	}
}

func TestRecencyBoost_Normalization(t *testing.T) {
	now := time.Now()
	results := []Result{
		{ID: "high", Score: 0.03, UpdatedAt: now.Add(-200 * 24 * time.Hour)},
		{ID: "low", Score: 0.01, UpdatedAt: now.Add(-1 * 24 * time.Hour)},
	}

	boosted := ApplyRecencyBoost(results, 0.3, 180, now)
	if boosted[0].ID != "high" {
		t.Errorf("higher-scoring doc should still rank first after normalization, got %s first", boosted[0].ID)
	}
	if boosted[0].Score <= 0 || boosted[0].Score > 1.0 {
		t.Errorf("boosted score should be in (0,1], got %f", boosted[0].Score)
	}
}

func TestRecencyBoost_ZeroHalfLife(t *testing.T) {
	now := time.Now()
	results := []Result{{ID: "1", Score: 0.5, UpdatedAt: now}}
	boosted := ApplyRecencyBoost(results, 0.3, 0, now)
	if !approxEqual(boosted[0].Score, 0.5, 1e-9) {
		t.Errorf("halfLifeDays=0 should return unchanged, got %f", boosted[0].Score)
	}
}

func TestRecencyBoost_VeryOldDoc(t *testing.T) {
	now := time.Now()
	results := []Result{
		{ID: "ancient", Score: 1.0, UpdatedAt: now.Add(-1000 * 24 * time.Hour)},
	}

	boosted := ApplyRecencyBoost(results, 0.5, 180, now)
	multiplier := math.Exp(-math.Ln2 * 1000 / 180)
	want := 0.5*1.0 + 0.5*multiplier
	if !approxEqual(boosted[0].Score, want, 1e-9) {
		t.Errorf("very old doc score = %f, want %f", boosted[0].Score, want)
	}
	if multiplier > 0.05 {
		t.Errorf("1000-day multiplier should be near zero, got %f", multiplier)
	}
}

func TestApplyRecencyBoost_StableTiebreaker(t *testing.T) {
	// Two results with identical Score and identical UpdatedAt timestamp.
	// After recency boost, both must have identical boosted scores (same pre-score, same age → same boost).
	// Tiebreaker: smaller ID wins.
	baseTime := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	results := []Result{
		{ID: "uuid-x-bbb", Score: 0.5, UpdatedAt: baseTime},
		{ID: "uuid-a-aaa", Score: 0.5, UpdatedAt: baseTime},
	}

	boosted := ApplyRecencyBoost(results, 0.3, 180, now)
	if len(boosted) != 2 {
		t.Fatalf("expected 2 results, got %d", len(boosted))
	}

	// Both must have identical scores (since pre-score and age are identical)
	if !approxEqual(boosted[0].Score, boosted[1].Score, 1e-9) {
		t.Errorf("tied results should have same boosted score: %f vs %f", boosted[0].Score, boosted[1].Score)
	}

	// Tiebreaker: smaller ID first
	if boosted[0].ID != "uuid-a-aaa" {
		t.Errorf("tiebreaker: expected first ID='uuid-a-aaa', got '%s'", boosted[0].ID)
	}
	if boosted[1].ID != "uuid-x-bbb" {
		t.Errorf("tiebreaker: expected second ID='uuid-x-bbb', got '%s'", boosted[1].ID)
	}

	// Determinism check: run twice, verify identical order
	for iter := 0; iter < 2; iter++ {
		result := ApplyRecencyBoost(results, 0.3, 180, now)
		if result[0].ID != "uuid-a-aaa" || result[1].ID != "uuid-x-bbb" {
			t.Errorf("iteration %d: order changed; got %s, %s", iter, result[0].ID, result[1].ID)
		}
	}
}
