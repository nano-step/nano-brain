package harvest

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
	"unsafe"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

const invalidUTF8 = string("a\xffb") // 0x61 0xff 0x62

// storable reports whether Postgres TEXT/JSONB would accept s (#675).
func storable(s string) bool {
	return !strings.Contains(s, "\x00") && utf8.ValidString(s)
}

func TestSanitizeText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"clean ascii", "plain title", "plain title"},
		{"clean multi-byte", "tiêu đề 日本語 🙂", "tiêu đề 日本語 🙂"},
		{"nul", "before\x00after", "beforeafter"},
		{"invalid utf8", invalidUTF8, "a\uFFFDb"},
		{"nul and invalid utf8", "x\x00" + invalidUTF8, "xa\uFFFDb"},
		{"truncated rune", "caf\xc3", "caf\uFFFD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeText(tt.in)
			if got != tt.want {
				t.Errorf("sanitizeText(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if !storable(got) {
				t.Errorf("sanitizeText(%q) = %q is not storable", tt.in, got)
			}
			if again := sanitizeText(got); again != got {
				t.Errorf("not idempotent: %q -> %q", got, again)
			}
		})
	}
}

// A clean string must come back as the very same string: the content hash of
// every already-stored session depends on it, and the pass runs on every
// field of every session.
func TestSanitizeText_CleanInputIsReturnedUnchanged(t *testing.T) {
	in := strings.Repeat("clean content — đã sạch\n", 64)
	out := sanitizeText(in)
	if unsafe.StringData(out) != unsafe.StringData(in) || len(out) != len(in) {
		t.Errorf("clean input was copied instead of returned as is")
	}
	if allocs := testing.AllocsPerRun(100, func() { _ = sanitizeText(in) }); allocs != 0 {
		t.Errorf("sanitizeText allocated %v times for clean input, want 0", allocs)
	}
}

func TestSanitizeStrings(t *testing.T) {
	clean := []string{"opencode", "session"}
	if got := sanitizeStrings(clean); &got[0] != &clean[0] {
		t.Errorf("clean slice was copied")
	}
	if got := sanitizeStrings(nil); got != nil {
		t.Errorf("sanitizeStrings(nil) = %v, want nil", got)
	}

	dirty := []string{"ok", "ticket:DEV\x00-1", invalidUTF8}
	got := sanitizeStrings(dirty)
	if want := []string{"ok", "ticket:DEV-1", "a\uFFFDb"}; !reflect.DeepEqual(got, want) {
		t.Errorf("sanitizeStrings = %q, want %q", got, want)
	}
	if dirty[1] != "ticket:DEV\x00-1" {
		t.Errorf("input slice was modified")
	}
}

func TestSanitizeMetadata(t *testing.T) {
	got := sanitizeMetadata(map[string]any{
		"source":        "pi",
		"session_id":    "ses\x00-1",
		"message_count": 3,
		"fallback":      true,
		"nested":        map[string]any{"id": "a" + invalidUTF8},
		"list":          []any{"b\x00c", 2},
		"str_list":      []string{"t" + invalidUTF8},
	})
	want := map[string]any{
		"source": "pi", "session_id": "ses-1", "message_count": 3, "fallback": true,
		"nested": map[string]any{"id": "aa\uFFFDb"}, "list": []any{"bc", 2}, "str_list": []string{"ta\uFFFDb"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sanitizeMetadata = %v, want %v", got, want)
	}
}

func TestSanitizeDocParams(t *testing.T) {
	clean := sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: "ws",
		ContentHash:   "hash",
		Title:         "Claude Code Session ses_1",
		Content:       "---\nsession_id: ses_1\n---\n",
		SourcePath:    "summary://claude/ses_1",
		Collection:    "sessions",
		Tags:          []string{"claude_code", "session", "fallback"},
	}
	// Clean data is stored exactly as before, so no stored session changes its
	// source_path or title and none is re-harvested.
	if got := sanitizeDocParams(clean); !reflect.DeepEqual(got, clean) {
		t.Errorf("clean params changed:\n got %+v\nwant %+v", got, clean)
	}

	for _, dirt := range []struct{ name, s string }{{"nul", "x\x00y"}, {"invalid utf8", invalidUTF8}} {
		t.Run(dirt.name, func(t *testing.T) {
			got := sanitizeDocParams(sqlc.UpsertDocumentBySourcePathParams{
				Title:      "title " + dirt.s,
				Content:    "content " + dirt.s,
				SourcePath: "summary://pi/" + dirt.s,
				Tags:       []string{"pi", dirt.s},
			})
			for field, v := range map[string]string{
				"Title": got.Title, "Content": got.Content, "SourcePath": got.SourcePath, "Tags[1]": got.Tags[1],
			} {
				if !storable(v) {
					t.Errorf("%s = %q is not storable", field, v)
				}
			}
		})
	}
}

func TestSanitizeSummaryMeta(t *testing.T) {
	clean := SummaryMeta{
		Source: "claude", SessionID: "ses_1", Title: "t", Agent: "a", ProjectPath: "/p",
		ParentID: "ses_0", Branch: "main", Cwd: "/p", Tags: []string{"ticket:DEV-1"}, WorkspaceHash: "ws",
	}
	if got := sanitizeSummaryMeta(clean); !reflect.DeepEqual(got, clean) {
		t.Errorf("clean meta changed:\n got %+v\nwant %+v", got, clean)
	}

	const d = "x\x00y" + invalidUTF8
	got := sanitizeSummaryMeta(SummaryMeta{
		SessionID: d, Title: d, Agent: d, ProjectPath: d, ParentID: d, Branch: d, Cwd: d, Tags: []string{d},
	})
	for field, v := range map[string]string{
		"SessionID": got.SessionID, "Title": got.Title, "Agent": got.Agent, "ProjectPath": got.ProjectPath,
		"ParentID": got.ParentID, "Branch": got.Branch, "Cwd": got.Cwd, "Tags[0]": got.Tags[0],
	} {
		if v != "xya\uFFFDb" {
			t.Errorf("%s = %q, want %q", field, v, "xya\uFFFDb")
		}
	}
}

// Header fields are rendered outside the message bodies, so a renderer that
// only sanitizes message content still emits a document Postgres rejects
// (#675). Every renderer must return a storable document whatever field the
// dirt is in.
func TestRenderers_SanitizeHeaderFields(t *testing.T) {
	for _, dirt := range []struct{ name, s string }{{"nul", "x\x00y"}, {"invalid utf8", invalidUTF8}} {
		d := dirt.s
		tests := []struct {
			name string
			md   string
		}{
			{"engine", RenderMarkdown(NormalizedSession{
				Source: "opencode", SessionID: "ses" + d, ParentID: "par" + d, Branch: "br" + d, Cwd: "/cwd" + d, Title: "title" + d,
				Messages: []NormalizedMessage{
					{Role: "user" + d, Content: "hello"},
					{Role: "assistant", ToolName: "bash" + d, Content: "ls"},
				},
			})},
			{"opencode sqlite", renderSQLiteMarkdown(
				SqSession{ID: "ses" + d, Title: "title" + d},
				[]sqMessage{{role: "user" + d, content: "hello"}},
			)},
			{"opencode json", renderMarkdown(
				&sessionFile{ID: "ses" + d, Title: "title" + d, Directory: "/dir" + d},
				[]renderedMessage{{Role: "user" + d, Content: "hello"}},
			)},
			{"claude code", renderClaudeCodeMarkdown("ses"+d, nil)},
			{"pi", renderPiMarkdown("ses"+d, nil)},
			{"git commit", renderCommitMarkdown(gitCommit{
				SHA: "abc1234", Subject: "ses" + d, Author: "au" + d, Email: "em" + d,
				Body: "body" + d, Files: []string{"file" + d + ".go"},
			})},
		}
		for _, tt := range tests {
			t.Run(dirt.name+"/"+tt.name, func(t *testing.T) {
				if !storable(tt.md) {
					t.Errorf("rendered markdown is not storable: %q", tt.md)
				}
				if !strings.Contains(tt.md, "ses") {
					t.Errorf("rendered markdown lost the session id: %q", tt.md)
				}
			})
		}
	}
}

// Adding the whole-document pass must not change what a clean session renders
// to: the content hash of stored sessions is the hash of this text.
func TestRenderers_CleanInputUnchanged(t *testing.T) {
	sess := NormalizedSession{
		Source: "opencode", SessionID: "ses_1", ParentID: "ses_0", Branch: "main", Cwd: "/data/app", Title: "tiêu đề",
		Messages: []NormalizedMessage{{Role: "user", Content: "hello"}, {Role: "assistant", ToolName: "bash", Content: "ls"}},
	}
	wantEngine := "---\nsession_id: ses_1\nsource: opencode\nparent_id: ses_0\nbranch: main\ncwd: /data/app\ntitle: tiêu đề\nmessage_count: 2\n---\n" +
		"\n## user ()\n\nhello\n\n## assistant ()\n\nTool: bash\nls\n"
	if got := RenderMarkdown(sess); got != wantEngine {
		t.Errorf("engine render changed:\n got %q\nwant %q", got, wantEngine)
	}

	wantSQLite := "---\nsession_id: ses_1\nsource: opencode\nmessage_count: 1\ncreated_at: 0001-01-01T00:00:00Z\ntitle: \"tiêu đề\"\n---\n" +
		"\n## user (0001-01-01T00:00:00Z)\n\nhello\n"
	if got := renderSQLiteMarkdown(SqSession{ID: "ses_1", Title: "tiêu đề"}, []sqMessage{{role: "user", content: "hello"}}); got != wantSQLite {
		t.Errorf("opencode sqlite render changed:\n got %q\nwant %q", got, wantSQLite)
	}
}

func TestTitleFromContent(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"short", "Use PostgreSQL\nsecond line", "Use PostgreSQL"},
		{"80 bytes is kept", strings.Repeat("a", 80), strings.Repeat("a", 80)},
		{"long ascii", strings.Repeat("a", 100), strings.Repeat("a", 77) + "..."},
		// "é" is 2 bytes: byte 77 falls in the middle of the 39th rune.
		{"long 2-byte runes", strings.Repeat("é", 60), strings.Repeat("é", 38) + "..."},
		// "日" is 3 bytes: byte 77 falls in the middle of the 26th rune.
		{"long 3-byte runes", strings.Repeat("日", 40), strings.Repeat("日", 25) + "..."},
		{"long 4-byte runes", strings.Repeat("🙂", 30), strings.Repeat("🙂", 19) + "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := titleFromContent(tt.in)
			if got != tt.want {
				t.Errorf("titleFromContent = %q, want %q", got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("titleFromContent returned invalid UTF-8: %q", got)
			}
		})
	}
}
