package harvest

import (
	"strings"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// sanitizeText makes a session-derived string storable in PostgreSQL: TEXT
// rejects NUL (0x00) and invalid UTF-8 (SQLSTATE 22021), and JSONB rejects
// "\u0000". NUL bytes are removed and invalid sequences become U+FFFD. A
// string that is already clean is returned as is, without allocating.
func sanitizeText(s string) string {
	return strings.ToValidUTF8(strings.ReplaceAll(s, "\x00", ""), "\uFFFD")
}

// sanitizeStrings applies sanitizeText to each element. The input slice is
// never modified; it is returned as is when every element is already clean.
func sanitizeStrings(in []string) []string {
	for i, s := range in {
		if sanitizeText(s) == s {
			continue
		}
		out := make([]string, len(in))
		copy(out, in[:i])
		for j := i; j < len(in); j++ {
			out[j] = sanitizeText(in[j])
		}
		return out
	}
	return in
}

// sanitizeMetadata sanitizes the string values of a document metadata map in
// place, before it is marshalled to JSONB, and returns the map. Nested maps
// and slices are walked too: a dirty string anywhere in the JSON tree makes
// Postgres reject the whole value.
func sanitizeMetadata(m map[string]any) map[string]any {
	for k, v := range m {
		m[k] = sanitizeJSONValue(v)
	}
	return m
}

func sanitizeJSONValue(v any) any {
	switch t := v.(type) {
	case string:
		return sanitizeText(t)
	case map[string]any:
		return sanitizeMetadata(t)
	case []string:
		return sanitizeStrings(t)
	case []any:
		for i, e := range t {
			t[i] = sanitizeJSONValue(e)
		}
		return t
	default:
		return v
	}
}

// sanitizeDocParams is the persistence boundary for session documents: every
// session-derived text field of the upsert is passed through sanitizeText.
// Content must already be sanitized by the caller, because ContentHash is
// computed from it; the pass here is a no-op kept as a last line of defence.
func sanitizeDocParams(p sqlc.UpsertDocumentBySourcePathParams) sqlc.UpsertDocumentBySourcePathParams {
	p.Title = sanitizeText(p.Title)
	p.Content = sanitizeText(p.Content)
	p.SourcePath = sanitizeText(p.SourcePath)
	p.Tags = sanitizeStrings(p.Tags)
	return p
}

// sanitizeSummaryMeta sanitizes the session-derived fields of a SummaryMeta,
// which internal/summarize persists (title, source path, tags, metadata).
func sanitizeSummaryMeta(m SummaryMeta) SummaryMeta {
	m.SessionID = sanitizeText(m.SessionID)
	m.Title = sanitizeText(m.Title)
	m.Agent = sanitizeText(m.Agent)
	m.ProjectPath = sanitizeText(m.ProjectPath)
	m.ParentID = sanitizeText(m.ParentID)
	m.Branch = sanitizeText(m.Branch)
	m.Cwd = sanitizeText(m.Cwd)
	m.Tags = sanitizeStrings(m.Tags)
	return m
}
