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
