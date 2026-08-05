package symbol

import (
	"slices"
	"testing"
)

func TestExpandImpactFrontierKeepsLegacyBareSuffixExpansion(t *testing.T) {
	// Given: historical qualified Ruby targets still depend on bare fallback.
	input := "app/controllers/stories.rb::Api::V2::StoriesController#sync"

	// When
	got := ExpandImpactFrontier([]string{input})

	// Then
	want := []string{input, "Api::V2::StoriesController#sync"}
	if !slices.Equal(got, want) {
		t.Fatalf("legacy impact frontier = %q, want %q", got, want)
	}
}

func TestExpandImpactFrontierDoesNotBareExpandCanonicalJSTarget(t *testing.T) {
	// Given: a source-scoped JS/TS target must never be reconciled by its suffix.
	input := "repo-a/lib/api.ts::run"

	// When
	got := ExpandImpactFrontier([]string{input})

	// Then
	want := []string{input}
	if !slices.Equal(got, want) {
		t.Fatalf("canonical JS/TS impact frontier = %q, want %q", got, want)
	}
}
