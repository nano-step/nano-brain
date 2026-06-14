package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func newIntegrationExtractor(t *testing.T) *graph.IntegrationExtractor {
	t.Helper()
	ex, err := graph.NewIntegrationExtractor()
	if err != nil {
		t.Fatalf("NewIntegrationExtractor: %v", err)
	}
	return ex
}

func TestIntegrationExtractorSupports(t *testing.T) {
	ex := newIntegrationExtractor(t)
	if !ex.Supports(".go") {
		t.Error("expected Supports('.go') = true")
	}
	if ex.Supports(".ts") || ex.Supports(".py") {
		t.Error("expected Supports to be false for non-Go files")
	}
}

func TestIntegrationExtractorHTTPNewRequest(t *testing.T) {
	src := []byte(`package main

import "net/http"

func callAPI() {
	req, _ := http.NewRequest("POST", "https://api.example.com/v1/users", nil)
	_ = req
}
`)
	ex := newIntegrationExtractor(t)
	edges, err := ex.ExtractEdges("internal/foo/client.go", src)
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}

	var found bool
	for _, e := range edges {
		if e.Kind != graph.EdgeIntegration {
			continue
		}
		if e.SourceNode == "" {
			t.Error("integration edge has empty SourceNode")
		}
		found = true
		meta := e.Metadata
		if meta["kind"] != "http_call" {
			t.Errorf("expected kind=http_call, got %v", meta["kind"])
		}
	}
	if !found {
		t.Errorf("expected at least one integration edge, got none; edges: %v", edges)
	}
}

func TestIntegrationExtractorHTTPNewRequestWithContext(t *testing.T) {
	src := []byte(`package main

import (
	"context"
	"net/http"
)

func callService(ctx context.Context) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://service.internal/health", nil)
	_ = req
}
`)
	ex := newIntegrationExtractor(t)
	edges, err := ex.ExtractEdges("internal/client.go", src)
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}

	var found bool
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			found = true
			if e.Metadata["kind"] != "http_call" {
				t.Errorf("expected kind=http_call, got %v", e.Metadata["kind"])
			}
		}
	}
	if !found {
		t.Error("expected integration edge for http.NewRequestWithContext")
	}
}

func TestIntegrationExtractorHTTPGet(t *testing.T) {
	src := []byte(`package main

import "net/http"

func ping() {
	resp, _ := http.Get("https://example.com/ping")
	_ = resp
}
`)
	ex := newIntegrationExtractor(t)
	edges, err := ex.ExtractEdges("internal/ping.go", src)
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}

	var found bool
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			found = true
		}
	}
	if !found {
		t.Error("expected integration edge for http.Get")
	}
}

func TestIntegrationExtractorPublish(t *testing.T) {
	src := []byte(`package main

type Queue struct{}
func (q *Queue) Publish(topic string, msg []byte) {}

func sendEvent(q *Queue) {
	q.Publish("user.created", []byte("{}"))
}
`)
	ex := newIntegrationExtractor(t)
	edges, err := ex.ExtractEdges("internal/queue/producer.go", src)
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}

	var found bool
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			found = true
			if e.Metadata["kind"] != "queue_publish" {
				t.Errorf("expected kind=queue_publish, got %v", e.Metadata["kind"])
			}
		}
	}
	if !found {
		t.Error("expected integration edge for .Publish call")
	}
}

func TestIntegrationExtractorNoEdgesForInternalCalls(t *testing.T) {
	src := []byte(`package main

func helper() {}

func main() {
	helper()
}
`)
	ex := newIntegrationExtractor(t)
	edges, err := ex.ExtractEdges("main.go", src)
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			t.Errorf("unexpected integration edge for plain function call: %+v", e)
		}
	}
}

func TestIntegrationExtractorLineNumbers(t *testing.T) {
	src := []byte(`package main

import "net/http"

func callRemote() {
	req, _ := http.NewRequest("POST", "https://remote.example.com/api", nil)
	_ = req
}
`)
	ex := newIntegrationExtractor(t)
	edges, err := ex.ExtractEdges("internal/remote.go", src)
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration && e.Line == 0 {
			t.Errorf("integration edge should have non-zero line number, got Line=0")
		}
	}
}

func TestIntegrationExtractorSourceNode(t *testing.T) {
	src := []byte(`package main

import "net/http"

func outerFunc() {
	http.Get("https://example.com")
}
`)
	ex := newIntegrationExtractor(t)
	edges, err := ex.ExtractEdges("internal/foo.go", src)
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			if e.SourceNode == "" {
				t.Error("SourceNode should not be empty")
			}
			// SourceNode should include the enclosing function name.
			if e.SourceNode != "internal/foo.go::outerFunc" {
				t.Errorf("expected SourceNode 'internal/foo.go::outerFunc', got %q", e.SourceNode)
			}
		}
	}
}
