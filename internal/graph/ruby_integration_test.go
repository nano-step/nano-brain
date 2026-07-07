//go:build integration

package graph_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	defaultTestServerURL = "http://localhost:3199"
	testRailWorkspaceHash    = "becf297d74539d99bb858bb91dd79b0611d2e47fd946e92149a1887af02b8d95"
)

func testServerURL() string {
	if v := os.Getenv("NANO_BRAIN_TEST_SERVER_URL"); v != "" {
		return v
	}
	return defaultTestServerURL
}

func testWorkspace() string {
	if v := os.Getenv("NANO_BRAIN_TEST_WORKSPACE"); v != "" {
		return v
	}
	return testRailWorkspaceHash
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// skipIfServerUnreachable skips the test when no live server is reachable at
// testServerURL(). These are acceptance tests: they exercise the full
// extraction pipeline via a running nano-brain server pre-indexed with the
// rails-app fixture (testWorkspace()), not a self-contained fixture — there
// is no server to connect to in a default CI run.
func skipIfServerUnreachable(t *testing.T) {
	t.Helper()
	probe := &http.Client{Timeout: 2 * time.Second}
	resp, err := probe.Get(testServerURL() + "/health")
	if err != nil {
		t.Skipf("skipping: no live server at %s (%v) — this acceptance test needs a running nano-brain server pre-indexed with the rails-app fixture (see NANO_BRAIN_TEST_SERVER_URL / NANO_BRAIN_TEST_WORKSPACE)", testServerURL(), err)
	}
	defer resp.Body.Close()
}

type apiResponse struct {
	Found      bool          `json:"found"`
	Entry      string        `json:"entry"`
	Chain      []flowNodeRes `json:"chain"`
	Externals  []flowNodeRes `json:"externals"`
	Nodes      []flowNodeRes `json:"nodes"`
	Edges      []flowEdgeRes `json:"edges"`
	Mermaid    string        `json:"mermaid,omitempty"`
	Endpoints  []endpointRes `json:"endpoints"`
	Message    string        `json:"message,omitempty"`
}

type flowNodeRes struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type flowEdgeRes struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type endpointRes struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

func apiPost(t *testing.T, path string, body interface{}) apiResponse {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	url := testServerURL() + path
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		t.Fatalf("POST %s returned %d: %s", path, resp.StatusCode, string(bodyBytes))
	}
	var result apiResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, string(bodyBytes))
	}
	return result
}

func apiPostRaw(t *testing.T, path string, body interface{}) (int, []byte) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	url := testServerURL() + path
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, bodyBytes
}

func apiGet(t *testing.T, path string) apiResponse {
	t.Helper()
	url := testServerURL() + path
	resp, err := httpClient.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		t.Fatalf("GET %s returned %d: %s", path, resp.StatusCode, string(bodyBytes))
	}
	var result apiResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, string(bodyBytes))
	}
	return result
}

// TestRailRouteExtraction verifies that route extraction produces the expected
// HTTP edges for the rails-app Rails project.
func TestRailRouteExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfServerUnreachable(t)

	ws := testWorkspace()
	resp := apiGet(t, fmt.Sprintf("/api/v1/graph/flow/endpoints?workspace=%s", ws))

	if len(resp.Endpoints) == 0 {
		t.Fatal("expected at least 1 HTTP endpoint, got 0")
	}
	t.Logf("Found %d HTTP endpoints", len(resp.Endpoints))

	lookup := make(map[string]bool)
	for _, ep := range resp.Endpoints {
		lookup[ep.Source+"|"+ep.Target] = true
	}

	expectedRoutes := []struct {
		source string
		target string
	}{
		{"POST /api/v1/signup", "Api::V1::TokensController#signup"},
		{"GET /api/v1/payments", "Api::V1::PaymentsController#index"},
		{"POST /api/v1/moments", "Api::V1::MomentsController#create"},
		{"GET /api/v1/moments", "Api::V1::MomentsController#index"},
		{"GET /story_statuses", "StoryStatusesController#index"},
		{"POST /story_statuses", "StoryStatusesController#create"},
		{"GET /users", "UsersController#index"},
		{"GET /", "HomeController#index"},
		{"GET /api/v2/stories", "Api::V2::StoriesController#index"},
		{"GET /admin/users", "Admin::UsersController#index"},
	}

	found := 0
	var missing []string
	for _, r := range expectedRoutes {
		if lookup[r.source+"|"+r.target] {
			found++
		} else {
			missing = append(missing, fmt.Sprintf("%s -> %s", r.source, r.target))
		}
	}

	t.Logf("Route extraction: %d/%d expected routes found", found, len(expectedRoutes))
	if len(missing) > 0 {
		t.Logf("Missing routes: %s", strings.Join(missing, ", "))
	}

	if found < len(expectedRoutes)/2 {
		t.Errorf("expected at least %d routes found, got %d (missing: %v)",
			len(expectedRoutes)/2, found, missing)
	}
}

// TestRubyCrossFileResolution verifies that cross-file calls are resolved
// by checking that flow traversal reaches more than just the entry+handler.
func TestRubyCrossFileResolution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfServerUnreachable(t)

	ws := testWorkspace()

	resp := apiPost(t, "/api/v1/graph/flow", map[string]interface{}{
		"workspace":  ws,
		"entry":      "POST /api/v1/signup",
		"max_depth":  8,
		"format":     "json",
	})

	if !resp.Found {
		t.Fatalf("expected found=true for POST /api/v1/signup, got false")
	}

	if len(resp.Nodes) < 2 {
		t.Errorf("expected at least 2 nodes in flow, got %d", len(resp.Nodes))
	}

	handlerFound := false
	for _, n := range resp.Nodes {
		if n.Role == "handler" {
			handlerFound = true
			if !strings.Contains(n.Name, "TokensController") {
				t.Errorf("expected handler to be TokensController, got %q", n.Name)
			}
		}
	}
	if !handlerFound {
		t.Error("expected a handler node in the flow")
	}

	httpEdgeFound := false
	for _, e := range resp.Edges {
		if e.Kind == "http" {
			httpEdgeFound = true
			if e.From != "POST /api/v1/signup" {
				t.Errorf("expected HTTP edge from 'POST /api/v1/signup', got %q", e.From)
			}
		}
	}
	if !httpEdgeFound {
		t.Error("expected an HTTP edge in the flow")
	}
}

// TestRubyReconcileEdges verifies that reconcile edges bridge
// Controller#action → file.rb::method by checking the flow includes func nodes.
func TestRubyReconcileEdges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfServerUnreachable(t)

	ws := testWorkspace()

	entries := []string{
		"POST /api/v1/signup",
		"GET /api/v1/payments",
		"GET /story_statuses",
		"GET /users",
	}

	for _, entry := range entries {
		t.Run(entry, func(t *testing.T) {
			resp := apiPost(t, "/api/v1/graph/flow", map[string]interface{}{
				"workspace": ws,
				"entry":     entry,
				"max_depth": 5,
				"format":    "json",
			})

			if !resp.Found {
				t.Fatalf("expected found=true, got false")
			}

			hasFunc := false
			for _, n := range resp.Nodes {
				if n.Role == "func" {
					hasFunc = true
				}
			}
			if !hasFunc {
				t.Errorf("expected at least one func node (reconcile bridge) for %s", entry)
			}
		})
	}
}

// TestRubyClassIndex verifies that class→file mapping works for the
// rails-app controllers and models.
func TestRubyClassIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfServerUnreachable(t)

	ws := testWorkspace()

	controllerTests := []struct {
		entry          string
		expectedPrefix string
	}{
		{"POST /api/v1/signup", "Api::V1::TokensController"},
		{"GET /api/v1/payments", "Api::V1::PaymentsController"},
		{"POST /api/v1/moments", "Api::V1::MomentsController"},
		{"GET /story_statuses", "StoryStatusesController"},
		{"GET /users", "UsersController"},
		{"GET /api/v2/stories", "Api::V2::StoriesController"},
		{"GET /admin/users", "Admin::UsersController"},
	}

	for _, tt := range controllerTests {
		t.Run(tt.entry, func(t *testing.T) {
			resp := apiPost(t, "/api/v1/graph/flow", map[string]interface{}{
				"workspace": ws,
				"entry":     tt.entry,
				"max_depth": 3,
				"format":    "json",
			})

			if !resp.Found {
				t.Fatalf("expected found=true for %s", tt.entry)
			}

			handlerFound := false
			for _, n := range resp.Nodes {
				if n.Role == "handler" && strings.HasPrefix(n.Name, tt.expectedPrefix) {
					handlerFound = true
				}
			}
			if !handlerFound {
				t.Errorf("expected handler with prefix %q for %s", tt.expectedPrefix, tt.entry)
				for _, n := range resp.Nodes {
					t.Logf("  node: %s [%s]", n.Name, n.Role)
				}
			}
		})
	}
}

// TestRubyFlowEndToEnd requests flow for POST /api/v1/signup and verifies
// that the full pipeline produces 3+ nodes in the result.
func TestRubyFlowEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfServerUnreachable(t)

	ws := testWorkspace()

	resp := apiPost(t, "/api/v1/graph/flow", map[string]interface{}{
		"workspace":  ws,
		"entry":      "POST /api/v1/signup",
		"max_depth":  8,
		"format":     "mermaid",
	})

	if !resp.Found {
		t.Fatalf("expected found=true for POST /api/v1/signup")
	}

	if len(resp.Nodes) < 3 {
		t.Errorf("expected at least 3 nodes in flow, got %d", len(resp.Nodes))
		for _, n := range resp.Nodes {
			t.Logf("  node: %s [%s]", n.Name, n.Role)
		}
	}

	chainNames := make(map[string]bool)
	for _, n := range resp.Chain {
		chainNames[n.Name] = true
	}

	if !chainNames["POST /api/v1/signup"] {
		t.Error("expected 'POST /api/v1/signup' in chain")
	}

	handlerFound := false
	for _, n := range resp.Chain {
		if strings.Contains(n.Name, "TokensController") && n.Role == "handler" {
			handlerFound = true
		}
	}
	if !handlerFound {
		t.Error("expected TokensController handler in chain")
	}

	if len(resp.Edges) == 0 {
		t.Error("expected at least 1 edge in flow")
	}

	if resp.Mermaid == "" {
		t.Error("expected non-empty mermaid diagram")
	} else {
		t.Logf("Mermaid diagram:\n%s", resp.Mermaid)
	}
}
