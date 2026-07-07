package mcp

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// Issue #565 (#542 F5): a path covered by a registered ancestor workspace must
// resolve to that ancestor via mostSpecificAncestor (longest path-boundary match),
// not string-prefix, and never bind a sibling with a shared prefix.
func TestMostSpecificAncestor(t *testing.T) {
	wss := []sqlc.Workspace{
		{Hash: "h-mono", Name: "monorepo", Path: "/src/monorepo"},
		{Hash: "h-inner", Name: "inner", Path: "/src/monorepo/services"},
		{Hash: "h-other", Name: "other", Path: "/src/monorepo-api"}, // shared prefix, NOT an ancestor
	}

	tests := []struct {
		name    string
		path    string
		wantOK  bool
		wantHsh string
	}{
		{"sub-repo covered by monorepo", "/src/monorepo/backend", true, "h-mono"},
		{"deeper path prefers most-specific ancestor", "/src/monorepo/services/api", true, "h-inner"},
		{"trailing-slash ancestor still matches", "/src/monorepo/services/api/x", true, "h-inner"},
		{"shared-prefix sibling is NOT an ancestor", "/src/monorepo-api/x", true, "h-other"},
		{"unrelated path has no ancestor", "/etc/passwd", false, ""},
		{"exact registered path is skipped (resolved by hash upstream)", "/src/monorepo", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := mostSpecificAncestor(wss, tt.path)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (got %q)", ok, tt.wantOK, got.Hash)
			}
			if ok && got.Hash != tt.wantHsh {
				t.Errorf("ancestor = %q, want %q", got.Hash, tt.wantHsh)
			}
		})
	}
}
