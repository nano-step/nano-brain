// Package openapigen builds nano-brain's OpenAPI 3.0 document from swag
// doc-comment annotations placed above REST handler functions.
//
// The pipeline is: swag parses source into a Swagger 2.0 document, then
// kin-openapi's openapi2conv converts it to OpenAPI 3.0 (swag v1 has no
// native 3.0 output; see 12-RESEARCH.md Pitfall 1). Both the
// `make generate-openapi` target and the drift-detection test
// (openapi_gen_test.go) call Generate so "regenerate" and "what the test
// checks" can never diverge.
package openapigen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/swaggo/swag/gen"
)

// nopDebugger discards swag's progress/debug log lines. swag's default
// Debugger implementation writes to stdout via log.Println, which would
// otherwise corrupt the JSON that cmd/generate-openapi streams to stdout.
type nopDebugger struct{}

func (nopDebugger) Printf(string, ...interface{}) {}

// Generate runs swag's AST parser over searchDir (using mainAPIFile as the
// swag "general API info" anchor), converts the resulting Swagger 2.0
// document to OpenAPI 3.0, and returns the deterministically-marshaled
// (indented, stable key order via encoding/json) JSON bytes.
//
// mainAPIFile is resolved relative to searchDir, matching swag's
// gen.Config.MainAPIFile semantics — callers running from different working
// directories (e.g. the test package vs. cmd/generate-openapi) must keep
// both arguments consistent with each other, not just individually valid.
//
// This is a build-time-only operation — nano-brain does not regenerate the
// spec at request time. Callers are expected to write the returned bytes to
// docs/openapi.json (see internal/openapigen/cmd/generate-openapi) or compare
// them against the committed file (see openapi_gen_test.go).
func Generate(searchDir, mainAPIFile string) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "nano-brain-openapigen-*")
	if err != nil {
		return nil, fmt.Errorf("openapigen: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	g := gen.New()
	if err := g.Build(&gen.Config{
		Debugger:    nopDebugger{}, // swag's default logger writes to stdout; silence it so stdout carries only the JSON (see cmd/generate-openapi)
		SearchDir:   searchDir,
		MainAPIFile: mainAPIFile,
		OutputDir:   tmpDir,
		OutputTypes: []string{"json"},
		// ParseDependency stays at its zero value (0 == none). All annotated
		// Request/Response structs (including unexported ones, per
		// Assumption A1) live in the SAME package as the handler that
		// references them, so no cross-package dependency walk is needed.
		// A non-zero value makes swag resolve the full transitive module
		// dependency tree via depth.Tree.Resolve() (AST-parsing every
		// imported package, including large third-party deps like pgx and
		// echo) — empirically confirmed during Plan 12-01 to take minutes
		// and multiple GB of RSS under `go test -race`, without being
		// needed for this repo's same-package annotation pattern.
		ParseInternal: true,
	}); err != nil {
		return nil, fmt.Errorf("openapigen: swag generation failed: %w", err)
	}

	swagger2Bytes, err := os.ReadFile(filepath.Join(tmpDir, "swagger.json"))
	if err != nil {
		return nil, fmt.Errorf("openapigen: reading generated swagger.json: %w", err)
	}

	var doc2 openapi2.T
	if err := json.Unmarshal(swagger2Bytes, &doc2); err != nil {
		return nil, fmt.Errorf("openapigen: unmarshal swagger 2.0 doc: %w", err)
	}

	doc3, err := openapi2conv.ToV3(&doc2)
	if err != nil {
		return nil, fmt.Errorf("openapigen: convert to OpenAPI 3.0: %w", err)
	}

	out, err := json.MarshalIndent(doc3, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("openapigen: marshal OpenAPI 3.0 doc: %w", err)
	}

	return out, nil
}
