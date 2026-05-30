package config

// SecretFieldPaths lists dot-paths that must be redacted in HTTP responses
// and rejected from HTTP patches. Security-critical: modifying this list
// changes what the config API exposes.
var SecretFieldPaths = []string{
	"database.url",
	"embedding.voyage_api_key",
	"summarization.api_key",
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
	return &cp
}
