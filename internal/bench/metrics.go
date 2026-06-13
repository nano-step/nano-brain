package bench

import (
	"math"
	"sort"
	"strings"
)

type QueryResult struct {
	Query              string
	RelevantDocIDs     []string
	RelevantSourcePaths []string
	ReturnedDocIDs     []string
	ReturnedSourcePaths []string
	LatencyMs          float64
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
	var counted int
	for _, r := range results {
		if len(r.RelevantDocIDs) == 0 {
			continue
		}
		counted++
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
	if counted == 0 {
		return 0
	}
	return sum / float64(counted)
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

func pathMatches(returnedPaths, relevantPaths []string) int {
	if len(relevantPaths) == 0 {
		return 0
	}
	relevant := make(map[string]bool, len(relevantPaths))
	for _, p := range relevantPaths {
		relevant[strings.ToLower(p)] = true
	}
	var hits int
	for _, p := range returnedPaths {
		pLower := strings.ToLower(p)
		if relevant[pLower] {
			hits++
			continue
		}
		for _, rel := range relevantPaths {
			relLower := strings.ToLower(rel)
			if strings.HasSuffix(pLower, "/"+relLower) || strings.HasSuffix(pLower, "\\"+relLower) {
				hits++
				break
			}
		}
	}
	return hits
}

func PrecisionAtKPaths(results []QueryResult, k int) float64 {
	if len(results) == 0 || k <= 0 {
		return 0
	}
	var sum float64
	for _, r := range results {
		if len(r.RelevantSourcePaths) == 0 {
			continue
		}
		top := r.ReturnedSourcePaths
		if len(top) > k {
			top = top[:k]
		}
		hits := pathMatches(top, r.RelevantSourcePaths)
		sum += float64(hits) / float64(k)
	}
	return sum / float64(len(results))
}

func RecallAtKPaths(results []QueryResult, k int) float64 {
	if len(results) == 0 || k <= 0 {
		return 0
	}
	var sum float64
	var counted int
	for _, r := range results {
		if len(r.RelevantSourcePaths) == 0 {
			continue
		}
		counted++
		top := r.ReturnedSourcePaths
		if len(top) > k {
			top = top[:k]
		}
		hits := pathMatches(top, r.RelevantSourcePaths)
		sum += float64(hits) / float64(len(r.RelevantSourcePaths))
	}
	if counted == 0 {
		return 0
	}
	return sum / float64(counted)
}

func MRRPaths(results []QueryResult) float64 {
	if len(results) == 0 {
		return 0
	}
	var sum float64
	for _, r := range results {
		if len(r.RelevantSourcePaths) == 0 {
			continue
		}
		relevant := make(map[string]bool, len(r.RelevantSourcePaths))
		for _, p := range r.RelevantSourcePaths {
			relevant[strings.ToLower(p)] = true
		}
		for rank, p := range r.ReturnedSourcePaths {
			pLower := strings.ToLower(p)
			if relevant[pLower] {
				sum += 1.0 / float64(rank+1)
				break
			}
		for _, rel := range r.RelevantSourcePaths {
			relLower := strings.ToLower(rel)
			if strings.HasSuffix(pLower, "/"+relLower) || strings.HasSuffix(pLower, "\\"+relLower) {
				sum += 1.0 / float64(rank+1)
				break
			}
		}
		}
	}
	return sum / float64(len(results))
}
