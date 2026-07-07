//go:build integration

package mcp_test

import (
	"fmt"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nano-brain/nano-brain/internal/search"
)

// TestMemoryVSearch_OverFetchCapFloor_DeepPagination is regression coverage for
// the R88 review follow-up to #545: vsearchDedupOverFetchCap (200 chunks) was an
// unconditional ceiling with no floor at baseFetchLimit. Once
// offset+max_results+1 exceeded the 200 cap, min(base*factor, cap) picked a
// fetchLimit SMALLER than the page window itself needed, silently truncating
// pages and reporting has_more=false even though more documents existed.
//
// Seeds 210 distinct-similarity, single-chunk documents (no hot-document
// collapse needed — this bug hits the plain page-window math, independent of
// dedup) and pages deep enough that baseFetchLimit (206) exceeds the cap
// (200):
//
//   offset=145, max_results=60 -> baseFetchLimit = 145+60+1 = 206 > 200
//
// Pre-fix: fetchLimit = min(206*5, 200) = 200, so only ranks 1-200 are
// fetched. The requested page (ranks 146-205) is truncated to ranks 146-200
// (55 items instead of 60), and has_more incorrectly reports false (200 is
// not > 205) even though 5 more documents (ranks 206-210) exist.
//
// Post-fix: fetchLimit = max(206, min(206*5, 200)) = 206, so ranks 1-206 are
// fetched. The full 60-item page (ranks 146-205) is returned and has_more
// correctly reports true (206 > 205).
func TestMemoryVSearch_OverFetchCapFloor_DeepPagination(t *testing.T) {
	ctx, q, wsHash, callTool := setupVSearchMCP(t)

	const totalDocs = 210
	for i := 0; i < totalDocs; i++ {
		alpha := float32(1.0) - float32(i)*0.004
		seedVSearchDoc(t, ctx, q, wsHash, fmt.Sprintf("deep-doc-%03d", i), []float32{alpha})
	}

	const maxResults = 60
	const offset = 145 // offset+maxResults+1 = 206 > vsearchDedupOverFetchCap (200)

	hashInput := search.QueryHashInput{Query: "probe concept", Scope: wsHash}
	cursor := search.EncodeCursor(offset, search.QueryHash(hashInput))

	result := callTool("memory_vsearch", map[string]any{
		"workspace": wsHash, "query": "probe concept", "max_results": maxResults, "cursor": cursor,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(*mcpsdk.TextContent).Text)
	}

	resp, _ := parseSearchResponse(t, result)
	items := parseResultItems(t, resp)

	if len(items) != maxResults {
		t.Fatalf("got %d results, want %d (baseFetchLimit=%d > cap=200 must not starve the page window)",
			len(items), maxResults, offset+maxResults+1)
	}
	if resp.NextCursor == "" {
		t.Fatalf("next_cursor empty, want non-empty: %d more documents exist beyond offset+max_results=%d (total seeded=%d)",
			totalDocs-(offset+maxResults), offset+maxResults, totalDocs)
	}

	seenDocs := make(map[string]bool, len(items))
	for _, item := range items {
		if seenDocs[item.DocumentID] {
			t.Errorf("duplicate document_id %s in a single group_by=document page", item.DocumentID)
		}
		seenDocs[item.DocumentID] = true
	}
	if len(seenDocs) != maxResults {
		t.Fatalf("page contains %d distinct documents, want %d", len(seenDocs), maxResults)
	}
}
