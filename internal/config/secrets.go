package config

import (
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

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

// RedactSecrets returns a copy of cfg that is safe to serialise: credentials
// are blanked, credentials embedded in URL fields are removed, and every
// slice, map and pointer is copied so the caller cannot reach the live config
// through the result.
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

	cp.Embedding.URL = RedactURLCredentials(cp.Embedding.URL)
	cp.Summarization.ProviderURL = RedactURLCredentials(cp.Summarization.ProviderURL)
	cp.CodeSummarization.ProviderURL = RedactURLCredentials(cp.CodeSummarization.ProviderURL)
	cp.Intelligence.ProviderURL = RedactURLCredentials(cp.Intelligence.ProviderURL)
	cp.Bench.ProviderURL = RedactURLCredentials(cp.Bench.ProviderURL)
	cp.Search.HyDE.ProviderURL = RedactURLCredentials(cp.Search.HyDE.ProviderURL)
	cp.Search.Reranking.ProviderURL = RedactURLCredentials(cp.Search.Reranking.ProviderURL)
	cp.Search.QueryPreprocessing.ProviderURL = RedactURLCredentials(cp.Search.QueryPreprocessing.ProviderURL)

	cp.Server.AllowedHosts = slices.Clone(cfg.Server.AllowedHosts)
	cp.Server.Auth.BypassPaths = slices.Clone(cfg.Server.Auth.BypassPaths)
	cp.Harvester.ClaudeCode.SessionDirs = slices.Clone(cfg.Harvester.ClaudeCode.SessionDirs)
	cp.Harvester.ClaudeCode.TicketPatterns = slices.Clone(cfg.Harvester.ClaudeCode.TicketPatterns)
	cp.Watcher.ExcludePatterns = slices.Clone(cfg.Watcher.ExcludePatterns)
	cp.Watcher.AllowedExtensions = slices.Clone(cfg.Watcher.AllowedExtensions)
	cp.Watcher.Workspaces = maps.Clone(cfg.Watcher.Workspaces)
	for dir, ws := range cp.Watcher.Workspaces {
		ws.ExcludePatterns = slices.Clone(ws.ExcludePatterns)
		ws.AllowedExtensions = slices.Clone(ws.AllowedExtensions)
		cp.Watcher.Workspaces[dir] = ws
	}
	cp.Search.HyDE.ContextHints = maps.Clone(cfg.Search.HyDE.ContextHints)
	if cfg.Summarization.WriteToDisk != nil {
		writeToDisk := *cfg.Summarization.WriteToDisk
		cp.Summarization.WriteToDisk = &writeToDisk
	}
	return &cp
}

// credentialQueryParams are the query parameter names whose values
// RedactURLCredentials replaces. Names are compared case-insensitively.
var credentialQueryParams = []string{"api_key", "apikey", "api-key", "x-api-key", "key", "token", "access_token", "refresh_token", "secret", "client_secret", "password", "sig", "signature", "code", "auth"}

var urlSchemePrefix = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

// RedactURLCredentials returns raw with the userinfo (user:password@) and the
// values of credential-like query parameters replaced by "<redacted>". The
// rest of the URL is returned byte for byte. A value that does not parse as a
// URL and contains an '@' is replaced entirely.
func RedactURLCredentials(raw string) string {
	if _, err := url.Parse(raw); err != nil && strings.Contains(raw, "@") {
		// The parser and this function may disagree on where the userinfo
		// ends (an unescaped '/', '?' or '#' in a password), so nothing is kept.
		return "<redacted>"
	}
	schemeEnd := len(urlSchemePrefix.FindString(raw))

	rest, fragment, hasFragment := strings.Cut(raw, "#")
	base, query, hasQuery := strings.Cut(rest, "?")
	authorityEnd := len(base)
	if i := strings.IndexByte(base[schemeEnd:], '/'); i >= 0 {
		authorityEnd = schemeEnd + i
	}
	if at := strings.LastIndexByte(base[schemeEnd:authorityEnd], '@'); at >= 0 {
		base = base[:schemeEnd] + "<redacted>" + base[schemeEnd+at:]
	}

	if hasQuery {
		pairs := strings.Split(query, "&")
		for i, pair := range pairs {
			name, _, hasValue := strings.Cut(pair, "=")
			if !hasValue {
				continue
			}
			decoded, err := url.QueryUnescape(name)
			if err != nil {
				decoded = name
			}
			if slices.Contains(credentialQueryParams, strings.ToLower(decoded)) {
				pairs[i] = name + "=<redacted>"
			}
		}
		base += "?" + strings.Join(pairs, "&")
	}
	if hasFragment {
		base += "#" + fragment
	}
	return base
}
