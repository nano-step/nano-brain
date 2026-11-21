package config

// SecretFieldPaths lists dot-paths that must be redacted in HTTP responses
// and rejected from HTTP patches. Security-critical: modifying this list
// changes what the config API exposes.
var SecretFieldPaths = []string{
	"database.url",
	"embedding.voyage_api_key",
	"summarization.api_key",
	"code_summarization.api_key",
	"intelligence.api_key",
	"bench.api_key",
	"search.hyde.api_key",
	"search.reranking.api_key",
	"search.query_preprocessing.api_key",
	"server.auth.users",
	"server.auth.tokens",
}

func IsSecretFieldPath(path string) bool {
	for _, s := range SecretFieldPaths {
		if s == path {
			return true
		}
	}
	return false
}

func RedactSecrets(cfg *Config) *Config {
	cp := *cfg
	cp.Database.URL = "<redacted>"
	cp.Embedding.VoyageAPIKey = "<redacted>"
	cp.Summarization.APIKey = "<redacted>"
	cp.CodeSummarization.APIKey = "<redacted>"
	cp.Intelligence.APIKey = "<redacted>"
	cp.Bench.APIKey = "<redacted>"
	cp.Search.HyDE.APIKey = "<redacted>"
	cp.Search.Reranking.APIKey = "<redacted>"
	cp.Search.QueryPreprocessing.APIKey = "<redacted>"
	cp.Server.Auth.Users = nil
	cp.Server.Auth.Tokens = nil
	return &cp
}
