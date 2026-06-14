package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func newEchoExtractor(t *testing.T) *graph.EchoRouteExtractor {
	t.Helper()
	ex, err := graph.NewEchoRouteExtractor()
	if err != nil {
		t.Fatalf("NewEchoRouteExtractor: %v", err)
	}
	return ex
}

func TestEchoRouteExtractor_Supports(t *testing.T) {
	ex := newEchoExtractor(t)
	if !ex.Supports(".go") {
		t.Error("should support .go")
	}
	for _, ext := range []string{".ts", ".js", ".py", ".rb", ""} {
		if ex.Supports(ext) {
			t.Errorf("should not support %q", ext)
		}
	}
}

// findEdges returns all edges matching the given kind, source, and target.
// Pass "" to skip a field check.
func findEdges(edges []graph.Edge, kind graph.EdgeKind, source, target string) []graph.Edge {
	var out []graph.Edge
	for _, e := range edges {
		if kind != "" && e.Kind != kind {
			continue
		}
		if source != "" && e.SourceNode != source {
			continue
		}
		if target != "" && e.TargetNode != target {
			continue
		}
		out = append(out, e)
	}
	return out
}

func TestEchoRouteExtractor_PlainRoute(t *testing.T) {
	src := []byte(`package routes
func setup(e *echo.Echo) {
	e.POST("/api/topup", handlers.HandleTopup)
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "POST /api/topup", "HandleTopup")
	if len(hits) == 0 {
		t.Fatalf("expected http edge POST /api/topup → HandleTopup; got %+v", edges)
	}
	e := hits[0]
	if e.Language != "go" {
		t.Errorf("Language = %q, want go", e.Language)
	}
	if e.Metadata["method"] != "POST" {
		t.Errorf("Metadata.method = %v, want POST", e.Metadata["method"])
	}
	if e.Metadata["path"] != "/api/topup" {
		t.Errorf("Metadata.path = %v, want /api/topup", e.Metadata["path"])
	}
	if e.SourceFile != "routes.go" {
		t.Errorf("SourceFile = %q, want routes.go", e.SourceFile)
	}
	if e.Line == 0 {
		t.Error("Line should be non-zero")
	}
}

func TestEchoRouteExtractor_AllVerbs(t *testing.T) {
	src := []byte(`package routes
func setup(e *echo.Echo) {
	e.GET("/a", hGet)
	e.POST("/b", hPost)
	e.PUT("/c", hPut)
	e.DELETE("/d", hDelete)
	e.PATCH("/e", hPatch)
	e.HEAD("/f", hHead)
	e.OPTIONS("/g", hOptions)
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct{ verb, path, handler string }{
		{"GET", "/a", "hGet"},
		{"POST", "/b", "hPost"},
		{"PUT", "/c", "hPut"},
		{"DELETE", "/d", "hDelete"},
		{"PATCH", "/e", "hPatch"},
		{"HEAD", "/f", "hHead"},
		{"OPTIONS", "/g", "hOptions"},
	}
	for _, tc := range cases {
		entryNode := tc.verb + " " + tc.path
		hits := findEdges(edges, graph.EdgeHTTP, entryNode, tc.handler)
		if len(hits) == 0 {
			t.Errorf("missing http edge %s → %s; edges: %+v", entryNode, tc.handler, edges)
		}
	}
}

func TestEchoRouteExtractor_SingleGroup(t *testing.T) {
	src := []byte(`package routes
func setup(e *echo.Echo) {
	g := e.Group("/api")
	g.GET("/balance", h.GetBalance)
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /api/balance", "GetBalance")
	if len(hits) == 0 {
		t.Fatalf("expected http edge GET /api/balance → GetBalance; got %+v", edges)
	}
}

func TestEchoRouteExtractor_NestedGroups(t *testing.T) {
	src := []byte(`package routes
func setup(e *echo.Echo) {
	g := e.Group("/api")
	v1 := g.Group("/v1")
	v1.POST("/query", h.Query)
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "POST /api/v1/query", "Query")
	if len(hits) == 0 {
		t.Fatalf("expected http edge POST /api/v1/query → Query; got %+v", edges)
	}
}

func TestEchoRouteExtractor_EmptyPrefixChain(t *testing.T) {
	// Mirrors the real nano-brain routes.go pattern:
	// api := s.echo.Group("/api/v1")
	// data := api.Group("")
	// write := data.Group("")
	// write.POST("/write", handlers.WriteDocument(...))
	src := []byte(`package routes
func (s *Server) registerRoutes() {
	api := s.echo.Group("/api/v1")
	data := api.Group("")
	write := data.Group("")
	write.POST("/write", handlers.WriteDocument(s.queries, s.db))
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "POST /api/v1/write", "WriteDocument")
	if len(hits) == 0 {
		t.Fatalf("expected http edge POST /api/v1/write → WriteDocument; got %+v", edges)
	}
}

func TestEchoRouteExtractor_GlobalUseMiddleware(t *testing.T) {
	src := []byte(`package routes
func setup(e *echo.Echo) {
	e.Use(logMW)
	e.POST("/topup", HandleTopup)
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	mwEdges := findEdges(edges, graph.EdgeMiddleware, "logMW", "HandleTopup")
	if len(mwEdges) == 0 {
		t.Fatalf("expected middleware edge logMW → HandleTopup; got %+v", edges)
	}
}

func TestEchoRouteExtractor_GroupUseMiddleware(t *testing.T) {
	src := []byte(`package routes
func setup(e *echo.Echo) {
	g := e.Group("/api")
	g.Use(AuthMW)
	g.POST("/topup", HandleTopup)
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	mwEdges := findEdges(edges, graph.EdgeMiddleware, "AuthMW", "HandleTopup")
	if len(mwEdges) == 0 {
		t.Fatalf("expected middleware edge AuthMW → HandleTopup; got %+v", edges)
	}
}

func TestEchoRouteExtractor_PerRouteMiddleware(t *testing.T) {
	src := []byte(`package routes
func setup(e *echo.Echo) {
	g := e.Group("/api")
	g.POST("/topup", h.HandleTopup, AuthMW)
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	mwEdges := findEdges(edges, graph.EdgeMiddleware, "AuthMW", "HandleTopup")
	if len(mwEdges) == 0 {
		t.Fatalf("expected per-route middleware edge AuthMW → HandleTopup; got %+v", edges)
	}
}

func TestEchoRouteExtractor_FactoryCallHandler(t *testing.T) {
	// handlers.WriteDocument(s.queries, s.db) → target = "WriteDocument"
	src := []byte(`package routes
func (s *Server) setup() {
	s.echo.POST("/write", handlers.WriteDocument(s.queries, s.db))
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "POST /write", "WriteDocument")
	if len(hits) == 0 {
		t.Fatalf("expected http edge POST /write → WriteDocument; got %+v", edges)
	}
}

func TestEchoRouteExtractor_UnqualifiedFactoryCallHandler(t *testing.T) {
	// WriteDocument(...) (no package prefix) → target = "WriteDocument"
	src := []byte(`package routes
func setup(e *echo.Echo) {
	e.POST("/write", WriteDocument(queries))
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "POST /write", "WriteDocument")
	if len(hits) == 0 {
		t.Fatalf("expected http edge POST /write → WriteDocument; got %+v", edges)
	}
}

func TestEchoRouteExtractor_MethodValueHandler(t *testing.T) {
	// h.HandleGraph (method value, not a call) → target = "HandleGraph"
	src := []byte(`package routes
func setup(e *echo.Echo) {
	e.GET("/graph", h.HandleGraph)
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /graph", "HandleGraph")
	if len(hits) == 0 {
		t.Fatalf("expected http edge GET /graph → HandleGraph; got %+v", edges)
	}
}

func TestEchoRouteExtractor_BareIdentifierHandler(t *testing.T) {
	src := []byte(`package routes
func setup(e *echo.Echo) {
	e.POST("/topup", HandleTopup)
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "POST /topup", "HandleTopup")
	if len(hits) == 0 {
		t.Fatalf("expected http edge POST /topup → HandleTopup; got %+v", edges)
	}
}

func TestEchoRouteExtractor_InlineClosureHandler_NoPanic(t *testing.T) {
	src := []byte(`package routes
func setup(e *echo.Echo) {
	e.GET("/health", func(c echo.Context) error {
		return c.String(200, "ok")
	})
}
`)
	ex := newEchoExtractor(t)
	// Must not panic.
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	// An http edge is still emitted (with synthetic target).
	httpEdges := findEdges(edges, graph.EdgeHTTP, "GET /health", "")
	if len(httpEdges) == 0 {
		t.Fatalf("expected http edge for inline closure; got %+v", edges)
	}
	// Target must not be empty.
	if httpEdges[0].TargetNode == "" {
		t.Errorf("TargetNode should not be empty for inline closure")
	}
}

func TestEchoRouteExtractor_NonLocalReceiver(t *testing.T) {
	// s.echo.POST(...) — receiver is a struct field, not a local variable.
	src := []byte(`package routes
func (s *Server) setup() {
	s.echo.POST("/health", handleHealth)
	s.echo.GET("/version", handleVersion)
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits1 := findEdges(edges, graph.EdgeHTTP, "POST /health", "handleHealth")
	if len(hits1) == 0 {
		t.Errorf("expected http edge POST /health → handleHealth; got %+v", edges)
	}
	hits2 := findEdges(edges, graph.EdgeHTTP, "GET /version", "handleVersion")
	if len(hits2) == 0 {
		t.Errorf("expected http edge GET /version → handleVersion; got %+v", edges)
	}
}

func TestEchoRouteExtractor_EmptyFile(t *testing.T) {
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("empty.go", []byte("package empty\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected no edges from empty file, got %+v", edges)
	}
}

func TestEchoRouteExtractor_MetadataFields(t *testing.T) {
	src := []byte(`package routes
func setup(e *echo.Echo) {
	e.PUT("/users/:id", updateUser)
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "PUT /users/:id", "updateUser")
	if len(hits) == 0 {
		t.Fatalf("expected edge PUT /users/:id → updateUser; got %+v", edges)
	}
	e := hits[0]
	if e.Metadata["method"] != "PUT" {
		t.Errorf("metadata method = %v, want PUT", e.Metadata["method"])
	}
	if e.Metadata["path"] != "/users/:id" {
		t.Errorf("metadata path = %v, want /users/:id", e.Metadata["path"])
	}
}

func TestEchoRouteExtractor_MultipleGroupUseAndRoute(t *testing.T) {
	// Combination: group Use + per-route MW + factory handler.
	src := []byte(`package routes
func (s *Server) registerRoutes() {
	api := s.echo.Group("/api/v1")
	api.Use(AuthMW)
	api.POST("/write", handlers.WriteDocument(s.queries), csrfMW)
}
`)
	ex := newEchoExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	// HTTP edge
	httpHits := findEdges(edges, graph.EdgeHTTP, "POST /api/v1/write", "WriteDocument")
	if len(httpHits) == 0 {
		t.Fatalf("expected http edge; got %+v", edges)
	}

	// Group-scoped middleware
	groupMW := findEdges(edges, graph.EdgeMiddleware, "AuthMW", "WriteDocument")
	if len(groupMW) == 0 {
		t.Errorf("expected group middleware edge AuthMW → WriteDocument; got %+v", edges)
	}

	// Per-route middleware
	routeMW := findEdges(edges, graph.EdgeMiddleware, "csrfMW", "WriteDocument")
	if len(routeMW) == 0 {
		t.Errorf("expected per-route middleware edge csrfMW → WriteDocument; got %+v", edges)
	}
}
