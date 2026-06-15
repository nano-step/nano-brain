package graph_test

import (
	"os"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/rs/zerolog"
)

func newRailsExtractor(t *testing.T) *graph.RailsExtractor {
	t.Helper()
	logger := zerolog.Nop()
	ex, err := graph.NewRailsExtractor(logger)
	if err != nil {
		t.Fatalf("NewRailsExtractor: %v", err)
	}
	return ex
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}

func TestRailsExtractor_Supports(t *testing.T) {
	ex := newRailsExtractor(t)
	if !ex.Supports(".rb") {
		t.Error("should support .rb")
	}
	for _, ext := range []string{".go", ".ts", ".py", ".java", ".js", ""} {
		if ex.Supports(ext) {
			t.Errorf("should not support %q", ext)
		}
	}
}

func TestRailsExtractor_RequiresFrameworks(t *testing.T) {
	ex := newRailsExtractor(t)
	fws := ex.RequiresFrameworks()
	if len(fws) != 1 || fws[0] != "rails" {
		t.Errorf("expected [rails], got %v", fws)
	}
}

func TestRailsExtractor_NonRoutesFile(t *testing.T) {
	ex := newRailsExtractor(t)
	edges, err := ex.ExtractEdges("app/controllers/users_controller.rb", []byte("class UsersController; end"))
	if err != nil {
		t.Fatal(err)
	}
	if edges != nil {
		t.Errorf("expected nil edges for non-routes.rb file, got %+v", edges)
	}
}

func TestRailsExtractor_RoutesFixture(t *testing.T) {
	ex := newRailsExtractor(t)
	content := readFixture(t, "testdata/rails/routes.rb")

	edges, err := ex.ExtractEdges("config/routes.rb", content)
	if err != nil {
		t.Fatal(err)
	}

	checkEdge(t, edges, "GET /story_statuses", "StoryStatusesController#index")
	checkEdge(t, edges, "POST /story_statuses", "StoryStatusesController#create")
	checkEdge(t, edges, "GET /story_statuses/:id", "StoryStatusesController#show")
	checkEdge(t, edges, "PATCH /story_statuses/:id", "StoryStatusesController#update")
	checkEdge(t, edges, "DELETE /story_statuses/:id", "StoryStatusesController#destroy")
	checkEdge(t, edges, "GET /story_statuses/new", "StoryStatusesController#new")
	checkEdge(t, edges, "GET /story_statuses/:id/edit", "StoryStatusesController#edit")
	checkEdge(t, edges, "POST /api/v1/signup", "Api::V1::TokensController#signup")
	checkEdge(t, edges, "GET /api/v1/payments/upcoming-month", "Api::V1::PaymentsController#upcoming_month")
	checkEdge(t, edges, "GET /api/v1/moments", "Api::V1::MomentsController#index")
	checkEdge(t, edges, "POST /api/v1/moments", "Api::V1::MomentsController#create")
	checkEdge(t, edges, "GET /api/v1/payments", "Api::V1::PaymentsController#index")
	checkEdge(t, edges, "GET /api/v1/payments/billing", "Api::V1::PaymentsController#billing")
	checkEdge(t, edges, "GET /users", "UsersController#index")
	checkEdge(t, edges, "GET /users/token_check", "UsersController#token_check")
	checkEdge(t, edges, "GET /", "HomeController#index")
	checkEdge(t, edges, "POST /users/sign_in", "UsersController#create")
	checkEdge(t, edges, "DELETE /users/sign_out", "UsersController#destroy")
	checkEdge(t, edges, "GET /users", "UsersController#index")
	checkEdge(t, edges, "GET /users/:id", "UsersController#show")

_redirectsSkipped := 0
	for _, e := range edges {
		if strings.Contains(e.TargetNode, "redirect") {
			_redirectsSkipped++
		}
	}
	if _redirectsSkipped != 0 {
		t.Errorf("redirect routes should be skipped, got %d", _redirectsSkipped)
	}
}

func TestRailsExtractor_EdgeMetadata(t *testing.T) {
	ex := newRailsExtractor(t)
	content := readFixture(t, "testdata/rails/routes.rb")

	edges, err := ex.ExtractEdges("config/routes.rb", content)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "POST /api/v1/signup", "Api::V1::TokensController#signup")
	if len(hits) == 0 {
		t.Fatalf("expected http edge for POST /api/v1/signup")
	}
	if hits[0].Metadata["method"] != "POST" {
		t.Errorf("expected Metadata method=POST, got %v", hits[0].Metadata["method"])
	}
	if hits[0].Metadata["path"] != "/api/v1/signup" {
		t.Errorf("expected Metadata path=/api/v1/signup, got %v", hits[0].Metadata["path"])
	}
}

func TestRailsExtractor_SourceFileNormalized(t *testing.T) {
	ex := newRailsExtractor(t)
	content := readFixture(t, "testdata/rails/routes.rb")

	edges, err := ex.ExtractEdges("config/routes.rb", content)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "GET /users", "UsersController#index")
	if len(hits) == 0 {
		t.Fatalf("expected http edge for GET /users")
	}
	if hits[0].SourceFile != "config/routes.rb" {
		t.Errorf("expected SourceFile 'config/routes.rb', got %q", hits[0].SourceFile)
	}
}

func TestRailsExtractor_LineNumbers(t *testing.T) {
	ex := newRailsExtractor(t)
	content := readFixture(t, "testdata/rails/routes.rb")

	edges, err := ex.ExtractEdges("config/routes.rb", content)
	if err != nil {
		t.Fatal(err)
	}

	hits := findEdges(edges, graph.EdgeHTTP, "POST /api/v1/signup", "Api::V1::TokensController#signup")
	if len(hits) == 0 {
		t.Fatalf("expected http edge for POST /api/v1/signup")
	}
	if hits[0].Line != 7 {
		t.Errorf("expected line 7 for POST /api/v1/signup, got %d", hits[0].Line)
	}
}

func checkEdge(t *testing.T, edges []graph.Edge, source, target string) {
	t.Helper()
	hits := findEdges(edges, graph.EdgeHTTP, source, target)
	if len(hits) == 0 {
		t.Fatalf("expected http edge %s -> %s", source, target)
	}
}
