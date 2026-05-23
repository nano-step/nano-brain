package search

import "sort"

// RRFMerge merges two ranked result lists using Reciprocal Rank Fusion.
// score = Σ 1/(k + rank + 1) for each list the result appears in.
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
		return out[i].Score > out[j].Score
	})

	return out
}
