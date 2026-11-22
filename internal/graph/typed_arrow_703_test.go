package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func hasEdge703(edges []graph.Edge, kind graph.EdgeKind, src, dst string) bool {
	for _, e := range edges {
		if e.Kind == kind && e.SourceNode == src && e.TargetNode == dst {
			return true
		}
	}
	return false
}

func TestTypeScriptGraphExtractor_TypedFirstParamArrow_703(t *testing.T) {
	ex, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		file string
		src  string
	}{
		{
			name: "ts_handler",
			file: "src/handler.ts",
			src:  "export const handler = (req: Request): Response => {\n  return helper(req);\n}\n",
		},
		{
			name: "tsx_handler",
			file: "src/Handler.tsx",
			src:  "export const handler = (req: Request): Response => {\n  const view = helper(req);\n  return view;\n}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			edges, err := ex.ExtractEdges(tc.file, []byte(tc.src))
			if err != nil {
				t.Fatalf("ExtractEdges(%s): %v", tc.file, err)
			}
			wantSym := tc.file + "::handler"
			if !hasEdge703(edges, graph.EdgeContains, tc.file, wantSym) {
				t.Errorf("%s: missing contains edge %s -> %s; got %+v", tc.file, tc.file, wantSym, edges)
			}
			if !hasEdge703(edges, graph.EdgeCalls, wantSym, "helper") {
				t.Errorf("%s: missing calls edge %s -> helper; got %+v", tc.file, wantSym, edges)
			}
		})
	}
}

func TestVueSFCExtractor_TypedFirstParamArrow_703(t *testing.T) {
	ex, err := graph.NewVueSFCExtractor()
	if err != nil {
		t.Fatal(err)
	}

	const file = "src/Card.vue"
	src := []byte(`<template>
  <div>{{ ok }}</div>
</template>

<script setup lang="ts">
export const handler = (req: Request): Response => {
  return helper(req);
}
</script>
`)

	edges, err := ex.ExtractEdges(file, src)
	if err != nil {
		t.Fatalf("ExtractEdges(%s): %v", file, err)
	}
	wantSym := file + "::handler"
	if !hasEdge703(edges, graph.EdgeContains, file, wantSym) {
		t.Errorf("%s: missing contains edge %s -> %s; got %+v", file, file, wantSym, edges)
	}
	if !hasEdge703(edges, graph.EdgeCalls, wantSym, "helper") {
		t.Errorf("%s: missing calls edge %s -> helper; got %+v", file, wantSym, edges)
	}
}

func TestJSControlFlowExtractor_TypedFirstParamArrow_703(t *testing.T) {
	ex, err := graph.NewJSControlFlowExtractor()
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range []string{"src/classify.ts", "src/classify.tsx"} {
		t.Run(file, func(t *testing.T) {
			src := []byte("export const classify = (x: number): string => {\n  if (x > 0) {\n    return \"pos\";\n  }\n  return \"neg\";\n};\n")
			cfgs, err := ex.ExtractCFGs(file, src)
			if err != nil {
				t.Fatalf("ExtractCFGs(%s): %v", file, err)
			}
			if len(cfgs) != 1 {
				t.Fatalf("%s: expected 1 CFG, got %d: %+v", file, len(cfgs), cfgs)
			}
			cfg := cfgs[0]
			if want := file + "::classify"; cfg.Entry != want {
				t.Errorf("%s: Entry = %q, want %q", file, cfg.Entry, want)
			}
			counts := countNodeTypes(cfg)
			if counts["decision"] < 1 || counts["terminal"] < 2 {
				t.Errorf("%s: expected >=1 decision and >=2 terminals, got %+v", file, counts)
			}
		})
	}
}
