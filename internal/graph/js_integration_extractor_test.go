package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func TestJSIntegrationExtractor_Supports(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		ext  string
		want bool
	}{
		{".js", true},
		{".jsx", true},
		{".ts", true},
		{".tsx", true},
		{".go", false},
		{".py", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := ex.Supports(tt.ext); got != tt.want {
			t.Errorf("Supports(%q) = %v, want %v", tt.ext, got, tt.want)
		}
	}
}

func TestJSIntegrationExtractor_EmptyFile(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}
	edges, err := ex.ExtractEdges("empty.js", []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges from empty file, got %d", len(edges))
	}
}

func TestJSIntegrationExtractor_SyntaxError(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("function broken( { return }")
	edges, err := ex.ExtractEdges("broken.js", src)
	if err != nil {
		t.Fatal("should not error on partial parse:", err)
	}
	_ = edges
}

func TestJSIntegrationExtractor_Fetch(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		src      string
		ext      string
		wantURL  string
		wantMeta map[string]any
	}{
		{
			name:    "fetch literal url",
			src:     `function load() { fetch("https://api.example.com/data"); }`,
			ext:     ".js",
			wantURL: "GET https://api.example.com/data",
			wantMeta: map[string]any{"kind": "http_call", "method": "GET", "url": "https://api.example.com/data"},
		},
		{
			name:    "fetch with variable",
			src:     `function load() { const url = "/api/data"; fetch(url); }`,
			ext:     ".js",
			wantURL: "GET <var:url>",
			wantMeta: map[string]any{"kind": "http_call", "method": "GET", "url": "<var:url>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges("test"+tt.ext, []byte(tt.src))
			if err != nil {
				t.Fatal(err)
			}
			var matching []graph.Edge
			for _, e := range edges {
				if e.Kind == graph.EdgeIntegration {
					matching = append(matching, e)
				}
			}
			if len(matching) == 0 {
				t.Fatal("expected at least one EdgeIntegration edge")
			}
			e := matching[0]
			if e.TargetNode != tt.wantURL {
				t.Errorf("TargetNode = %q, want %q", e.TargetNode, tt.wantURL)
			}
			if !metadataContains(e.Metadata, tt.wantMeta) {
				t.Errorf("Metadata = %v, want to contain %v", e.Metadata, tt.wantMeta)
			}
			if e.SourceNode != "test"+tt.ext+"::load" {
				t.Errorf("SourceNode = %q, want %q", e.SourceNode, "test"+tt.ext+"::load")
			}
		})
	}
}

func TestJSIntegrationExtractor_AxiosShorthand(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		src       string
		ext       string
		wantURL   string
		wantMeta  map[string]any
		wantSrc   string
	}{
		{
			name:    "axios.get literal url",
			src:     `function fetchUsers() { axios.get("https://api.example.com/users"); }`,
			ext:     ".ts",
			wantURL: "GET https://api.example.com/users",
			wantMeta: map[string]any{"kind": "http_call", "method": "GET", "url": "https://api.example.com/users"},
		},
		{
			name:    "axios.post literal url",
			src:     `function createUser() { axios.post("https://api.example.com/users", {name:"Alice"}); }`,
			ext:     ".js",
			wantURL: "POST https://api.example.com/users",
			wantMeta: map[string]any{"kind": "http_call", "method": "POST", "url": "https://api.example.com/users"},
		},
		{
			name:    "httpClient.delete",
			src:     `function remove() { httpClient.delete("/api/items/1"); }`,
			ext:     ".js",
			wantURL: "DELETE /api/items/1",
			wantMeta: map[string]any{"kind": "http_call", "method": "DELETE", "url": "/api/items/1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges("test"+tt.ext, []byte(tt.src))
			if err != nil {
				t.Fatal(err)
			}
			var matching []graph.Edge
			for _, e := range edges {
				if e.Kind == graph.EdgeIntegration {
					matching = append(matching, e)
				}
			}
			if len(matching) == 0 {
				t.Fatal("expected at least one EdgeIntegration edge")
			}
			e := matching[0]
			if e.TargetNode != tt.wantURL {
				t.Errorf("TargetNode = %q, want %q", e.TargetNode, tt.wantURL)
			}
			if !metadataContains(e.Metadata, tt.wantMeta) {
				t.Errorf("Metadata = %v, want to contain %v", e.Metadata, tt.wantMeta)
			}
		})
	}
}

func TestJSIntegrationExtractor_AxiosObjectConfig(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		src      string
		ext      string
		wantURL  string
	}{
		{
			name:    "axios object config with method",
			src:     `function req() { axios({method:"POST", url:"https://api.example.com/data"}); }`,
			ext:     ".js",
			wantURL: "POST https://api.example.com/data",
		},
		{
			name:    "axios object config without method (defaults to GET)",
			src:     `function req() { axios({url:"/api/users"}); }`,
			ext:     ".js",
			wantURL: "GET /api/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges("test"+tt.ext, []byte(tt.src))
			if err != nil {
				t.Fatal(err)
			}
			var matching []graph.Edge
			for _, e := range edges {
				if e.Kind == graph.EdgeIntegration {
					matching = append(matching, e)
				}
			}
			if len(matching) == 0 {
				t.Fatal("expected at least one EdgeIntegration edge")
			}
			e := matching[0]
			if e.TargetNode != tt.wantURL {
				t.Errorf("TargetNode = %q, want %q", e.TargetNode, tt.wantURL)
			}
		})
	}
}

func TestJSIntegrationExtractor_EmitterEmit(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		src           string
		wantTarget    string
		wantTopic     string
	}{
		{
			name:       "emitter.emit literal topic",
			src:        `function notify() { emitter.emit("user.created", {id:1}); }`,
			wantTarget: "emit:user.created",
			wantTopic:  "user.created",
		},
		{
			name:       "emitter.emit with variable topic",
			src:        `function notify() { const topic = "user.created"; emitter.emit(topic, {id:1}); }`,
			wantTarget: "emit:<var:topic>",
			wantTopic:  "<var:topic>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges("test.js", []byte(tt.src))
			if err != nil {
				t.Fatal(err)
			}
			var matching []graph.Edge
			for _, e := range edges {
				if e.Kind == graph.EdgeIntegration {
					matching = append(matching, e)
				}
			}
			if len(matching) == 0 {
				t.Fatal("expected at least one EdgeIntegration edge")
			}
			e := matching[0]
			if e.TargetNode != tt.wantTarget {
				t.Errorf("TargetNode = %q, want %q", e.TargetNode, tt.wantTarget)
			}
			meta, ok := e.Metadata["topic"].(string)
			if !ok || meta != tt.wantTopic {
				t.Errorf("Metadata[topic] = %v, want %q", e.Metadata["topic"], tt.wantTopic)
			}
			if e.Metadata["kind"] != "queue_publish" {
				t.Errorf("Metadata[kind] = %v, want %q", e.Metadata["kind"], "queue_publish")
			}
		})
	}
}

func TestJSIntegrationExtractor_ChannelPublish(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		src        string
		wantTarget string
	}{
		{
			name:       "channel.publish routing key",
			src:        `function sendMsg() { channel.publish("exchange", "order.created", {data}); }`,
			wantTarget: "publish:order.created",
		},
		{
			name:       "redis.publish channel",
			src:        `function broadcast() { redis.publish("notifications", msg); }`,
			wantTarget: "publish:notifications",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges("test.js", []byte(tt.src))
			if err != nil {
				t.Fatal(err)
			}
			var matching []graph.Edge
			for _, e := range edges {
				if e.Kind == graph.EdgeIntegration {
					matching = append(matching, e)
				}
			}
			if len(matching) == 0 {
				t.Fatal("expected at least one EdgeIntegration edge")
			}
			e := matching[0]
			if e.TargetNode != tt.wantTarget {
				t.Errorf("TargetNode = %q, want %q", e.TargetNode, tt.wantTarget)
			}
		})
	}
}

func TestJSIntegrationExtractor_Consumer(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		src        string
		wantSource string
		wantTopic  string
	}{
		{
			name:       "emitter.on event",
			src:        `function setup() { emitter.on("user.created", (user) => { console.log(user); }); }`,
			wantSource: "CONSUME user.created",
			wantTopic:  "user.created",
		},
		{
			name:       "channel.consume queue",
			src:        `function start() { channel.consume("orders", (msg) => { process(msg); }); }`,
			wantSource: "CONSUME orders",
			wantTopic:  "orders",
		},
		{
			name:       "redis.subscribe channel",
			src:        `function listen() { redis.subscribe("updates", (data) => { handle(data); }); }`,
			wantSource: "CONSUME updates",
			wantTopic:  "updates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges("test.js", []byte(tt.src))
			if err != nil {
				t.Fatal(err)
			}
			var matching []graph.Edge
			for _, e := range edges {
				if e.Kind == graph.EdgeIntegration {
					matching = append(matching, e)
				}
			}
			if len(matching) == 0 {
				t.Fatal("expected at least one EdgeIntegration edge")
			}
			e := matching[0]
			if e.SourceNode != tt.wantSource {
				t.Errorf("SourceNode = %q, want %q", e.SourceNode, tt.wantSource)
			}
			meta, ok := e.Metadata["topic"].(string)
			if !ok || meta != tt.wantTopic {
				t.Errorf("Metadata[topic] = %v, want %q", e.Metadata["topic"], tt.wantTopic)
			}
			if e.Metadata["kind"] != "queue_consumer" {
				t.Errorf("Metadata[kind] = %v, want %q", e.Metadata["kind"], "queue_consumer")
			}
		})
	}
}

func TestJSIntegrationExtractor_BareFunctionCall(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		src        string
		wantTarget string
		wantKind   string
	}{
		{
			name:       "bare emit",
			src:        `function run() { emit("started", data); }`,
			wantTarget: "emit:started",
			wantKind:   "queue_publish",
		},
		{
			name:       "bare on",
			src:        `function run() { on("event", handler); }`,
			wantTarget: "event",
			wantKind:   "queue_consumer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges("test.js", []byte(tt.src))
			if err != nil {
				t.Fatal(err)
			}
			var matching []graph.Edge
			for _, e := range edges {
				if e.Kind == graph.EdgeIntegration {
					matching = append(matching, e)
				}
			}
			if len(matching) == 0 {
				t.Fatal("expected at least one EdgeIntegration edge")
			}
			e := matching[0]
			if tt.wantKind == "queue_publish" && e.TargetNode != tt.wantTarget {
				t.Errorf("TargetNode = %q, want %q", e.TargetNode, tt.wantTarget)
			}
			if tt.wantKind == "queue_consumer" && e.SourceNode != "CONSUME "+tt.wantTarget {
				t.Errorf("SourceNode = %q, want %q", e.SourceNode, "CONSUME "+tt.wantTarget)
			}
		})
	}
}

func TestJSIntegrationExtractor_TopLevelCall_NoEdge(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte(`fetch("https://api.example.com");`)
	edges, err := ex.ExtractEdges("test.js", src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			t.Error("expected no EdgeIntegration for top-level call")
		}
	}
}

func TestJSIntegrationExtractor_MultipleEdges(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte(`
function handler() {
  fetch("https://api.example.com/users");
  emitter.emit("user.fetched", {id: 1});
  redis.subscribe("events", (e) => {});
}
`)
	edges, err := ex.ExtractEdges("test.js", src)
	if err != nil {
		t.Fatal(err)
	}
	var integrationEdges []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			integrationEdges = append(integrationEdges, e)
		}
	}
	if len(integrationEdges) != 3 {
		t.Fatalf("expected 3 integration edges, got %d", len(integrationEdges))
	}

	seen := make(map[string]bool)
	consumerSeen := false
	for _, e := range integrationEdges {
		if e.Kind != graph.EdgeIntegration {
			t.Errorf("edge Kind = %v, want EdgeIntegration", e.Kind)
		}
		seen[e.TargetNode] = true
		if e.SourceNode == "CONSUME events" {
			consumerSeen = true
		}
		if e.SourceNode != "test.js::handler" && e.SourceNode != "CONSUME events" {
			t.Errorf("unexpected SourceNode = %q", e.SourceNode)
		}
	}
	if !seen["GET https://api.example.com/users"] {
		t.Error("missing fetch HTTP edge")
	}
	if !seen["emit:user.fetched"] {
		t.Error("missing emit queue publish edge")
	}
	if !consumerSeen {
		t.Error("missing subscribe consumer edge (source=CONSUME events)")
	}
}

func TestJSIntegrationExtractor_TSFile(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte(`
function load(): void {
  fetch("https://api.example.com/data");
}
`)
	edges, err := ex.ExtractEdges("test.ts", src)
	if err != nil {
		t.Fatal(err)
	}
	var integrationEdges []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			integrationEdges = append(integrationEdges, e)
		}
	}
	if len(integrationEdges) != 1 {
		t.Fatalf("expected 1 integration edge, got %d", len(integrationEdges))
	}
	e := integrationEdges[0]
	if e.Language != "typescript" {
		t.Errorf("Language = %q, want typescript", e.Language)
	}
	if e.TargetNode != "GET https://api.example.com/data" {
		t.Errorf("TargetNode = %q, want GET https://api.example.com/data", e.TargetNode)
	}
}

func TestJSIntegrationExtractor_ArrowFunction(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}
	src := []byte(`
const handler = () => {
  fetch("/api/data");
};
`)
	edges, err := ex.ExtractEdges("test.js", src)
	if err != nil {
		t.Fatal(err)
	}
	var integrationEdges []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			integrationEdges = append(integrationEdges, e)
		}
	}
	if len(integrationEdges) != 1 {
		t.Fatalf("expected 1 integration edge from arrow function, got %d", len(integrationEdges))
	}
	e := integrationEdges[0]
	if e.SourceNode != "test.js::handler" {
		t.Errorf("SourceNode = %q, want test.js::handler", e.SourceNode)
	}
}

func metadataContains(meta map[string]any, want map[string]any) bool {
	for k, v := range want {
		got, ok := meta[k]
		if !ok || got != v {
			return false
		}
	}
	return true
}

func TestJSIntegrationExtractor_FixtureFile(t *testing.T) {
	ex, err := graph.NewJSIntegrationExtractor()
	if err != nil {
		t.Fatal(err)
	}

	fixturePath := filepath.Join("testdata", "javascript", "sample.js")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skip("no fixture file, skipping:", err)
	}

	edges, err := ex.ExtractEdges(fixturePath, content)
	if err != nil {
		t.Fatal(err)
	}
	_ = edges
}
