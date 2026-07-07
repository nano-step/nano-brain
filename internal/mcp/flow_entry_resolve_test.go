package mcp

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

// Issue #563 (split from #542 F1): memory_flow must resolve the full mounted URL
// an agent supplies (e.g. "POST /api/payments/payment-intent") to the stored
// router-local HTTP key ("POST /payment-intent") via "/"-aligned suffix match.
func TestResolveFlowEntry(t *testing.T) {
	edges := []graph.Edge{
		{Kind: graph.EdgeHTTP, SourceNode: "POST /payment-intent", TargetNode: "createPaymentIntent"},
		{Kind: graph.EdgeHTTP, SourceNode: "GET /health", TargetNode: "health"},
		// a more specific route sharing a suffix, to test most-specific-wins
		{Kind: graph.EdgeHTTP, SourceNode: "POST /payments/payment-intent", TargetNode: "otherHandler"},
		{Kind: graph.EdgeCalls, SourceNode: "a.go::foo", TargetNode: "bar"},
	}

	tests := []struct {
		name      string
		entry     string
		wantKey   string
		wantOK    bool
		wantExact bool // resolved == entry
	}{
		{"exact router-local still matches", "POST /payment-intent", "POST /payment-intent", true, true},
		{"full mounted URL resolves to router-local", "POST /api/payments/payment-intent", "POST /payments/payment-intent", true, false},
		{"most-specific suffix wins", "POST /x/payments/payment-intent", "POST /payments/payment-intent", true, false},
		{"method must match", "GET /api/payments/payment-intent", "", false, false},
		{"no mid-segment false match", "POST /prefix-payment-intent", "", false, false},
		{"unknown route", "DELETE /nope", "", false, false},
		{"unknown symbol entry", "a.go::missing", "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, ok := resolveFlowEntry(edges, tt.entry)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (got key %q)", ok, tt.wantOK, got)
			}
			if ok && got != tt.wantKey {
				t.Errorf("resolved key = %q, want %q", got, tt.wantKey)
			}
			if ok && (got == tt.entry) != tt.wantExact {
				t.Errorf("exact = %v, want %v (key %q, entry %q)", got == tt.entry, tt.wantExact, got, tt.entry)
			}
		})
	}
}

// The "/prefix-payment-intent" case above pins the boundary guarantee: because a
// stored path starts with "/", HasSuffix cannot match mid-segment ("/payment-intent"
// is NOT a suffix of "/prefix-payment-intent"), so we never bind a wrong route.
func TestResolveFlowEntry_CandidatesOnAmbiguousSuffix(t *testing.T) {
	// Two stored keys, same method, whose paths are BOTH suffixes of the request
	// at the same length would be identical strings — so ambiguity surfaces only
	// when a shorter and a longer both match; both are reported as candidates.
	edges := []graph.Edge{
		{Kind: graph.EdgeHTTP, SourceNode: "POST /payment-intent", TargetNode: "h1"},
		{Kind: graph.EdgeHTTP, SourceNode: "POST /payments/payment-intent", TargetNode: "h2"},
	}
	got, candidates, ok := resolveFlowEntry(edges, "POST /api/payments/payment-intent")
	if !ok {
		t.Fatal("expected resolution")
	}
	if got != "POST /payments/payment-intent" {
		t.Errorf("best = %q, want most-specific POST /payments/payment-intent", got)
	}
	if len(candidates) != 2 {
		t.Errorf("candidates = %v, want both suffix matches reported", candidates)
	}
}
