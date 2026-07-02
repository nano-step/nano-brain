package openapigen_test

import (
	"bytes"
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
