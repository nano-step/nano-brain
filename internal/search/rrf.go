package search

import "sort"

// ComputeRRFK calculates an adjusted k value for RRF based on signal confidence.
func ComputeRRFK(bm25Results, vectorResults []Result, baseK float64) float64 {
	if len(bm25Results) < 3 || len(vectorResults) < 3 {
		return baseK
	}

	bm25Set := make(map[string]bool)
	for _, r := range bm25Results {
		bm25Set[r.ID] = true
	}

	var overlap int
	for _, r := range vectorResults {
		if bm25Set[r.ID] {
			overlap++
		}
	}

	minLen := len(bm25Results)
	if len(vectorResults) < minLen {
		minLen = len(vectorResults)
	}
	
	if minLen == 0 {
		return baseK
	}
	
	overlapRatio := float64(overlap) / float64(minLen)
	adjustedK := baseK * (1.5 - overlapRatio)
	
	minK := baseK * 0.5
	maxK := baseK * 2.0
	
	if adjustedK < minK {
		return minK
	}
	if adjustedK > maxK {
		return maxK
	}
	
	return adjustedK
}

// DynamicRRFMerge combines two ranked result lists using Reciprocal Rank Fusion
// with a dynamically adjusted k parameter based on signal confidence.
func DynamicRRFMerge(bm25Results, vectorResults []Result, baseK float64) []Result {
	k := ComputeRRFK(bm25Results, vectorResults, baseK)
	return RRFMerge(bm25Results, vectorResults, k)
}

// RRFMerge merges two ranked result lists using Reciprocal Rank Fusion.
// score = Σ 1/(k + rank + 1) for each list the result appears in.
// When a chunk appears in both lists, metadata is kept from the first occurrence (BM25).
// Both queries return identical metadata for the same chunk ID so the choice is safe.
func RRFMerge(bm25Results, vectorResults []Result, k float64) []Result {
	type entry struct {
		result Result
		score  float64
	}
	merged := make(map[string]*entry)

	for rank, r := range bm25Results {
		s := 1.0 / (k + float64(rank) + 1)
		if e, ok := merged[r.ID]; ok {
			e.score += s
		} else {
			merged[r.ID] = &entry{result: r, score: s}
		}
	}

	for rank, r := range vectorResults {
		s := 1.0 / (k + float64(rank) + 1)
		if e, ok := merged[r.ID]; ok {
			e.score += s
		} else {
			merged[r.ID] = &entry{result: r, score: s}
		}
	}

	out := make([]Result, 0, len(merged))
	for _, e := range merged {
		e.result.Score = e.score
		out = append(out, e.result)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		// Stable tiebreaker for deterministic cursor pagination (#358).
		return out[i].ID < out[j].ID
	})

	return out
}
