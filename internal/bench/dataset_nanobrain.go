package bench

import "time"

func NanoBrainDataset() *BenchmarkDataset {
	return &BenchmarkDataset{
		Scale:         len(nanoBrainQueries),
		WorkspaceHash: "7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Entries:       nanoBrainQueries,
	}
}

var nanoBrainQueries = []DatasetEntry{
	{
		Query: "hybrid search pipeline",
		RelevantSourcePaths: []string{
			"internal/search/service.go",
			"internal/search/rrf.go",
			"internal/search/search.go",
			"internal/search/AGENTS.md",
		},
	},
	{
		Query: "embedding provider ollama",
		RelevantSourcePaths: []string{
			"internal/embed/provider.go",
			"internal/embed/ollama.go",
		},
	},
	{
		Query: "session harvesting opencode",
		RelevantSourcePaths: []string{
			"internal/harvest/harvester.go",
			"internal/harvest/opencode.go",
		},
	},
	{
		Query: "watcher file system notify",
		RelevantSourcePaths: []string{
			"internal/watcher/watcher.go",
		},
	},
	{
		Query: "RRF fusion ranking",
		RelevantSourcePaths: []string{
			"internal/search/rrf.go",
		},
	},
	{
		Query: "deduplication content hash",
		RelevantSourcePaths: []string{
			"internal/search/dedup.go",
		},
	},
	{
		Query: "code aware ranking boost",
		RelevantSourcePaths: []string{
			"internal/search/ranking.go",
		},
	},
	{
		Query: "MCP tools memory query",
		RelevantSourcePaths: []string{
			"internal/mcp/tools.go",
		},
	},
	{
		Query: "snippet extraction relevant context",
		RelevantSourcePaths: []string{
			"internal/search/snippet.go",
		},
	},
	{
		Query: "config hot reload search",
		RelevantSourcePaths: []string{
			"internal/config/config.go",
			"internal/search/service.go",
		},
	},
	{
		Query: "database migration goose",
		RelevantSourcePaths: []string{
			"internal/storage/migrate.go",
		},
	},
	{
		Query: "HTTP handlers search API",
		RelevantSourcePaths: []string{
			"internal/server/handlers/search.go",
			"internal/server/handlers/query.go",
		},
	},
	{
		Query: "BM25 full text search PostgreSQL",
		RelevantSourcePaths: []string{
			"internal/search/service.go",
			"internal/search/AGENTS.md",
		},
	},
	{
		Query: "vector similarity cosine pgvector",
		RelevantSourcePaths: []string{
			"internal/search/service.go",
		},
	},
	{
		Query: "reranker cross encoder Cohere",
		RelevantSourcePaths: []string{
			"internal/search/reranking/cohere.go",
		},
	},
	{
		Query: "page rank document scoring",
		RelevantSourcePaths: []string{
			"internal/search/pagerank.go",
		},
	},
	{
		Query: "entity boost query extraction",
		RelevantSourcePaths: []string{
			"internal/search/entity.go",
		},
	},
	{
		Query: "recency decay half life",
		RelevantSourcePaths: []string{
			"internal/search/recency.go",
		},
	},
	{
		Query: "workspace isolation multi-tenant",
		RelevantSourcePaths: []string{
			"internal/server/handlers/workspace.go",
		},
	},
	{
		Query: "telemetry search latency tracking",
		RelevantSourcePaths: []string{
			"internal/telemetry/telemetry.go",
		},
	},
}
