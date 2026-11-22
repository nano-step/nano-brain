package graph_test

import (
	"fmt"
	"sync"
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
			src:  "import { helper } from \"./helper\";\nexport const handler = (req: Request): Response => {\n  return helper(req);\n}\n",
		},
		{
			name: "tsx_handler",
			file: "src/Handler.tsx",
			src:  "import { helper } from \"./helper\";\nexport const handler = (req: Request): Response => {\n  const view = helper(req);\n  return view;\n}\n",
		},
	}

	ic := graph.ImportContext{
		Exists: func(p string) bool { return p == "src/helper.ts" },
		ReadFile: func(p string) ([]byte, error) {
			if p == "src/helper.ts" {
				return []byte("export const helper = (req: Request): Response => req;\n"), nil
			}
			return nil, fmt.Errorf("unexpected path %s", p)
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

			icEdges, err := ex.ExtractEdgesWithImportContext(tc.file, []byte(tc.src), ic)
			if err != nil {
				t.Fatalf("ExtractEdgesWithImportContext(%s): %v", tc.file, err)
			}
			if !hasEdge703(icEdges, graph.EdgeContains, tc.file, wantSym) {
				t.Errorf("%s (ImportContext): missing contains edge %s -> %s; got %+v", tc.file, tc.file, wantSym, icEdges)
			}
			if !hasEdge703(icEdges, graph.EdgeImports, tc.file, "src/helper.ts") {
				t.Errorf("%s (ImportContext): missing resolved import edge %s -> src/helper.ts; got %+v", tc.file, tc.file, icEdges)
			}
			if !hasEdge703(icEdges, graph.EdgeCalls, wantSym, "src/helper.ts::helper") {
				t.Errorf("%s (ImportContext): missing resolved calls edge %s -> src/helper.ts::helper; got %+v", tc.file, wantSym, icEdges)
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
import { helper } from './helper';
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

	ic := graph.ImportContext{
		Exists: func(p string) bool { return p == "src/helper.ts" },
	}
	icEdges, err := ex.ExtractEdgesWithImportContext(file, src, ic)
	if err != nil {
		t.Fatalf("ExtractEdgesWithImportContext(%s): %v", file, err)
	}
	if !hasEdge703(icEdges, graph.EdgeContains, file, wantSym) {
		t.Errorf("%s (ImportContext): missing contains edge %s -> %s; got %+v", file, file, wantSym, icEdges)
	}
	if !hasEdge703(icEdges, graph.EdgeImports, file, "src/helper.ts") {
		t.Errorf("%s (ImportContext): missing resolved import edge %s -> src/helper.ts; got %+v", file, file, icEdges)
	}
	if !hasEdge703(icEdges, graph.EdgeCalls, wantSym, "helper") {
		t.Errorf("%s (ImportContext): missing calls edge %s -> helper; got %+v", file, wantSym, icEdges)
	}
}

func TestVueSFCExtractor_ConcurrentExtractEdges_703(t *testing.T) {
	ex, err := graph.NewVueSFCExtractor()
	if err != nil {
		t.Fatal(err)
	}

	vueA := []byte("<template>\n  <div>{{ ok }}</div>\n</template>\n\n<script setup lang=\"ts\">\nimport ChildCard from './ChildCard.vue';\nexport const alpha = (req: Request): Response => {\n  return runAlpha(req, ChildCard);\n};\n</script>\n")
	vueB := []byte("<template>\n  <span>{{ ready }}</span>\n</template>\n\n<script setup lang=\"ts\">\nimport { helper } from './helper';\nexport const beta = ({ id }: Input): Output => {\n  return helper(id);\n};\n</script>\n")
	ic := graph.ImportContext{
		Exists: func(p string) bool {
			return p == "src/ChildCard.vue" || p == "src/helper.ts"
		},
	}

	const goroutines = 8
	const iterations = 30

	start := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*iterations)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				if (worker+i)%2 == 0 {
					const file = "src/Alpha.vue"
					edges, err := ex.ExtractEdges(file, vueA)
					if err != nil {
						errCh <- fmt.Errorf("worker %d iter %d ExtractEdges: %w", worker, i, err)
						return
					}
					wantSym := file + "::alpha"
					if !hasEdge703(edges, graph.EdgeContains, file, wantSym) || !hasEdge703(edges, graph.EdgeCalls, wantSym, "runAlpha") {
						errCh <- fmt.Errorf("worker %d iter %d ExtractEdges missing edges: %+v", worker, i, edges)
						return
					}
				} else {
					const file = "src/Beta.vue"
					edges, err := ex.ExtractEdgesWithImportContext(file, vueB, ic)
					if err != nil {
						errCh <- fmt.Errorf("worker %d iter %d ExtractEdgesWithImportContext: %w", worker, i, err)
						return
					}
					wantSym := file + "::beta"
					if !hasEdge703(edges, graph.EdgeContains, file, wantSym) ||
						!hasEdge703(edges, graph.EdgeCalls, wantSym, "helper") ||
						!hasEdge703(edges, graph.EdgeImports, file, "src/helper.ts") {
						errCh <- fmt.Errorf("worker %d iter %d ExtractEdgesWithImportContext missing edges: %+v", worker, i, edges)
						return
					}
				}
			}
		}(g)
	}

	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
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
