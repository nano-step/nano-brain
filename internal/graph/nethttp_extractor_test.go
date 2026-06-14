package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func newNetHTTPExtractor(t *testing.T) *graph.NetHTTPExtractor {
	t.Helper()
	ex, err := graph.NewNetHTTPExtractor()
	if err != nil {
		t.Fatalf("NewNetHTTPExtractor: %v", err)
	}
	return ex
}

func TestNetHTTPExtractor_Supports(t *testing.T) {
	ex := newNetHTTPExtractor(t)
	if !ex.Supports(".go") {
		t.Error("should support .go")
	}
	for _, ext := range []string{".ts", ".js", ".py", ".rb", ""} {
		if ex.Supports(ext) {
			t.Errorf("should not support %q", ext)
		}
	}
}

func TestNetHTTPExtractor_HandleFunc(t *testing.T) {
	src := []byte(`package routes
func setup() {
	http.HandleFunc("/api/topup", handlers.HandleTopup)
}
`)
	ex := newNetHTTPExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "HTTP /api/topup", "HandleTopup")
	if len(hits) == 0 {
		t.Fatalf("expected http edge HTTP /api/topup → HandleTopup; got %+v", edges)
	}
	e := hits[0]
	if e.Language != "go" {
		t.Errorf("Language = %q, want go", e.Language)
	}
	if e.Metadata["method"] != "HTTP" {
		t.Errorf("Metadata.method = %v, want HTTP", e.Metadata["method"])
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

func TestNetHTTPExtractor_Handle(t *testing.T) {
	src := []byte(`package routes
func setup() {
	http.Handle("/api/topup", handlers.HandleTopup)
}
`)
	ex := newNetHTTPExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "HTTP /api/topup", "HandleTopup")
	if len(hits) == 0 {
		t.Fatalf("expected http edge HTTP /api/topup → HandleTopup; got %+v", edges)
	}
}

func TestNetHTTPExtractor_MuxVariable(t *testing.T) {
	src := []byte(`package routes
func setup() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/topup", handlers.HandleTopup)
	mux.Handle("/api/health", handleHealth)
}
`)
	ex := newNetHTTPExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits1 := findEdges(edges, graph.EdgeHTTP, "HTTP /api/topup", "HandleTopup")
	if len(hits1) == 0 {
		t.Errorf("expected http edge HTTP /api/topup → HandleTopup; got %+v", edges)
	}
	hits2 := findEdges(edges, graph.EdgeHTTP, "HTTP /api/health", "handleHealth")
	if len(hits2) == 0 {
		t.Errorf("expected http edge HTTP /api/health → handleHealth; got %+v", edges)
	}
}

func TestNetHTTPExtractor_GorillaMuxMethods(t *testing.T) {
	src := []byte(`package routes
func setup() {
	r := mux.NewRouter()
	r.HandleFunc("/api/topup", handlers.HandleTopup).Methods("GET", "POST")
}
`)
	ex := newNetHTTPExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hitsGET := findEdges(edges, graph.EdgeHTTP, "GET /api/topup", "HandleTopup")
	if len(hitsGET) == 0 {
		t.Errorf("expected http edge GET /api/topup → HandleTopup; got %+v", edges)
	}
	hitsPOST := findEdges(edges, graph.EdgeHTTP, "POST /api/topup", "HandleTopup")
	if len(hitsPOST) == 0 {
		t.Errorf("expected http edge POST /api/topup → HandleTopup; got %+v", edges)
	}

	// Should NOT have a bare "HTTP /api/topup" edge (the Methods call should consume the HandleFunc).
	hitsBare := findEdges(edges, graph.EdgeHTTP, "HTTP /api/topup", "")
	if len(hitsBare) > 0 {
		t.Errorf("unexpected bare HTTP edge when .Methods() is present; got %+v", hitsBare)
	}
}

func TestNetHTTPExtractor_GorillaMuxNoMethods(t *testing.T) {
	src := []byte(`package routes
func setup() {
	r := mux.NewRouter()
	r.HandleFunc("/api/topup", handlers.HandleTopup)
}
`)
	ex := newNetHTTPExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "HTTP /api/topup", "HandleTopup")
	if len(hits) == 0 {
		t.Fatalf("expected http edge HTTP /api/topup → HandleTopup; got %+v", edges)
	}
}

func TestNetHTTPExtractor_InlineClosure_NoPanic(t *testing.T) {
	src := []byte(`package routes
func setup() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
}
`)
	ex := newNetHTTPExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	httpEdges := findEdges(edges, graph.EdgeHTTP, "HTTP /health", "")
	if len(httpEdges) == 0 {
		t.Fatalf("expected http edge for inline closure; got %+v", edges)
	}
	if httpEdges[0].TargetNode == "" {
		t.Errorf("TargetNode should not be empty for inline closure")
	}
}

func TestNetHTTPExtractor_UnresolvableHandler(t *testing.T) {
	src := []byte(`package routes
func setup() {
	http.HandleFunc("/api/topup", handlers.HandleTopup(someArg))
}
`)
	ex := newNetHTTPExtractor(t)
	edges, err := ex.ExtractEdges("routes.go", src)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "HTTP /api/topup", "HandleTopup")
	if len(hits) == 0 {
		t.Fatalf("expected http edge HTTP /api/topup → HandleTopup; got %+v", edges)
	}
}

func TestNetHTTPExtractor_EmptyFile(t *testing.T) {
	ex := newNetHTTPExtractor(t)
	edges, err := ex.ExtractEdges("empty.go", []byte("package empty\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected no edges from empty file, got %+v", edges)
	}
}
