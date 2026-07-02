package handlers

// mcpProtocolDoc and sseProtocolDoc are never called — they exist purely as
// swag doc-comment anchors. The /mcp and /sse routes are registered inline in
// internal/server/routes.go via echo.WrapHandler(...), wrapping the MCP SDK's
// own http.Handler; there is no named handler function in this package for
// swag to attach a @Router annotation to (see RESEARCH.md Pitfall 4 / Pattern
// 2). Per D-03, these are protocol tunnels, not JSON REST endpoints — only
// their presence is documented here, with no request/response schema.

//nolint:unused // doc-only anchor for swag, never invoked
//
// mcpProtocolDoc godoc
// @Summary      MCP streamable HTTP transport (protocol endpoint, not a typical JSON REST API — see the MCP spec)
// @Description  Bidirectional MCP protocol transport wrapped via echo.WrapHandler; not a JSON request/response endpoint
// @Tags         protocol
// @Success      200 "protocol response (not JSON — see MCP spec)"
// @Router       /mcp [get]
// @Router       /mcp [post]
// @Router       /mcp [delete]
func mcpProtocolDoc() {}

//nolint:unused // doc-only anchor for swag, never invoked
//
// sseProtocolDoc godoc
// @Summary      Server-Sent Events MCP transport (protocol endpoint, not a typical JSON REST API — see the MCP spec)
// @Description  MCP SSE transport wrapped via echo.WrapHandler; not a JSON request/response endpoint
// @Tags         protocol
// @Success      200 "protocol response (not JSON — see MCP spec)"
// @Router       /sse [get]
// @Router       /sse [post]
func sseProtocolDoc() {}
