package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/rs/zerolog"
)

func newNuxtExtractor(t *testing.T) *graph.NuxtExtractor {
	t.Helper()
	logger := zerolog.Nop()
	ex, err := graph.NewNuxtExtractor(logger)
	if err != nil {
		t.Fatalf("NewNuxtExtractor: %v", err)
	}
	return ex
}

func TestNuxtExtractor_Supports(t *testing.T) {
	ex := newNuxtExtractor(t)
	for _, ext := range []string{".vue", ".ts", ".js"} {
		if !ex.Supports(ext) {
			t.Errorf("should support %q", ext)
		}
	}
	for _, ext := range []string{".go", ".py", ".rb", ".java", ""} {
		if ex.Supports(ext) {
			t.Errorf("should not support %q", ext)
		}
	}
}

func TestNuxtExtractor_RequiresFrameworks(t *testing.T) {
	ex := newNuxtExtractor(t)
	fws := ex.RequiresFrameworks()
	if len(fws) != 1 || fws[0] != "nuxt" {
		t.Errorf("expected [nuxt], got %v", fws)
	}
}

func TestNuxtExtractor_IndexPage(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("pages/index.vue", []byte("<template><p>Home</p></template>"))
	if err != nil {
		t.Fatal(err)
	}
	hits := findEdges(edges, graph.EdgeHTTP, "GET /", "pages/index.vue")
	if len(hits) == 0 {
		t.Fatalf("expected http edge GET / -> pages/index.vue; got %+v", edges)
	}
	if hits[0].Metadata["method"] != "GET" {
		t.Errorf("expected method GET, got %v", hits[0].Metadata["method"])
	}
	if hits[0].Metadata["path"] != "/" {
		t.Errorf("expected path /, got %v", hits[0].Metadata["path"])
	}
}

func TestNuxtExtractor_StaticPage(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("pages/about.vue", []byte("<template><p>About</p></template>"))
	if err != nil {
		t.Fatal(err)
	}
	hits := findEdges(edges, graph.EdgeHTTP, "GET /about", "pages/about.vue")
	if len(hits) == 0 {
		t.Fatalf("expected http edge GET /about -> pages/about.vue; got %+v", edges)
	}
}

func TestNuxtExtractor_NestedIndexPage(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("pages/users/index.vue", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	hits := findEdges(edges, graph.EdgeHTTP, "GET /users", "pages/users/index.vue")
	if len(hits) == 0 {
		t.Fatalf("expected http edge GET /users -> pages/users/index.vue; got %+v", edges)
	}
}

func TestNuxtExtractor_DynamicParam(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("pages/users/[id].vue", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	hits := findEdges(edges, graph.EdgeHTTP, "GET /users/:id", "pages/users/[id].vue")
	if len(hits) == 0 {
		t.Fatalf("expected http edge GET /users/:id -> pages/users/[id].vue; got %+v", edges)
	}
}

func TestNuxtExtractor_NestedDynamicParam(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("pages/users/[id]/posts.vue", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	hits := findEdges(edges, graph.EdgeHTTP, "GET /users/:id/posts", "pages/users/[id]/posts.vue")
	if len(hits) == 0 {
		t.Fatalf("expected http edge GET /users/:id/posts -> pages/users/[id]/posts.vue; got %+v", edges)
	}
}

func TestNuxtExtractor_DoubleDynamicParam(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("pages/users/[id]/posts/[postId].vue", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	hits := findEdges(edges, graph.EdgeHTTP, "GET /users/:id/posts/:postId", "pages/users/[id]/posts/[postId].vue")
	if len(hits) == 0 {
		t.Fatalf("expected http edge GET /users/:id/posts/:postId -> pages/users/[id]/posts/[postId].vue; got %+v", edges)
	}
}

func TestNuxtExtractor_APIRouteGet(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("server/api/users.get.ts", []byte(`export default defineEventHandler(() => {})`))
	if err != nil {
		t.Fatal(err)
	}
	hits := findEdges(edges, graph.EdgeHTTP, "GET /api/users", "server/api/users.get.ts")
	if len(hits) == 0 {
		t.Fatalf("expected http edge GET /api/users -> server/api/users.get.ts; got %+v", edges)
	}
	if hits[0].Metadata["method"] != "GET" {
		t.Errorf("expected method GET, got %v", hits[0].Metadata["method"])
	}
	if hits[0].Metadata["path"] != "/api/users" {
		t.Errorf("expected path /api/users, got %v", hits[0].Metadata["path"])
	}
}

func TestNuxtExtractor_APIRoutePut(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("server/api/users/[id].put.ts", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	hits := findEdges(edges, graph.EdgeHTTP, "PUT /api/users/:id", "server/api/users/[id].put.ts")
	if len(hits) == 0 {
		t.Fatalf("expected http edge PUT /api/users/:id -> server/api/users/[id].put.ts; got %+v", edges)
	}
}

func TestNuxtExtractor_APIRouteMultiMethod(t *testing.T) {
	tests := []struct {
		file    string
		method  string
		route   string
		handler string
	}{
		{"server/api/users.post.ts", "POST", "/api/users", "server/api/users.post.ts"},
		{"server/api/users.delete.ts", "DELETE", "/api/users", "server/api/users.delete.ts"},
		{"server/api/users.patch.ts", "PATCH", "/api/users", "server/api/users.patch.ts"},
		{"server/api/health.head.ts", "HEAD", "/api/health", "server/api/health.head.ts"},
		{"server/api/status.options.ts", "OPTIONS", "/api/status", "server/api/status.options.ts"},
	}
	ex := newNuxtExtractor(t)
	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			edges, err := ex.ExtractEdges(tc.file, []byte{})
			if err != nil {
				t.Fatal(err)
			}
			hits := findEdges(edges, graph.EdgeHTTP, tc.method+" "+tc.route, tc.handler)
			if len(hits) == 0 {
				t.Fatalf("expected http edge %s %s -> %s; got %+v", tc.method, tc.route, tc.handler, edges)
			}
		})
	}
}

func TestNuxtExtractor_NonNuxtFile(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("src/components/Button.vue", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected no edges for non-pages file, got %+v", edges)
	}
}

func TestNuxtExtractor_NonMatchingExtension(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("pages/readme.md", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	if edges != nil {
		t.Errorf("expected nil edges for unsupported extension, got %+v", edges)
	}
}

func TestNuxtExtractor_SubdirectoryPages(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("src/pages/index.vue", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	hits := findEdges(edges, graph.EdgeHTTP, "GET /", "pages/index.vue")
	if len(hits) == 0 {
		t.Fatalf("expected http edge for pages in subdir; got %+v", edges)
	}
}

func TestNuxtExtractor_SubdirectoryServerAPI(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("my-app/server/api/users.get.ts", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	hits := findEdges(edges, graph.EdgeHTTP, "GET /api/users", "server/api/users.get.ts")
	if len(hits) == 0 {
		t.Fatalf("expected http edge for server/api in subdir; got %+v", edges)
	}
}

func TestNuxtExtractor_MetadataField(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("pages/users/[id].vue", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	hits := findEdges(edges, graph.EdgeHTTP, "GET /users/:id", "pages/users/[id].vue")
	if len(hits) == 0 {
		t.Fatalf("expected http edge; got %+v", edges)
	}
	if hits[0].Metadata == nil {
		t.Fatalf("expected non-nil Metadata; got nil")
	}
	if hits[0].Metadata["method"] != "GET" {
		t.Errorf("expected Metadata method=GET, got %v", hits[0].Metadata["method"])
	}
	if hits[0].Metadata["path"] != "/users/:id" {
		t.Errorf("expected Metadata path=/users/:id, got %v", hits[0].Metadata["path"])
	}
}

func TestNuxtExtractor_SourceFileNormalized(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("pages/about.vue", nil)
	if err != nil {
		t.Fatal(err)
	}
	hits := findEdges(edges, graph.EdgeHTTP, "GET /about", "pages/about.vue")
	if len(hits) == 0 {
		t.Fatalf("expected http edge; got %+v", edges)
	}
	if len(hits) > 0 && hits[0].SourceFile != "pages/about.vue" {
		t.Errorf("expected SourceFile 'pages/about.vue', got %q", hits[0].SourceFile)
	}
}

func TestNuxtExtractor_PagesInSubdir(t *testing.T) {
	ex := newNuxtExtractor(t)
	edges, err := ex.ExtractEdges("project/pages/users/[id].vue", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	hits := findEdges(edges, graph.EdgeHTTP, "GET /users/:id", "pages/users/[id].vue")
	if len(hits) == 0 {
		t.Fatalf("expected http edge for pages in subdir; got %+v", edges)
	}
}
