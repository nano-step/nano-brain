package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func newGinExtractor(t *testing.T) *graph.GinExtractor {
	t.Helper()
	ex, err := graph.NewGinExtractor()
	if err != nil {
		t.Fatalf("NewGinExtractor: %v", err)
	}
	return ex
}

func TestGinExtractor_Supports(t *testing.T) {
	ex := newGinExtractor(t)
	if !ex.Supports(".go") {
		t.Error("should support .go")
	}
	for _, ext := range []string{".ts", ".js", ".py", ".rb", ""} {
		if ex.Supports(ext) {
			t.Errorf("should not support %q", ext)
		}
	}
}

func TestGinExtractor_AllVerbs(t *testing.T) {
	src := []byte(`package routes
func setup(r *gin.Engine) {
	r.GET("/a", hGet)
	r.POST("/b", hPost)
	r.PUT("/c", hPut)
	r.DELETE("/d", hDelete)
	r.PATCH("/e", hPatch)
	r.HEAD("/f", hHead)
	r.OPTIONS("/g", hOptions)
}
`)
	ex := newGinExtractor(t)
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

func TestGinExtractor_SingleGroup(t *testing.T) {
	src := []byte(`package routes
func setup(r *gin.Engine) {
	g := r.Group("/api")
	g.GET("/balance", h.GetBalance)
}
`)
	ex := newGinExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /api/balance", "GetBalance")
	if len(hits) == 0 {
		t.Fatalf("expected http edge GET /api/balance → GetBalance; got %+v", edges)
	}
}

func TestGinExtractor_NestedGroups(t *testing.T) {
	src := []byte(`package routes
func setup(r *gin.Engine) {
	g := r.Group("/api")
	v1 := g.Group("/v1")
	v1.POST("/query", h.Query)
}
`)
	ex := newGinExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "POST /api/v1/query", "Query")
	if len(hits) == 0 {
		t.Fatalf("expected http edge POST /api/v1/query → Query; got %+v", edges)
	}
}

func TestGinExtractor_UseMiddleware(t *testing.T) {
	src := []byte(`package routes
func setup(r *gin.Engine) {
	r.Use(logMW)
	r.GET("/topup", HandleTopup)
}
`)
	ex := newGinExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	mwEdges := findEdges(edges, graph.EdgeMiddleware, "logMW", "HandleTopup")
	if len(mwEdges) == 0 {
		t.Fatalf("expected middleware edge logMW → HandleTopup; got %+v", edges)
	}
}

func TestGinExtractor_GroupUseMiddleware(t *testing.T) {
	src := []byte(`package routes
func setup(r *gin.Engine) {
	g := r.Group("/api")
	g.Use(AuthMW)
	g.POST("/topup", HandleTopup)
}
`)
	ex := newGinExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	mwEdges := findEdges(edges, graph.EdgeMiddleware, "AuthMW", "HandleTopup")
	if len(mwEdges) == 0 {
		t.Fatalf("expected middleware edge AuthMW → HandleTopup; got %+v", edges)
	}
}

func TestGinExtractor_FactoryCallHandler(t *testing.T) {
	src := []byte(`package routes
func (s *Server) setup() {
	s.r.GET("/write", handlers.WriteDocument(s.queries))
}
`)
	ex := newGinExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /write", "WriteDocument")
	if len(hits) == 0 {
		t.Fatalf("expected http edge GET /write → WriteDocument; got %+v", edges)
	}
}

func TestGinExtractor_InlineClosure_NoPanic(t *testing.T) {
	src := []byte(`package routes
func setup(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.String(200, "ok")
	})
}
`)
	ex := newGinExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	httpEdges := findEdges(edges, graph.EdgeHTTP, "GET /health", "")
	if len(httpEdges) == 0 {
		t.Fatalf("expected http edge for inline closure; got %+v", edges)
	}
	if httpEdges[0].TargetNode == "" {
		t.Errorf("TargetNode should not be empty for inline closure")
	}
}

func TestGinExtractor_NonLocalReceiver(t *testing.T) {
	src := []byte(`package routes
func (s *Server) setup() {
	s.r.POST("/health", handleHealth)
	s.r.GET("/version", handleVersion)
}
`)
	ex := newGinExtractor(t)
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

func TestGinExtractor_EmptyFile(t *testing.T) {
	ex := newGinExtractor(t)
	edges, err := ex.ExtractEdges("empty.go", []byte("package empty\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected no edges from empty file, got %+v", edges)
	}
}

func TestGinExtractor_AnyMethod(t *testing.T) {
	src := []byte(`package routes
func setup(r *gin.Engine) {
	r.Any("/ping", handlePing)
}
`)
	ex := newGinExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	for _, verb := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"} {
		entryNode := verb + " /ping"
		hits := findEdges(edges, graph.EdgeHTTP, entryNode, "handlePing")
		if len(hits) == 0 {
			t.Errorf("missing http edge %s → handlePing; edges: %+v", entryNode, edges)
		}
	}
}
