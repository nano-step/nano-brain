package harvest

import (
	"strings"
	"time"
)

// NormalizedMessage is a source-agnostic representation of a single message
// within a session.
type NormalizedMessage struct {
	Role        string
	Content     string
	Timestamp   time.Time
	ToolName    string
	IsSidechain bool
}

// NormalizedSession is a source-agnostic representation of a harvested session.
// All SessionSource adapters produce this type so the generic Engine can
// process sessions uniformly.
type NormalizedSession struct {
	Source        string
	SessionID     string
	ParentID      string
	WorkspaceHash string
	Branch        string
	Cwd           string
	Title         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Messages      []NormalizedMessage
}

// IsActive reports whether the session was updated recently enough that it
// should be skipped by the harvest engine (still in progress).
// Mirrors the existing isActiveSession logic in opencode_sqlite.go:274.
func (s NormalizedSession) IsActive() bool {
	return !s.UpdatedAt.IsZero() && time.Since(s.UpdatedAt) < 10*time.Minute
}

// RenderMarkdown renders a NormalizedSession into a markdown document string
// suitable for summarization or raw storage. The output format mirrors the
// per-source render functions (renderSQLiteMarkdown, renderClaudeCodeMarkdown).
func RenderMarkdown(sess NormalizedSession) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("session_id: " + sess.SessionID + "\n")
	b.WriteString("source: " + sess.Source + "\n")
	if sess.ParentID != "" {
		b.WriteString("parent_id: " + sess.ParentID + "\n")
	}
	if sess.Branch != "" {
		b.WriteString("branch: " + sess.Branch + "\n")
	}
	if sess.Cwd != "" {
		b.WriteString("cwd: " + sess.Cwd + "\n")
	}
	if !sess.CreatedAt.IsZero() {
		b.WriteString("created_at: " + sess.CreatedAt.Format(time.RFC3339) + "\n")
	}
	if sess.Title != "" {
		b.WriteString("title: " + sess.Title + "\n")
	}
	b.WriteString("message_count: " + itoa(len(sess.Messages)) + "\n")
	b.WriteString("---\n")

	for _, msg := range sess.Messages {
		ts := ""
		if !msg.Timestamp.IsZero() {
			ts = msg.Timestamp.UTC().Format(time.RFC3339)
		}
		if msg.ToolName != "" {
			b.WriteString("\n## assistant (" + ts + ")\n\n")
			b.WriteString("Tool: " + msg.ToolName + "\n")
		} else {
			b.WriteString("\n## " + msg.Role + " (" + ts + ")\n\n")
		}
		b.WriteString(sanitizeText(msg.Content))
		b.WriteString("\n")
	}
	return b.String()
}

// itoa is a minimal int-to-string helper to avoid importing strconv or fmt.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
