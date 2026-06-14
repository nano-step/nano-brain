package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func newPythonIntegrationExtractor(t *testing.T) *graph.PythonIntegrationExtractor {
	t.Helper()
	ex, err := graph.NewPythonIntegrationExtractor()
	if err != nil {
		t.Fatalf("NewPythonIntegrationExtractor: %v", err)
	}
	return ex
}

func TestPythonIntegrationExtractor_Supports(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	tests := []struct {
		ext  string
		want bool
	}{
		{".py", true},
		{".go", false},
		{".js", false},
		{".ts", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := ex.Supports(tt.ext); got != tt.want {
			t.Errorf("Supports(%q) = %v, want %v", tt.ext, got, tt.want)
		}
	}
}

func TestPythonIntegrationExtractor_EmptyFile(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	edges, err := ex.ExtractEdges("empty.py", []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges from empty file, got %d", len(edges))
	}
}

func TestPythonIntegrationExtractor_SyntaxError(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	src := []byte("def broken(\n    pass\nimport os\n")
	edges, err := ex.ExtractEdges("broken.py", src)
	if err != nil {
		t.Fatal("should not error on partial parse:", err)
	}
	_ = edges
}

func TestPythonIntegrationExtractor_RequestsHTTP(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	tests := []struct {
		name     string
		src      string
		wantURL  string
		wantMeta map[string]any
	}{
		{
			name:    "requests.get literal url",
			src:     "def fetch():\n    requests.get('https://api.example.com/users')\n",
			wantURL: "GET https://api.example.com/users",
			wantMeta: map[string]any{"kind": "http_call", "method": "GET", "url": "https://api.example.com/users"},
		},
		{
			name:    "requests.post literal url",
			src:     "def create():\n    requests.post('https://api.example.com/users', json={'name': 'Alice'})\n",
			wantURL: "POST https://api.example.com/users",
			wantMeta: map[string]any{"kind": "http_call", "method": "POST", "url": "https://api.example.com/users"},
		},
		{
			name:    "requests.get with variable",
			src:     "def fetch():\n    url = '/api/data'\n    requests.get(url)\n",
			wantURL: "GET <var:url>",
			wantMeta: map[string]any{"kind": "http_call", "method": "GET", "url": "<var:url>"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges("test.py", []byte(tt.src))
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
			if e.SourceNode != "test.py::fetch" && e.SourceNode != "test.py::create" {
				t.Errorf("SourceNode = %q, want test.py::fetch or test.py::create", e.SourceNode)
			}
		})
	}
}

func TestPythonIntegrationExtractor_HttpxHTTP(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	tests := []struct {
		name    string
		src     string
		wantURL string
	}{
		{
			name:    "httpx.get",
			src:     "def fetch():\n    httpx.get('https://api.example.com/data')\n",
			wantURL: "GET https://api.example.com/data",
		},
		{
			name:    "httpx.post",
			src:     "def create():\n    httpx.post('https://api.example.com/items', json={})\n",
			wantURL: "POST https://api.example.com/items",
		},
		{
			name:    "async httpx.get",
			src:     "async def fetch():\n    async with httpx.AsyncClient() as client:\n        await client.get('https://api.example.com/data')\n",
			wantURL: "GET https://api.example.com/data",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges("test.py", []byte(tt.src))
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

func TestPythonIntegrationExtractor_SessionHTTP(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	tests := []struct {
		name    string
		src     string
		wantURL string
	}{
		{
			name:    "session.get literal url",
			src:     "def fetch(s):\n    s.get('https://api.example.com/users')\n",
			wantURL: "GET https://api.example.com/users",
		},
		{
			name:    "client.post",
			src:     "def create(client):\n    client.post('/api/items')\n",
			wantURL: "POST /api/items",
		},
		{
			name:    "session.put",
			src:     "def update(session):\n    session.put('/api/items/1', json={'name': 'foo'})\n",
			wantURL: "PUT /api/items/1",
		},
		{
			name:    "session.delete",
			src:     "def remove(session):\n    session.delete('/api/items/1')\n",
			wantURL: "DELETE /api/items/1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges("test.py", []byte(tt.src))
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

func TestPythonIntegrationExtractor_BasicPublish(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	tests := []struct {
		name       string
		src        string
		wantTarget string
		wantTopic  string
	}{
		{
			name:       "channel.basic_publish with routing_key kwarg",
			src:        "def send():\n    channel.basic_publish(exchange='', routing_key='order.created', body=b'hello')\n",
			wantTarget: "publish:order.created",
			wantTopic:  "order.created",
		},
		{
			name:       "channel.basic_publish with variable routing_key",
			src:        "def send():\n    key = 'order.created'\n    channel.basic_publish(exchange='', routing_key=key, body=b'hello')\n",
			wantTarget: "publish:<var:key>",
			wantTopic:  "<var:key>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges("test.py", []byte(tt.src))
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

func TestPythonIntegrationExtractor_RedisPublish(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	tests := []struct {
		name       string
		src        string
		wantTarget string
		wantTopic  string
	}{
		{
			name:       "redis.publish literal channel",
			src:        "def broadcast(r):\n    r.publish('notifications', 'hello')\n",
			wantTarget: "publish:notifications",
			wantTopic:  "notifications",
		},
		{
			name:       "redis.publish with variable channel",
			src:        "def broadcast(r):\n    ch = 'updates'\n    r.publish(ch, 'hello')\n",
			wantTarget: "publish:<var:ch>",
			wantTopic:  "<var:ch>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges("test.py", []byte(tt.src))
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

func TestPythonIntegrationExtractor_Consumer(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	tests := []struct {
		name       string
		src        string
		wantSource string
		wantTopic  string
	}{
		{
			name:       "channel.consume",
			src:        "def start(ch):\n    ch.consume('orders', callback)\n",
			wantSource: "CONSUME orders",
			wantTopic:  "orders",
		},
		{
			name:       "pubsub.subscribe",
			src:        "def listen(pubsub):\n    pubsub.subscribe('updates')\n",
			wantSource: "CONSUME updates",
			wantTopic:  "updates",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges("test.py", []byte(tt.src))
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

func TestPythonIntegrationExtractor_TopLevelCall_NoEdge(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	src := []byte("import requests\nrequests.get('https://api.example.com')\n")
	edges, err := ex.ExtractEdges("test.py", src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			t.Error("expected no EdgeIntegration for top-level call")
		}
	}
}

func TestPythonIntegrationExtractor_MultipleEdges(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	src := []byte(`
def handler():
    requests.get("https://api.example.com/users")
    r.publish("user.fetched", {"id": 1})
    pubsub.subscribe("events", callback)
`)
	edges, err := ex.ExtractEdges("test.py", src)
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
		if e.SourceNode != "test.py::handler" && e.SourceNode != "CONSUME events" {
			t.Errorf("unexpected SourceNode = %q", e.SourceNode)
		}
	}
	if !seen["GET https://api.example.com/users"] {
		t.Error("missing requests.get HTTP edge")
	}
	if !seen["publish:user.fetched"] {
		t.Error("missing redis publish edge")
	}
	if !consumerSeen {
		t.Error("missing subscribe consumer edge (source=CONSUME events)")
	}
}

func TestPythonIntegrationExtractor_LineNumbers(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	src := []byte("def call():\n    requests.get('https://example.com/api')\n")
	edges, err := ex.ExtractEdges("test.py", src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration && e.Line == 0 {
			t.Errorf("integration edge should have non-zero line number, got Line=0")
		}
	}
}

func TestPythonIntegrationExtractor_SourceNode(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	src := []byte("def outer_func():\n    requests.get('https://example.com')\n")
	edges, err := ex.ExtractEdges("internal/foo.py", src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			if e.SourceNode == "" {
				t.Error("SourceNode should not be empty")
			}
			if e.SourceNode != "internal/foo.py::outer_func" {
				t.Errorf("expected SourceNode 'internal/foo.py::outer_func', got %q", e.SourceNode)
			}
		}
	}
}

func TestPythonIntegrationExtractor_EmitPublish(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	tests := []struct {
		name       string
		src        string
		wantTarget string
		wantKind   string
	}{
		{
			name:       "emitter.emit",
			src:        "def notify():\n    emitter.emit('user.created', {'id': 1})\n",
			wantTarget: "emit:user.created",
			wantKind:   "queue_publish",
		},
		{
			name:       "emitter.send",
			src:        "def notify():\n    emitter.send('notification', data)\n",
			wantTarget: "send:notification",
			wantKind:   "queue_publish",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges("test.py", []byte(tt.src))
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

func TestPythonIntegrationExtractor_BarePublish(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	src := []byte("def notify():\n    emit('event.occurred', data)\n")
	edges, err := ex.ExtractEdges("test.py", src)
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
	if e.TargetNode != "emit:event.occurred" {
		t.Errorf("TargetNode = %q, want %q", e.TargetNode, "emit:event.occurred")
	}
}

func TestPythonIntegrationExtractor_BareConsumer(t *testing.T) {
	ex := newPythonIntegrationExtractor(t)
	src := []byte("def setup():\n    subscribe('events', handler)\n")
	edges, err := ex.ExtractEdges("test.py", src)
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
	if e.SourceNode != "CONSUME events" {
		t.Errorf("SourceNode = %q, want %q", e.SourceNode, "CONSUME events")
	}
}
