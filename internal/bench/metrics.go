package bench

import (
	"math"
	"sort"
)

type QueryResult struct {
	Query          string
	RelevantDocIDs []string
	ReturnedDocIDs []string
	LatencyMs      float64
}

func PrecisionAtK(results []QueryResult, k int) float64 {
	if len(results) == 0 || k <= 0 {
		return 0
	}
	var sum float64
	for _, r := range results {
		relevant := toSet(r.RelevantDocIDs)
		top := r.ReturnedDocIDs
		if len(top) > k {
			top = top[:k]
		}
		var hits int
		for _, id := range top {
			if relevant[id] {
				hits++
			}
		}
		sum += float64(hits) / float64(k)
	}
	return sum / float64(len(results))
}

func RecallAtK(results []QueryResult, k int) float64 {
	if len(results) == 0 || k <= 0 {
		return 0
	}
	var sum float64
	for _, r := range results {
		if len(r.RelevantDocIDs) == 0 {
			continue
		}
		relevant := toSet(r.RelevantDocIDs)
		top := r.ReturnedDocIDs
		if len(top) > k {
			top = top[:k]
		}
		var hits int
		for _, id := range top {
			if relevant[id] {
				hits++
			}
		}
		sum += float64(hits) / float64(len(r.RelevantDocIDs))
	}
	return sum / float64(len(results))
}

func MeanReciprocalRank(results []QueryResult) float64 {
	if len(results) == 0 {
		return 0
	}
	var sum float64
	for _, r := range results {
		relevant := toSet(r.RelevantDocIDs)
		for rank, id := range r.ReturnedDocIDs {
			if relevant[id] {
				sum += 1.0 / float64(rank+1)
				break
			}
		}
	}
	return sum / float64(len(results))
}

// Percentile computes the p-th percentile (0-100) using nearest-rank method.
func Percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	idx := int(math.Ceil(p / 100 * float64(len(sorted))))
	if idx < 1 {
		idx = 1
	}
	if idx > len(sorted) {
		idx = len(sorted)
	}
	return sorted[idx-1]
}

func toSet(ids []string) map[string]bool {
	s := make(map[string]bool, len(ids))
	for _, id := range ids {
		s[id] = true
	}
	return s
}
