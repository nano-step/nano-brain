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

func TestIntegrationExtractorConsumeSubscribe(t *testing.T) {
	src := []byte(`package main

type Bus struct{}
func (b *Bus) Subscribe(topic string, handler func()) {}

func setupEvents(b *Bus) {
	b.Subscribe("user.created", func() {})
}
`)
	ex := newIntegrationExtractor(t)
	edges, err := ex.ExtractEdges("internal/events.go", src)
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}

	var found bool
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration && e.Metadata["kind"] == "queue_consumer" {
			found = true
			if e.SourceNode != "CONSUME user.created" {
				t.Errorf("expected SourceNode 'CONSUME user.created', got %q", e.SourceNode)
			}
			if e.TargetNode != "internal/events.go::setupEvents" {
				t.Errorf("expected TargetNode 'internal/events.go::setupEvents', got %q", e.TargetNode)
			}
			if e.Metadata["method"] != "Subscribe" {
				t.Errorf("expected method=Subscribe, got %v", e.Metadata["method"])
			}
			if e.Metadata["topic"] != "user.created" {
				t.Errorf("expected topic=user.created, got %v", e.Metadata["topic"])
			}
			if e.Metadata["receiver"] != "b" {
				t.Errorf("expected receiver=b, got %v", e.Metadata["receiver"])
			}
		}
	}
	if !found {
		t.Error("expected queue_consumer edge for .Subscribe call")
	}
}

func TestIntegrationExtractorConsumeConsume(t *testing.T) {
	src := []byte(`package main

type Queue struct{}
func (q *Queue) Consume(name string, handler func()) {}

func setupQueue(q *Queue) {
	q.Consume("orders", func() {})
}
`)
	ex := newIntegrationExtractor(t)
	edges, err := ex.ExtractEdges("internal/queue.go", src)
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}

	var found bool
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration && e.Metadata["kind"] == "queue_consumer" {
			found = true
			if e.Metadata["method"] != "Consume" {
				t.Errorf("expected method=Consume, got %v", e.Metadata["method"])
			}
			if e.Metadata["topic"] != "orders" {
				t.Errorf("expected topic=orders, got %v", e.Metadata["topic"])
			}
		}
	}
	if !found {
		t.Error("expected queue_consumer edge for .Consume call")
	}
}

func TestIntegrationExtractorConsumeListen(t *testing.T) {
	src := []byte(`package main

type Router struct{}
func (r *Router) Listen(path string, handler func()) {}

func setupRoutes(r *Router) {
	r.Listen("/events", func() {})
}
`)
	ex := newIntegrationExtractor(t)
	edges, err := ex.ExtractEdges("internal/router.go", src)
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}

	var found bool
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration && e.Metadata["kind"] == "queue_consumer" {
			found = true
			if e.Metadata["method"] != "Listen" {
				t.Errorf("expected method=Listen, got %v", e.Metadata["method"])
			}
		}
	}
	if !found {
		t.Error("expected queue_consumer edge for .Listen call")
	}
}

func TestIntegrationExtractorConsumeOn(t *testing.T) {
	src := []byte(`package main

type Emitter struct{}
func (e *Emitter) On(event string, handler func()) {}

func registerHandlers(e *Emitter) {
	e.On("message.received", func() {})
}
`)
	ex := newIntegrationExtractor(t)
	edges, err := ex.ExtractEdges("internal/emitter.go", src)
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}

	var found bool
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration && e.Metadata["kind"] == "queue_consumer" {
			found = true
			if e.Metadata["method"] != "On" {
				t.Errorf("expected method=On, got %v", e.Metadata["method"])
			}
		}
	}
	if !found {
		t.Error("expected queue_consumer edge for .On call")
	}
}

func TestIntegrationExtractorConsumeVariableTopic(t *testing.T) {
	src := []byte(`package main

type Bus struct{}
func (b *Bus) Subscribe(topic string, handler func()) {}

func setup(b *Bus, topic string) {
	b.Subscribe(topic, func() {})
}
`)
	ex := newIntegrationExtractor(t)
	edges, err := ex.ExtractEdges("internal/var.go", src)
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}

	var found bool
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration && e.Metadata["kind"] == "queue_consumer" {
			found = true
			topic := e.Metadata["topic"].(string)
			if topic == "" || topic[0] != '<' {
				t.Errorf("expected variable placeholder topic, got %q", topic)
			}
		}
	}
	if !found {
		t.Error("expected queue_consumer edge for variable topic")
	}
}

// TestIntegrationExtractor_TopLevelPublish_Module covers #586: a publish call at
// package top level (e.g. a package-var initializer, outside any func) must
// still produce its topic-coupling edge, attributed to a synthetic "<module>"
// symbol, so the pub/sub stitcher can link it. HTTP/cache calls at top level
// stay dropped (see the JS/Python _NoEdge tests exercising the shared filter).
func TestIntegrationExtractor_TopLevelPublish_Module(t *testing.T) {
	ex := newIntegrationExtractor(t)
	src := []byte("package main\n\nvar _ = bus.Publish(\"channelX\", nil)\n")
	edges, err := ex.ExtractEdges("producer.go", src)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration && e.TargetNode == "Publish:channelX" {
			found = true
			if e.SourceNode != "producer.go::<module>" {
				t.Errorf("SourceNode = %q, want %q", e.SourceNode, "producer.go::<module>")
			}
		}
	}
	if !found {
		t.Fatal("expected a Publish:channelX edge for the top-level publisher")
	}
}
