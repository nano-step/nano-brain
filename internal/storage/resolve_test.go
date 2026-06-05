package storage_test

import (
	"context"
	"testing"

	"github.com/nano-brain/nano-brain/internal/storage"
)

func TestIsHex(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"a7b3c9d1e4f56789abcdef0123456789abcdef0123456789abcdef0123456789", true},
		{"ABCDEF0123456789", true},
		{"nano-brain", false},
		{"deadbeef", true},
		{"g0000000", false},
		{"", true}, // empty string — all chars pass vacuously
	}
	for _, tc := range cases {
		if got := storage.IsHex(tc.input); got != tc.want {
			t.Errorf("IsHex(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestResolveWorkspaceParam_FullHash(t *testing.T) {
	// Full 64-char hex hash must be returned as-is without any DB lookup.
	// Pass nil for queries to confirm no DB call is made.
	fullHash := "a7b3c9d1e4f56789abcdef0123456789abcdef0123456789abcdef0123456789"
	got, err := storage.ResolveWorkspaceParam(context.Background(), nil, fullHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != fullHash {
		t.Errorf("got %q, want %q", got, fullHash)
	}
}
