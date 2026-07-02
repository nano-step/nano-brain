// Command generate-openapi regenerates the committed docs/openapi.json from
// swag annotations and writes the result to stdout. Invoked via
// `make generate-openapi` (see Makefile), which redirects stdout to
// docs/openapi.json. Routes through the same openapigen.Generate function
// the drift-detection test uses, so "regenerate" and "what the test checks"
// can never diverge.
package main

import (
	"fmt"
	"os"

	"github.com/nano-brain/nano-brain/internal/openapigen"
)

func main() {
	out, err := openapigen.Generate(".", "internal/server/doc.go")
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate-openapi:", err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintln(os.Stderr, "generate-openapi: writing output:", err)
		os.Exit(1)
	}
}
