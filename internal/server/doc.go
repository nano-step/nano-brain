// Package server hosts nano-brain's REST API server.
//
// This file is the swag "general API info" anchor (swag's -g/MainAPIFile
// target). It carries no executable code — only the doc-comment block below,
// parsed by swaggo/swag into the OpenAPI document's top-level info and
// securityDefinitions sections. See internal/openapigen.Generate and the
// `make generate-openapi` target for how this feeds the committed
// docs/openapi.json.
//
// @title       nano-brain REST API
// @version     1.0
// @description Self-describing OpenAPI 3.0 spec for nano-brain's non-MCP REST API
// @BasePath    /
//
// @securityDefinitions.apikey WorkspaceAuth
// @in                         query
// @name                       workspace
// @description                Workspace hash via query param or JSON body; resolved by workspaceMiddleware.
//
// @securityDefinitions.apikey WorkspaceRegisteredAuth
// @in                         query
// @name                       workspace
// @description                Workspace must be already-registered (write-path gate); enforced by workspaceRegisteredMiddleware.
//
// @securityDefinitions.apikey CSRFToken
// @in                         header
// @name                       X-CSRF-Token
// @description                Required on write endpoints; enforced by middleware.CSRF.
package server
