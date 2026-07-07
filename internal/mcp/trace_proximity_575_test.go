package mcp

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// Issue #575 (#542 F2): a bare call must resolve to the same-named symbol nearest
// the caller in the directory tree; genuine ties stay ambiguous.
func TestNearestSymbolMatch(t *testing.T) {
	sp := func(f string) sqlc.ResolveSymbolByNameRow {
		return sqlc.ResolveSymbolByNameRow{SourcePath: f + "?symbol=foo&kind=function"}
	}
	tests := []struct {
		name     string
		caller   string
		matches  []sqlc.ResolveSymbolByNameRow
		wantOK   bool
		wantFile string // source_path prefix expected (before "?")
	}{
		{
			name:     "cross-repo: nearest subtree wins",
			caller:   "backend/controllers/pay.js",
			matches:  []sqlc.ResolveSymbolByNameRow{sp("backend/services/pay.js"), sp("frontend/net/pay.js")},
			wantOK:   true,
			wantFile: "backend/services/pay.js",
		},
		{
			name:     "same file wins outright",
			caller:   "backend/controllers/pay.js",
			matches:  []sqlc.ResolveSymbolByNameRow{sp("backend/controllers/pay.js"), sp("frontend/net/pay.js")},
			wantOK:   true,
			wantFile: "backend/controllers/pay.js",
		},
		{
			name:    "tie within same subtree stays ambiguous",
			caller:  "backend/controllers/pay.js",
			matches: []sqlc.ResolveSymbolByNameRow{sp("backend/services/pay.js"), sp("backend/utils/pay.js")},
			wantOK:  false,
		},
		{
			name:    "empty caller file -> no disambiguation",
			caller:  "",
			matches: []sqlc.ResolveSymbolByNameRow{sp("backend/services/pay.js"), sp("frontend/net/pay.js")},
			wantOK:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := nearestSymbolMatch(tt.caller, "", tt.matches)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (got %q)", ok, tt.wantOK, got.SourcePath)
			}
			if ok {
				gotFile := got.SourcePath[:len(tt.wantFile)]
				if gotFile != tt.wantFile {
					t.Errorf("nearest = %q, want prefix %q", got.SourcePath, tt.wantFile)
				}
			}
		})
	}
}

func TestSharedPathDepth(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"a/b/x.go", "a/b/y.go", 2},
		{"a/x.go", "c/y.go", 0},
		{"backend/ctrl.js", "backend/svc.js", 1},
		{"a/b/c.go", "a/b/c.go", 3},
	}
	for _, c := range cases {
		if got := sharedPathDepth(c.a, c.b); got != c.want {
			t.Errorf("sharedPathDepth(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
