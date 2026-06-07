package graph

import "math"

func ComputePageRank(edges []Edge, dampingFactor float64, maxIterations int, tolerance float64) map[string]float64 {
	outLinks := make(map[string][]string)
	nodes := make(map[string]struct{})

	for _, e := range edges {
		outLinks[e.SourceNode] = append(outLinks[e.SourceNode], e.TargetNode)
		nodes[e.SourceNode] = struct{}{}
		nodes[e.TargetNode] = struct{}{}
	}

	n := len(nodes)
	if n == 0 {
		return nil
	}

	scores := make(map[string]float64, n)
	initial := 1.0 / float64(n)
	for node := range nodes {
		scores[node] = initial
	}

	base := (1 - dampingFactor) / float64(n)

	for iter := 0; iter < maxIterations; iter++ {
		newScores := make(map[string]float64, n)
		for node := range nodes {
			newScores[node] = base
		}

		for src, targets := range outLinks {
			share := scores[src] / float64(len(targets))
			for _, tgt := range targets {
				newScores[tgt] += dampingFactor * share
			}
		}

		diff := 0.0
		for node := range nodes {
			diff += math.Abs(newScores[node] - scores[node])
		}

		scores = newScores
		if diff < tolerance {
			break
		}
	}

	maxScore := 0.0
	for _, s := range scores {
		if s > maxScore {
			maxScore = s
		}
	}
	if maxScore > 0 {
		for node := range scores {
			scores[node] /= maxScore
		}
	}

	return scores
}
