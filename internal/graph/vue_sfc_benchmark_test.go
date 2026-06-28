package graph_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func loadFixture(t testing.TB, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "vue-fixture", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return data
}

func newTestVueExtractor(t testing.TB) *graph.VueSFCExtractor {
	t.Helper()
	ex, err := graph.NewVueSFCExtractor()
	if err != nil {
		t.Fatalf("NewVueSFCExtractor: %v", err)
	}
	return ex
}

type VueTask struct {
	ID            string
	Category      string
	Fixture       string
	Node          string
	Direction     string
	EdgeType      string
	ExpectTargets []string
	ExpectCount   int
}

var vueTasks = []VueTask{
	{ID: "contains-small", Category: "contains", Fixture: "small-counter.vue",
		Node: "small-counter.vue", Direction: "out", EdgeType: "contains",
		ExpectTargets: []string{"small-counter.vue::count", "small-counter.vue::increment"}, ExpectCount: 3},
	{ID: "imports-small", Category: "imports", Fixture: "small-counter.vue",
		Node: "small-counter.vue", Direction: "out", EdgeType: "imports",
		ExpectTargets: []string{"vue", "./IconButton.vue"}, ExpectCount: 2},
	{ID: "calls-medium", Category: "calls", Fixture: "medium-component.vue",
		Node: "medium-component.vue::addUser", Direction: "out", EdgeType: "calls",
		ExpectTargets: []string{"push", "persistUsers"}, ExpectCount: 3},
	{ID: "components-heavy", Category: "imports", Fixture: "component-heavy.vue",
		Node: "component-heavy.vue", Direction: "out", EdgeType: "imports",
		ExpectCount: 10},
	{ID: "contains-large", Category: "contains", Fixture: "large-page.vue",
		Node: "large-page.vue", Direction: "out", EdgeType: "contains",
		ExpectCount: 30},
	{ID: "js-require", Category: "imports", Fixture: "js-script.vue",
		Node: "js-script.vue", Direction: "out", EdgeType: "imports",
		ExpectTargets: []string{"vue", "axios", "lodash"}, ExpectCount: 3},
	{ID: "mixed-blocks", Category: "imports", Fixture: "mixed-blocks.vue",
		Node: "mixed-blocks.vue", Direction: "out", EdgeType: "imports",
		ExpectTargets: []string{"vue", "./ProfileCard.vue"}, ExpectCount: 4},
	{ID: "template-only", Category: "contains", Fixture: "template-only.vue",
		Node: "template-only.vue", Direction: "out", EdgeType: "",
		ExpectCount: 0},
	{ID: "malformed", Category: "contains", Fixture: "malformed.vue",
		Node: "malformed.vue", Direction: "out", EdgeType: "",
		ExpectCount: 0},
}

func filterByKind(edges []graph.Edge, kind graph.EdgeKind) []graph.Edge {
	var out []graph.Edge
	for _, e := range edges {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

func graphOutgoing(edges []graph.Edge, node string) []graph.Edge {
	var out []graph.Edge
	for _, e := range edges {
		if e.SourceNode == node {
			out = append(out, e)
		}
	}
	return out
}

func graphIncoming(edges []graph.Edge, node string) []graph.Edge {
	var out []graph.Edge
	for _, e := range edges {
		if e.TargetNode == node {
			out = append(out, e)
		}
	}
	return out
}

func bfsTrace(edges []graph.Edge, start string, maxDepth int) []string {
	type entry struct{ node string; depth int }
	queue := []entry{{start, 0}}
	visited := map[string]bool{start: true}
	var trace []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		trace = append(trace, cur.node)
		if cur.depth >= maxDepth {
			continue
		}
		for _, e := range edges {
			if e.SourceNode == cur.node && !visited[e.TargetNode] {
				visited[e.TargetNode] = true
				queue = append(queue, entry{e.TargetNode, cur.depth + 1})
			}
		}
	}
	return trace
}

func reverseBFS(edges []graph.Edge, start string, maxDepth int) []string {
	type entry struct{ node string; depth int }
	queue := []entry{{start, 0}}
	visited := map[string]bool{start: true}
	var impacted []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth > 0 {
			impacted = append(impacted, cur.node)
		}
		if cur.depth >= maxDepth {
			continue
		}
		for _, e := range edges {
			if e.TargetNode == cur.node && !visited[e.SourceNode] {
				visited[e.SourceNode] = true
				queue = append(queue, entry{e.SourceNode, cur.depth + 1})
			}
		}
	}
	return impacted
}

func BenchmarkVueSFC_ExtractEdges(b *testing.B) {
	fixtures := []struct {
		name  string
		fix   string
	}{
		{"Small", "small-counter.vue"},
		{"Medium", "medium-component.vue"},
		{"Large", "large-page.vue"},
		{"Empty", "template-only.vue"},
		{"ComponentHeavy", "component-heavy.vue"},
	}
	for _, f := range fixtures {
		b.Run(f.name, func(b *testing.B) {
			ex := newTestVueExtractor(b)
			content := loadFixture(b, f.fix)
			b.SetBytes(int64(len(content)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := ex.ExtractEdges(f.fix, content); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestVueSFC_AgentWorkflow(t *testing.T) {
	ex := newTestVueExtractor(t)
	passed, failed := 0, 0

	for _, task := range vueTasks {
		t.Run(task.ID, func(t *testing.T) {
			content := loadFixture(t, task.Fixture)
			edges, err := ex.ExtractEdges(task.Fixture, content)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if task.EdgeType != "" {
				edges = filterByKind(edges, graph.EdgeKind(task.EdgeType))
			}
			switch task.Direction {
			case "out":
				edges = graphOutgoing(edges, task.Node)
			case "in":
				edges = graphIncoming(edges, task.Node)
			case "both":
				out := graphOutgoing(edges, task.Node)
				in := graphIncoming(edges, task.Node)
				edges = append(out, in...)
			}
			if len(edges) < task.ExpectCount {
				t.Errorf("got %d edges, want >= %d", len(edges), task.ExpectCount)
				failed++
				return
			}
			if len(task.ExpectTargets) > 0 {
				targetSet := make(map[string]bool)
				for _, e := range edges {
					targetSet[e.TargetNode] = true
				}
				for _, want := range task.ExpectTargets {
					if !targetSet[want] {
						t.Errorf("missing expected target %q", want)
					}
				}
			}
			passed++
		})
	}
	total := passed + failed
	fmt.Printf("AgentWorkflow: %d/%d passed (%.0f%%)\n", passed, total, float64(passed)/float64(total)*100)
	if failed > 0 {
		t.Errorf("%d tasks failed", failed)
	}
}

func BenchmarkVueSFC_Pipeline_ExtractAndQuery(b *testing.B) {
	ex := newTestVueExtractor(b)
	content := loadFixture(b, "large-page.vue")
	filePath := "large-page.vue"
	node := filePath
	callsNode := node + "::loadDashboardData"

	b.Run("Extract", func(b *testing.B) {
		b.SetBytes(int64(len(content)))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := ex.ExtractEdges(filePath, content); err != nil {
				b.Fatal(err)
			}
		}
	})

	edges, err := ex.ExtractEdges(filePath, content)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("GraphOut", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = graphOutgoing(edges, node)
		}
	})

	b.Run("GraphIn", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = graphIncoming(edges, callsNode)
		}
	})

	b.Run("Trace_Depth3", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = bfsTrace(edges, callsNode, 3)
		}
	})

	b.Run("Impact_Depth1", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = reverseBFS(edges, callsNode, 1)
		}
	})
}

func BenchmarkVueSFC_Allocations(b *testing.B) {
	fixtures := []struct {
		name string
		fix  string
	}{
		{"Small", "small-counter.vue"},
		{"Medium", "medium-component.vue"},
		{"Large", "large-page.vue"},
		{"Empty", "template-only.vue"},
		{"ComponentHeavy", "component-heavy.vue"},
	}
	for _, f := range fixtures {
		b.Run(f.name, func(b *testing.B) {
			ex := newTestVueExtractor(b)
			content := loadFixture(b, f.fix)
			b.SetBytes(int64(len(content)))
			b.ReportAllocs()
			b.ResetTimer()
			var totalEdges int
			for i := 0; i < b.N; i++ {
				edges, err := ex.ExtractEdges(f.fix, content)
				if err != nil {
					b.Fatal(err)
				}
				totalEdges += len(edges)
			}
			if totalEdges > 0 {
				allocs := b.Elapsed().Nanoseconds()
				b.ReportMetric(float64(totalEdges)/float64(b.N), "edges/op")
				b.ReportMetric(float64(totalEdges), "total-edges")
				_ = allocs
			}
		})
	}
}
