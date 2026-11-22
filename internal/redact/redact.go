// Package redact scrubs credential-shaped substrings from text before it
// leaves the process for external providers (embedding, summarization,
// reranking, LLM calls). It is an egress-only control (#660, option c):
// content stored in the local database is left untouched so search and
// retrieval stay lossless; only what a provider would receive is masked.
package redact

import "regexp"

type pattern struct {
	name        string
	re          *regexp.Regexp
	replacement string
}

// Ordered from most-specific/multi-line shapes to generic assignments. Each
// match is replaced with [REDACTED:<kind>] (or `key=[REDACTED]` for
// assignments, preserving the field name so surrounding text stays readable).
var patterns = []pattern{
	{
		"private-key",
		regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY( BLOCK)?-----[\s\S]*?-----END [A-Z0-9 ]*PRIVATE KEY( BLOCK)?-----`),
		"[REDACTED:private-key]",
	},
	{
		"jwt",
		regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`),
		"[REDACTED:jwt]",
	},
	{
		"aws-key",
		regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`),
		"[REDACTED:aws-key]",
	},
	{
		"github-token",
		regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr|ghc)_[A-Za-z0-9]{30,}\b|\bgithub_pat_[A-Za-z0-9_]{22,}\b`),
		"[REDACTED:github-token]",
	},
	{
		"api-key",
		regexp.MustCompile(`\bsk-(?:ant-|proj-|live-|test-|or-)?[A-Za-z0-9_-]{16,}\b|\bxai-[A-Za-z0-9_-]{16,}\b|\bnpm_[A-Za-z0-9]{30,}\b|\bpypi-[A-Za-z0-9_-]{16,}\b`),
		"[REDACTED:api-key]",
	},
	{
		"slack-token",
		regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`),
		"[REDACTED:slack-token]",
	},
	{
		"google-key",
		regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{30,}\b`),
		"[REDACTED:google-key]",
	},
	{
		"bearer",
		regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]{12,}`),
		"[REDACTED:bearer]",
	},
	{
		"secret-assignment",
		regexp.MustCompile(`(?i)(\w*(?:api[_-]?key|api[_-]?secret|secret[_-]?key|access[_-]?key|auth[_-]?token|access[_-]?token|refresh[_-]?token|client[_-]?secret|private[_-]?key|password|passwd|pgpass)\w*)\s*[:=]\s*["']?[^\s"',;$}\[]{8,}`),
		"${1}=[REDACTED]",
	},
}

// Scrub replaces credential-shaped substrings in text with stable
// [REDACTED:<kind>] markers. Non-secret content is returned unchanged.
func Scrub(text string) string {
	for _, p := range patterns {
		if p.re.MatchString(text) {
			text = p.re.ReplaceAllString(text, p.replacement)
		}
	}
	return text
}
