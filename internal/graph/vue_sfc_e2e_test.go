package graph_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func TestVueSFC_E2E_SmallCounter(t *testing.T) {
	ex := newTestVueExtractor(t)
	content := loadFixture(t, "small-counter.vue")
	edges, err := ex.ExtractEdges("components/SmallCounter.vue", content)
	if err != nil {
		t.Fatal(err)
	}

	containsEdges := filterEdges(edges, graph.EdgeContains)
	if len(containsEdges) == 0 {
		t.Fatal("expected contains edges from small-counter.vue")
	}
	foundIncrement := false
	for _, e := range containsEdges {
		if e.TargetNode == "components/SmallCounter.vue::increment" {
			foundIncrement = true
		}
	}
	if !foundIncrement {
		t.Error("missing contains edge for 'increment'")
	}

	importEdges := filterEdges(edges, graph.EdgeImports)
	if len(importEdges) == 0 {
		t.Fatal("expected import edges from small-counter.vue")
	}
	foundVueImport := false
	foundComponentImport := false
	for _, e := range importEdges {
		if e.TargetNode == "vue" {
			foundVueImport = true
		}
		if e.TargetNode == "./IconButton.vue" {
			foundComponentImport = true
			if e.Metadata == nil || e.Metadata["component"] != true {
				t.Error("component import should have {component: true} metadata")
			}
		}
	}
	if !foundVueImport {
		t.Error("missing import edge for 'vue'")
	}
	if !foundComponentImport {
		t.Error("missing import edge for './IconButton.vue'")
	}

	callEdges := filterEdges(edges, graph.EdgeCalls)
	if len(callEdges) == 0 {
		t.Error("expected call edges from small-counter.vue")
	}
}

func TestVueSFC_E2E_ComponentHeavy(t *testing.T) {
	ex := newTestVueExtractor(t)
	content := loadFixture(t, "component-heavy.vue")
	edges, err := ex.ExtractEdges("views/ComponentHeavy.vue", content)
	if err != nil {
		t.Fatal(err)
	}

	importEdges := filterEdges(edges, graph.EdgeImports)
	vueImports := 0
	for _, e := range importEdges {
		if e.Metadata != nil && e.Metadata["component"] == true {
			vueImports++
			if ext := filepath.Ext(e.TargetNode); ext != ".vue" {
				t.Errorf("component import %q should end in .vue", e.TargetNode)
			}
		}
	}
	if vueImports < 10 {
		t.Errorf("expected 10+ component imports, got %d", vueImports)
	}

	sortedImports := make([]string, len(importEdges))
	for i, e := range importEdges {
		sortedImports[i] = e.TargetNode
	}
	sort.Strings(sortedImports)
	for i := 1; i < len(sortedImports); i++ {
		if sortedImports[i] == sortedImports[i-1] {
			t.Errorf("duplicate import: %s", sortedImports[i])
		}
	}
}

func TestVueSFC_E2E_EmptyTemplate(t *testing.T) {
	ex := newTestVueExtractor(t)
	content := loadFixture(t, "template-only.vue")
	edges, err := ex.ExtractEdges("pages/NotFound.vue", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges from template-only.vue, got %d: %+v", len(edges), edges)
	}
}

func TestVueSFC_E2E_Malformed(t *testing.T) {
	ex := newTestVueExtractor(t)
	content := loadFixture(t, "malformed.vue")
	edges, err := ex.ExtractEdges("components/Malformed.vue", content)
	if err != nil {
		t.Fatalf("should not error on malformed file: %v", err)
	}
	if edges == nil {
		edges = []graph.Edge{}
	}
	if len(edges) == 0 {
		t.Error("expected at least some edges from partial parse of malformed.vue")
	}
}

func TestVueSFC_E2E_AllFixtures(t *testing.T) {
	ex := newTestVueExtractor(t)
	fixtures := []string{
		"small-counter.vue",
		"medium-component.vue",
		"large-page.vue",
		"template-only.vue",
		"component-heavy.vue",
		"js-script.vue",
		"mixed-blocks.vue",
		"malformed.vue",
	}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			content := loadFixture(t, name)
			edges, err := ex.ExtractEdges("test/"+name, content)
			if err != nil {
				t.Errorf("unexpected error on %s: %v", name, err)
			}
			if edges == nil {
				edges = []graph.Edge{}
			}
			for _, e := range edges {
				if e.SourceFile == "" {
					t.Errorf("edge from %s has empty SourceFile", name)
				}
				if e.Kind == "" {
					t.Errorf("edge from %s has empty Kind", name)
				}
			}
		})
	}
}

func TestVueSFC_E2E_MixedBlocks(t *testing.T) {
	ex := newTestVueExtractor(t)
	content := loadFixture(t, "mixed-blocks.vue")
	edges, err := ex.ExtractEdges("components/UserProfile.vue", content)
	if err != nil {
		t.Fatal(err)
	}

	containsEdges := filterEdges(edges, graph.EdgeContains)
	if len(containsEdges) == 0 {
		t.Fatal("expected contains edges from mixed-blocks.vue")
	}

	names := make(map[string]bool)
	for _, e := range containsEdges {
		parts := splitContainsTarget(e.TargetNode)
		names[parts] = true
	}

	if !names["loadProfile"] {
		t.Error("missing contains edge for 'loadProfile' from <script setup> block")
	}
	if !names["switchTab"] {
		t.Error("missing contains edge for 'switchTab' from <script setup> block")
	}
}

func TestVueSFC_E2E_JavaScriptScript(t *testing.T) {
	ex := newTestVueExtractor(t)
	content := loadFixture(t, "js-script.vue")
	edges, err := ex.ExtractEdges("views/DataView.vue", content)
	if err != nil {
		t.Fatal(err)
	}

	importEdges := filterEdges(edges, graph.EdgeImports)
	requireImports := 0
	for _, e := range importEdges {
		if e.TargetNode == "axios" || e.TargetNode == "lodash" || e.TargetNode == "vue" {
			requireImports++
		}
	}
	if requireImports < 3 {
		t.Errorf("expected 3 require imports, got %d", requireImports)
	}
}

func filterEdges(edges []graph.Edge, kind graph.EdgeKind) []graph.Edge {
	var result []graph.Edge
	for _, e := range edges {
		if e.Kind == kind {
			result = append(result, e)
		}
	}
	return result
}

func splitContainsTarget(target string) string {
	for i := len(target) - 1; i >= 0; i-- {
		if target[i] == ':' {
			return target[i+1:]
		}
	}
	return target
}

func TestVueSFC_E2E_AgentScenarios(t *testing.T) {
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
				edges = filterEdges(edges, graph.EdgeKind(task.EdgeType))
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
	t.Logf("AgentScenarios: %d/%d passed", passed, passed+failed)
}

func TestVueSFC_E2E_EdgeQuality(t *testing.T) {
	ex := newTestVueExtractor(t)
	type edgeRange struct {
		containsMin, containsMax int
		importsMin, importsMax   int
		callsMin, callsMax       int
	}
	ranges := map[string]edgeRange{
		"small-counter.vue":    {containsMin: 2, containsMax: 5, importsMin: 1, importsMax: 3, callsMin: 0, callsMax: 5},
		"medium-component.vue": {containsMin: 15, containsMax: 30, importsMin: 3, importsMax: 6, callsMin: 5, callsMax: 15},
		"large-page.vue":       {containsMin: 30, containsMax: 60, importsMin: 15, importsMax: 30, callsMin: 20, callsMax: 60},
		"template-only.vue":    {containsMin: 0, containsMax: 0, importsMin: 0, importsMax: 0, callsMin: 0, callsMax: 0},
		"component-heavy.vue":  {containsMin: 5, containsMax: 15, importsMin: 10, importsMax: 20, callsMin: 0, callsMax: 5},
		"js-script.vue":        {containsMin: 0, containsMax: 5, importsMin: 2, importsMax: 5, callsMin: 0, callsMax: 0},
		"mixed-blocks.vue":     {containsMin: 5, containsMax: 15, importsMin: 3, importsMax: 6, callsMin: 1, callsMax: 10},
		"malformed.vue":        {containsMin: 0, containsMax: 5, importsMin: 1, importsMax: 3, callsMin: 0, callsMax: 0},
	}
	for name, expect := range ranges {
		t.Run(name, func(t *testing.T) {
			content := loadFixture(t, name)
			edges, err := ex.ExtractEdges(name, content)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			contains := len(filterEdges(edges, graph.EdgeContains))
			imports := len(filterEdges(edges, graph.EdgeImports))
			calls := len(filterEdges(edges, graph.EdgeCalls))
			if contains < expect.containsMin || contains > expect.containsMax {
				t.Errorf("contains=%d, want [%d,%d]", contains, expect.containsMin, expect.containsMax)
			}
			if imports < expect.importsMin || imports > expect.importsMax {
				t.Errorf("imports=%d, want [%d,%d]", imports, expect.importsMin, expect.importsMax)
			}
			if calls < expect.callsMin || calls > expect.callsMax {
				t.Errorf("calls=%d, want [%d,%d]", calls, expect.callsMin, expect.callsMax)
			}
		})
	}
}
