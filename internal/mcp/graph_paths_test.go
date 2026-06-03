package mcp

import (
	"path"
	"strings"
	"testing"
)

func resolveNodeNoDB(workspaceRoot, node string) string {
	filePart, sym := splitNodeSymbol(node)
	if path.IsAbs(filePart) {
		return node
	}
	if path.Ext(filePart) == "" {
		return node
	}
	return path.Join(workspaceRoot, filePart) + sym
}

func TestResolveNodeAgainstWorkspace_PassThroughRules(t *testing.T) {
	root := "/Users/me/proj"
	tests := []struct {
		in   string
		want string
	}{
		{"context", "context"},
		{"github.com/foo/bar", "github.com/foo/bar"},
		{"/abs/x.go::F", "/abs/x.go::F"},
		{"/abs/x.go", "/abs/x.go"},
		{"internal/x.go::F", "/Users/me/proj/internal/x.go::F"},
		{"internal/x.go", "/Users/me/proj/internal/x.go"},
		{"cmd/main.go", "/Users/me/proj/cmd/main.go"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := resolveNodeNoDB(root, tc.in)
			if !strings.EqualFold(got, tc.want) {
				t.Errorf("resolveNodeNoDB(%q, %q) = %q, want %q", root, tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitNodeSymbol(t *testing.T) {
	tests := []struct {
		in       string
		wantFile string
		wantSym  string
	}{
		{"internal/x.go::F", "internal/x.go", "::F"},
		{"internal/x.go", "internal/x.go", ""},
		{"/abs/x.go::F", "/abs/x.go", "::F"},
		{"context", "context", ""},
		{"", "", ""},
		{"a::b::c", "a", "::b::c"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			gotFile, gotSym := splitNodeSymbol(tc.in)
			if gotFile != tc.wantFile || gotSym != tc.wantSym {
				t.Errorf("splitNodeSymbol(%q) = (%q, %q), want (%q, %q)",
					tc.in, gotFile, gotSym, tc.wantFile, tc.wantSym)
			}
		})
	}
}

func TestStripWorkspacePrefix(t *testing.T) {
	tests := []struct {
		root string
		in   string
		want string
	}{
		{"/Users/me/proj", "/Users/me/proj/internal/x.go", "internal/x.go"},
		{"/Users/me/proj", "/Users/me/proj/internal/x.go::F", "internal/x.go::F"},
		{"/Users/me/proj/", "/Users/me/proj/internal/x.go", "internal/x.go"},
		{"/Users/me/proj", "context", "context"},
		{"/Users/me/proj", "github.com/foo/bar", "github.com/foo/bar"},
		{"", "/abs/x.go", "/abs/x.go"},
		{"/Users/me/proj", "/Users/me/projOTHER/x.go", "/Users/me/projOTHER/x.go"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := stripWorkspacePrefix(tc.root, tc.in)
			if got != tc.want {
				t.Errorf("stripWorkspacePrefix(%q, %q) = %q, want %q", tc.root, tc.in, got, tc.want)
			}
		})
	}
}
