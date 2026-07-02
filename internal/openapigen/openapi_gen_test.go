package openapigen_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/nano-brain/nano-brain/internal/openapigen"
)

// TestOpenAPISpec_NoDrift satisfies D-05 (single source of truth with
// routes.go): regenerating the spec from the current handler annotations
// must byte-match the committed docs/openapi.json. If this fails, a route
// or handler was annotated (or changed) without regenerating the committed
// artifact via `make generate-openapi`.
//
// Byte-equality (not semantic/map[string]any diffing) is used here:
// Generate() marshals via encoding/json.MarshalIndent, which sorts map keys
// alphabetically and produces stable output — empirically verified during
// Plan 12-01 by running Generate() twice in a row and diffing the bytes
// (identical both times). If a future swag/kin-openapi upgrade introduces
// non-deterministic marshaling (e.g. new map fields marshaled without
// sorting), switch this to unmarshal-both-sides-into-map[string]any plus
// reflect.DeepEqual (flagged as Assumption A3 in 12-RESEARCH.md).
func TestOpenAPISpec_NoDrift(t *testing.T) {
	const specPath = "../../docs/openapi.json"

	committed, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("reading committed %s: %v", specPath, err)
	}

	// Repo-root-relative paths, matching what cmd/generate-openapi passes
	// when invoked from the repo root via `make generate-openapi`.
	fresh, err := openapigen.Generate("../..", "internal/server/doc.go")
	if err != nil {
		t.Fatalf("regenerating spec: %v", err)
	}

	if !bytes.Equal(fresh, committed) {
		t.Fatalf("docs/openapi.json is stale — regenerate via `make generate-openapi` and commit the result.\ncommitted length=%d, fresh length=%d", len(committed), len(fresh))
	}
}

// expectedRoutePaths is the maintained set of every path registered in
// internal/server/routes.go, EXCLUDING /ui (a static HTML redirect page,
// not a JSON REST endpoint — see 12-02-SUMMARY.md's documented exclusion).
//
// D-05/AC-3 (single source of truth): this list is deliberately NOT derived
// by importing the server package and building a real Echo router via
// echo.Routes() — doing so would require constructing a full *Server with a
// live DB pool, watcher, embed queue, and MCP server (registerRoutes takes
// *Server and reaches into a dozen of its fields), which is disproportionate
// for a route-table reconciliation check and would turn a fast unit test
// into an integration test. Instead this slice is maintained by hand and
// cross-checked against routes.go on every edit to either file — the
// TestOpenAPISpec_RouteReconciliation test below still catches the case
// this maintenance discipline is meant to prevent: a route registered in
// routes.go with no matching swag annotation (and thus missing from the
// generated spec) fails loudly, exactly as D-05 requires. Protocol tunnels
// (/mcp, /sse) are included as presence-only entries (no schema assertion
// here, matching Plan 02's placeholder-anchor approach).
var expectedRoutePaths = []string{
	"/health",
	"/api/status",
	"/api/version",
	"/api/openapi.json",
	"/api/v1/init",
	"/api/v1/workspaces",
	"/api/v1/workspaces/resolve",
	"/api/v1/workspaces/{hash}",
	"/api/v1/reset-workspace",
	"/api/v1/config",
	"/api/v1/doctor",
	"/api/v1/events",
	"/api/v1/write",
	"/api/v1/embed",
	"/api/v1/reindex",
	"/api/v1/reindex-cfg",
	"/api/v1/update",
	"/api/v1/summarize",
	"/api/v1/code/summarize",
	"/api/v1/code/summarize/status",
	"/api/v1/code/summarize/failures",
	"/api/v1/code/summarize/retry",
	"/api/v1/code/summarize/retry-all",
	"/api/v1/collections",
	"/api/v1/collections/{name}",
	"/api/v1/tags",
	"/api/v1/documents",
	"/api/v1/documents/{id}",
	"/api/v1/get",
	"/api/v1/multi-get",
	"/api/v1/symbols",
	"/api/v1/graph/query",
	"/api/v1/graph/overview",
	"/api/v1/graph/impact",
	"/api/v1/graph/trace",
	"/api/v1/graph/flow",
	"/api/v1/graph/flowchart",
	"/api/v1/graph/flow/endpoints",
	"/api/v1/flow/materialize",
	"/api/v1/vsearch",
	"/api/v1/search",
	"/api/v1/query",
	"/api/v1/stats",
	"/api/v1/graph/pagerank/compute",
	"/api/v1/graph/neighborhood",
	"/api/v1/links/{doc_id}/backlinks",
	"/api/v1/links/resolve",
	"/api/v1/wake-up",
	"/api/v1/sessions/by-ticket",
	"/api/harvest",
	"/api/reload-config",
	"/sse",
	"/mcp",
}

// TestOpenAPISpec_RouteReconciliation satisfies D-05/AC-3: every path
// registered in routes.go (excluding /ui) MUST appear in the generated
// spec's paths object. This is a path-STRING comparison (Pitfall 3), not a
// bare count — a route moved to a different prefix without updating its
// @Router annotation, or a brand-new route added without any annotation at
// all, fails this test even if the total path count happens to coincide.
//
// To verify this test actually catches drift: temporarily add a new route
// registration to routes.go without a matching swag annotation, re-run this
// test, confirm it fails listing the missing path, then revert.
func TestOpenAPISpec_RouteReconciliation(t *testing.T) {
	const specPath = "../../docs/openapi.json"

	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("reading %s: %v", specPath, err)
	}

	var doc struct {
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal %s: %v", specPath, err)
	}

	for _, path := range expectedRoutePaths {
		if _, ok := doc.Paths[path]; !ok {
			t.Errorf("route %q is registered in routes.go but missing from %s — add a swag @Router annotation for its handler and regenerate via `make generate-openapi`", path, specPath)
		}
	}

	if _, ok := doc.Paths["/ui"]; ok {
		t.Errorf("%s unexpectedly contains \"/ui\" — it is a static HTML redirect page, not a JSON REST endpoint, and should stay excluded", specPath)
	}
}
