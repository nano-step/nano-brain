package handlers

import (
	"embed"
	"net/http"

	"github.com/labstack/echo/v4"
)

// openapiSpecFS embeds a colocated copy of the committed docs/openapi.json.
// Go's //go:embed directive cannot reach outside its own package directory
// (it cannot embed ../../../docs/openapi.json), so `make generate-openapi`
// writes the canonical docs/openapi.json AND this colocated copy in the same
// step — see the Makefile's generate-openapi recipe. The drift test
// (internal/openapigen/openapi_gen_test.go) only compares the canonical
// docs/openapi.json against a fresh regeneration; TestOpenAPISpecHandler
// below guards that this embedded copy stays byte-identical to it.
//
//go:embed openapi.json
var openapiSpecFS embed.FS

// OpenAPISpec godoc
// @Summary      Serve the generated OpenAPI 3.0 specification
// @Description  Returns the committed OpenAPI 3.0 document describing every REST route in this server. Regenerated via `make generate-openapi`; see docs/openapi.json.
// @Tags         meta
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Router       /api/openapi.json [get]
func OpenAPISpec() echo.HandlerFunc {
	specJSON, _ := openapiSpecFS.ReadFile("openapi.json")
	return func(c echo.Context) error {
		return c.Blob(http.StatusOK, "application/json", specJSON)
	}
}
