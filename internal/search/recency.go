package search

import (
	"math"
	"sort"
	"time"
)

// ApplyRecencyBoost blends RRF scores with an exponential-decay recency signal.
// Scores are normalized to [0,1] before blending so recency doesn't dominate.
// final = (1 - weight) * normalized_score + weight * exp(-ln2 * ageDays / halfLife)
func ApplyRecencyBoost(results []Result, weight float64, halfLifeDays int, now time.Time) []Result {
	if len(results) == 0 || weight <= 0 || halfLifeDays <= 0 {
		return results
	}

	maxScore := 0.0
	for _, r := range results {
		if r.Score > maxScore {
			maxScore = r.Score
		}
	}

	halfLife := float64(halfLifeDays)
	boosted := make([]Result, len(results))

	for i, r := range results {
		boosted[i] = r

		normalized := 0.0
		if maxScore > 0 {
			normalized = r.Score / maxScore
		}

		ageDays := now.Sub(r.UpdatedAt).Hours() / 24
		if ageDays < 0 {
			ageDays = 0
		}
		multiplier := math.Exp(-math.Ln2 * ageDays / halfLife)

		boosted[i].Score = (1-weight)*normalized + weight*multiplier
	}

	sort.Slice(boosted, func(i, j int) bool {
		if boosted[i].Score != boosted[j].Score {
			return boosted[i].Score > boosted[j].Score
		}
		// Stable tiebreaker for deterministic cursor pagination (#358).
		return boosted[i].ID < boosted[j].ID
	})

	return boosted
}
