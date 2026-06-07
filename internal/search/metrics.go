package search

import "math"

func NDCG(relevanceScores []float64, k int) float64 {
	if k <= 0 || len(relevanceScores) == 0 {
		return 0
	}
	if k > len(relevanceScores) {
		k = len(relevanceScores)
	}

	dcg := dcgAtK(relevanceScores, k)
	ideal := idealDCG(relevanceScores, k)
	if ideal == 0 {
		return 0
	}
	return dcg / ideal
}

func dcgAtK(scores []float64, k int) float64 {
	var dcg float64
	for i := 0; i < k && i < len(scores); i++ {
		dcg += scores[i] / math.Log2(float64(i+2))
	}
	return dcg
}

func idealDCG(scores []float64, k int) float64 {
	sorted := make([]float64, len(scores))
	copy(sorted, scores)
	sortDesc(sorted)
	return dcgAtK(sorted, k)
}

func sortDesc(s []float64) {
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[j] > s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func Recall(relevantFound int, totalRelevant int) float64 {
	if totalRelevant == 0 {
		return 0
	}
	return float64(relevantFound) / float64(totalRelevant)
}

func MRR(rankOfFirstRelevant int) float64 {
	if rankOfFirstRelevant <= 0 {
		return 0
	}
	return 1.0 / float64(rankOfFirstRelevant)
}

func MeanMRR(ranks []int) float64 {
	if len(ranks) == 0 {
		return 0
	}
	var sum float64
	for _, r := range ranks {
		sum += MRR(r)
	}
	return sum / float64(len(ranks))
}

func MeanNDCG(scores []float64) float64 {
	if len(scores) == 0 {
		return 0
	}
	var sum float64
	for _, s := range scores {
		sum += s
	}
	return sum / float64(len(scores))
}
