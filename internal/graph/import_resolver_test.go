package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

// --- ResolveImportTarget: pure resolution logic (no disk access) ---

func TestResolveImportTarget_Relative(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		sourceRel string
		want      string
	}{
		{"same-dir", "./sibling", "repo-a/consumer.ts", "repo-a/sibling"},
		{"parent-dir", "../shared/util", "repo-a/pkg/consumer.ts", "repo-a/shared/util"},
		{"root-level-source", "./sibling", "consumer.ts", "sibling"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := graph.ResolveImportTarget(tc.raw, tc.sourceRel, nil, nil)
			if got != tc.want {
				t.Errorf("ResolveImportTarget(%q, %q) = %q, want %q", tc.raw, tc.sourceRel, got, tc.want)
			}
		})
	}
}

func TestResolveImportTarget_Aliased(t *testing.T) {
	aliasMap := map[string]string{"~/": "repo-a", "@/": "repo-a"}

	got := graph.ResolveImportTarget("~/utils/enums", "repo-a/consumer.ts", aliasMap, nil)
	want := "repo-a/utils/enums"
	if got != want {
		t.Errorf("aliased resolve = %q, want %q", got, want)
	}

	got = graph.ResolveImportTarget("@/utils/enums", "repo-a/consumer.ts", aliasMap, nil)
	if got != want {
		t.Errorf("@/ aliased resolve = %q, want %q", got, want)
	}
}

func TestResolveImportTarget_LongestAliasPrefixWins(t *testing.T) {
	aliasMap := map[string]string{
		"~/":       "repo-a",
		"~/utils/": "repo-a/special-utils",
	}
	got := graph.ResolveImportTarget("~/utils/enums", "repo-a/consumer.ts", aliasMap, nil)
	want := "repo-a/special-utils/enums"
	if got != want {
		t.Errorf("longest-prefix resolve = %q, want %q (should prefer the more specific \"~/utils/\" prefix)", got, want)
	}
}

func TestResolveImportTarget_ExactVsWildcardAliasMatch(t *testing.T) {
	// "@utils" (no trailing slash) came from a non-wildcard tsconfig mapping
	// ("@utils": ["./src/utils"]) and must match the specifier EXACTLY.
	// "@utils/*" (stored with a trailing slash, see parseTSConfigPaths) is a
	// wildcard mapping and may prefix-match.
	aliasMap := map[string]string{
		"@utils":  "src/exact-utils",
		"@utils/": "src/wild-utils",
	}

	got := graph.ResolveImportTarget("@utils", "consumer.ts", aliasMap, nil)
	if want := "src/exact-utils"; got != want {
		t.Errorf("exact alias %q resolved to %q, want %q", "@utils", got, want)
	}

	got = graph.ResolveImportTarget("@utils/foo", "consumer.ts", aliasMap, nil)
	if want := "src/wild-utils/foo"; got != want {
		t.Errorf("wildcard alias %q resolved to %q, want %q", "@utils/foo", got, want)
	}

	// "@utils-sibling" must NOT match the exact "@utils" alias (it is neither
	// an exact match nor prefixed by the wildcard "@utils/" key).
	raw := "@utils-sibling"
	got = graph.ResolveImportTarget(raw, "consumer.ts", aliasMap, nil)
	if got != raw {
		t.Errorf("non-alias specifier %q resolved to %q, want unchanged (raw fallback)", raw, got)
	}
}

func TestResolveImportTarget_BarePackagePassthrough(t *testing.T) {
	aliasMap := map[string]string{"~/": "repo-a"}
	for _, raw := range []string{"vue", "ramda", "@scope/pkg", "lodash/debounce"} {
		got := graph.ResolveImportTarget(raw, "repo-a/consumer.ts", aliasMap, func(string) bool { return true })
		if got != raw {
			t.Errorf("bare package %q resolved to %q, want unchanged", raw, got)
		}
	}
}

func TestResolveImportTarget_ExtIndexFallback(t *testing.T) {
	admitted := map[string]bool{
		"repo-a/utils/enums.ts": true,
	}
	exists := func(p string) bool { return admitted[p] }

	got := graph.ResolveImportTarget("./utils/enums", "repo-a/consumer.ts", nil, exists)
	want := "repo-a/utils/enums.ts"
	if got != want {
		t.Errorf("ext-fallback resolve = %q, want %q", got, want)
	}

	admittedIndex := map[string]bool{
		"repo-a/components/index.vue": true,
	}
	existsIndex := func(p string) bool { return admittedIndex[p] }
	got = graph.ResolveImportTarget("./components", "repo-a/consumer.ts", nil, existsIndex)
	want = "repo-a/components/index.vue"
	if got != want {
		t.Errorf("index-fallback resolve = %q, want %q", got, want)
	}
}

func TestResolveImportTarget_UnresolvedFallsBackToRaw(t *testing.T) {
	exists := func(string) bool { return false }
	raw := "~/nonexistent/thing"
	got := graph.ResolveImportTarget(raw, "repo-a/consumer.ts", map[string]string{"~/": "repo-a"}, exists)
	if got != raw {
		t.Errorf("unresolved specifier = %q, want raw %q unchanged (fail-safe)", got, raw)
	}
}

func TestResolveImportTarget_PathEscapeClamp(t *testing.T) {
	raw := "../../../outside"
	got := graph.ResolveImportTarget(raw, "consumer.ts", nil, func(string) bool { return true })
	if got != raw {
		t.Errorf("escaping relative import resolved to %q, want raw %q unchanged (path-escape clamp)", got, raw)
	}
}

// --- AliasIndex / BuildAliasIndex: nearest-ancestor loading (G3) ---

func TestBuildAliasIndex_NearestAncestorAcrossTwoConfigRoots(t *testing.T) {
	root := fixtureRoot(t)
	idx, err := graph.BuildAliasIndex(root)
	if err != nil {
		t.Fatalf("BuildAliasIndex: %v", err)
	}

	aMap := idx.AliasMapFor("repo-a/consumer.ts")
	if aMap["~/"] != "repo-a" {
		t.Errorf("repo-a alias map[\"~/\"] = %q, want %q", aMap["~/"], "repo-a")
	}

	bMap := idx.AliasMapFor("repo-b/consumer.ts")
	if bMap["~/"] != "repo-b/src" {
		t.Errorf("repo-b alias map[\"~/\"] = %q, want %q", bMap["~/"], "repo-b/src")
	}

	// The critical G3 assertion: repo-a's map must NOT leak into repo-b's
	// lookup (a single global map would fabricate a false cross-repo edge).
	if aMap["~/"] == bMap["~/"] {
		t.Fatalf("repo-a and repo-b resolved to the same alias root %q — nearest-ancestor isolation broken", aMap["~/"])
	}
}

func TestBuildAliasIndex_NestedConfigWinsOverShallower(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "tsconfig.json"), `{"compilerOptions":{"baseUrl":".","paths":{"~/*":["./shallow/*"]}}}`)
	mustWriteFile(t, filepath.Join(root, "pkg", "nested", "tsconfig.json"), `{"compilerOptions":{"baseUrl":".","paths":{"~/*":["./deep/*"]}}}`)

	idx, err := graph.BuildAliasIndex(root)
	if err != nil {
		t.Fatalf("BuildAliasIndex: %v", err)
	}

	nested := idx.AliasMapFor("pkg/nested/consumer.ts")
	if want := "pkg/nested/deep"; nested["~/"] != want {
		t.Errorf("nested config alias = %q, want %q (nearest ancestor must win)", nested["~/"], want)
	}

	shallow := idx.AliasMapFor("pkg/other/consumer.ts")
	if want := "shallow"; shallow["~/"] != want {
		t.Errorf("shallow-scoped file alias = %q, want %q (root config, no nested ancestor)", shallow["~/"], want)
	}
}

func TestBuildAliasIndex_ConventionAliasesWithoutTsconfig(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "consumer.ts"), "export const x = 1;")

	idx, err := graph.BuildAliasIndex(root)
	if err != nil {
		t.Fatalf("BuildAliasIndex: %v", err)
	}
	m := idx.AliasMapFor("consumer.ts")
	if m["~/"] != "" || m["@/"] != "" {
		t.Errorf("no-config convention aliases = %+v, want {\"~/\":\"\", \"@/\":\"\"}", m)
	}
}

func TestBuildAliasIndex_TolerantJSONC(t *testing.T) {
	root := t.TempDir()
	// Comments + a trailing comma, as real-world tsconfig.json commonly has.
	mustWriteFile(t, filepath.Join(root, "tsconfig.json"), `{
		// alias config
		"compilerOptions": {
			"baseUrl": ".",
			"paths": {
				"~/*": ["./*"], /* trailing comma above, block comment here */
			},
		},
	}`)

	idx, err := graph.BuildAliasIndex(root)
	if err != nil {
		t.Fatalf("BuildAliasIndex: %v", err)
	}
	m := idx.AliasMapFor("consumer.ts")
	if m["~/"] != "" {
		t.Errorf("JSONC-tolerant alias map[\"~/\"] = %q, want %q", m["~/"], "")
	}
}

func TestBuildAliasIndex_TolerantJSONCPreservesCommaInsideString(t *testing.T) {
	root := t.TempDir()
	// The trailing comma after the "paths" object forces the tolerant JSONC
	// path (strict json.Unmarshal fails). The alias target string itself
	// contains a literal ",}" — a naive trailing-comma stripper that doesn't
	// track string-literal state would mistake that for a real trailing
	// comma before a closing brace and corrupt the string value.
	mustWriteFile(t, filepath.Join(root, "tsconfig.json"), `{
		"compilerOptions": {
			"baseUrl": ".",
			"paths": {
				"@weird/*": ["./glob,}pattern/*"],
			}
		},
	}`)

	idx, err := graph.BuildAliasIndex(root)
	if err != nil {
		t.Fatalf("BuildAliasIndex: %v", err)
	}
	m := idx.AliasMapFor("consumer.ts")
	want := "glob,}pattern"
	if m["@weird/"] != want {
		t.Errorf("alias target with embedded \",}\" in a string literal = %q, want %q (string content must survive intact)", m["@weird/"], want)
	}
}

func TestBuildAliasIndex_SkipsNodeModules(t *testing.T) {
	root := t.TempDir()
	// A tsconfig.json inside node_modules must never contribute an alias —
	// dependency trees aren't part of the workspace's own source.
	mustWriteFile(t, filepath.Join(root, "node_modules", "some-pkg", "tsconfig.json"), `{"compilerOptions":{"paths":{"~/*":["./should-not-appear/*"]}}}`)
	mustWriteFile(t, filepath.Join(root, "consumer.ts"), "export const x = 1;")

	idx, err := graph.BuildAliasIndex(root)
	if err != nil {
		t.Fatalf("BuildAliasIndex: %v", err)
	}
	m := idx.AliasMapFor("consumer.ts")
	if m["~/"] != "" {
		t.Errorf("node_modules tsconfig leaked into alias map: %+v", m)
	}
}

// --- ExtractEdgesWithImportContext: real TypeScriptGraphExtractor end-to-end
// against the committed testdata/import-fixture (AC-B1/B2/B3). ---

func TestTypeScriptExtractor_ExtractEdgesWithImportContext_ResolvesAgainstFixture(t *testing.T) {
	root := fixtureRoot(t)
	idx, err := graph.BuildAliasIndex(root)
	if err != nil {
		t.Fatalf("BuildAliasIndex: %v", err)
	}
	ex, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatalf("NewTypeScriptGraphExtractor: %v", err)
	}

	relFile := "repo-a/consumer.ts"
	content, err := os.ReadFile(filepath.Join(root, relFile))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	ic := graph.ImportContext{
		AliasMap: idx.AliasMapFor(relFile),
		Exists:   graph.DiskExistsChecker(root),
	}
	edges, err := ex.ExtractEdgesWithImportContext(relFile, content, ic)
	if err != nil {
		t.Fatalf("ExtractEdgesWithImportContext: %v", err)
	}

	targets := map[string]graph.Edge{}
	for _, e := range edges {
		if e.Kind == graph.EdgeImports {
			targets[e.TargetNode] = e
		}
	}

	// AC-B1: alias-resolved import.
	aliasEdge, ok := targets["repo-a/utils/enums.ts"]
	if !ok {
		t.Fatalf("no resolved edge for aliased import ~/utils/enums; got targets: %+v", targets)
	}
	if aliasEdge.Metadata["raw_specifier"] != "~/utils/enums" {
		t.Errorf("aliased edge metadata = %+v, want raw_specifier=~/utils/enums", aliasEdge.Metadata)
	}

	// AC-B2: relative-resolved import.
	relEdge, ok := targets["repo-a/sibling.ts"]
	if !ok {
		t.Fatalf("no resolved edge for relative import ./sibling; got targets: %+v", targets)
	}
	if relEdge.Metadata["raw_specifier"] != "./sibling" {
		t.Errorf("relative edge metadata = %+v, want raw_specifier=./sibling", relEdge.Metadata)
	}

	// AC-B3: bare package specifier stays unresolved and unchanged.
	if _, ok := targets["vue"]; !ok {
		t.Fatalf("bare package 'vue' import missing or resolved; got targets: %+v", targets)
	}
}

// TestTypeScriptExtractor_ExtractEdges_Unchanged pins down that the plain
// (no ImportContext) ExtractEdges path is completely unaffected by this
// feature — it must keep returning raw specifiers exactly as before.
func TestTypeScriptExtractor_ExtractEdges_Unchanged(t *testing.T) {
	root := fixtureRoot(t)
	ex, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatalf("NewTypeScriptGraphExtractor: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, "repo-a/consumer.ts"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	edges, err := ex.ExtractEdges("repo-a/consumer.ts", content)
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}

	raw := map[string]bool{}
	for _, e := range edges {
		if e.Kind == graph.EdgeImports {
			raw[e.TargetNode] = true
		}
	}
	for _, want := range []string{"~/utils/enums", "./sibling", "vue"} {
		if !raw[want] {
			t.Errorf("plain ExtractEdges missing raw target %q (edges: %+v)", want, raw)
		}
	}
}

// TestExtractEdgesWithImportContext_NoCrossRepoLeakage is the end-to-end
// version of the G3 guard: resolving both repo-a's and repo-b's consumer.ts
// against a SHARED AliasIndex built over the whole fixture root must not
// collide on the same target, proving nearest-ancestor selection is applied
// at extraction time, not just in the index lookup.
func TestExtractEdgesWithImportContext_NoCrossRepoLeakage(t *testing.T) {
	root := fixtureRoot(t)
	idx, err := graph.BuildAliasIndex(root)
	if err != nil {
		t.Fatalf("BuildAliasIndex: %v", err)
	}
	ex, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatalf("NewTypeScriptGraphExtractor: %v", err)
	}

	resolveOne := func(relFile string) string {
		content, err := os.ReadFile(filepath.Join(root, relFile))
		if err != nil {
			t.Fatalf("read fixture %s: %v", relFile, err)
		}
		ic := graph.ImportContext{AliasMap: idx.AliasMapFor(relFile), Exists: graph.DiskExistsChecker(root)}
		edges, err := ex.ExtractEdgesWithImportContext(relFile, content, ic)
		if err != nil {
			t.Fatalf("extract %s: %v", relFile, err)
		}
		for _, e := range edges {
			if e.Kind == graph.EdgeImports && e.Metadata["raw_specifier"] == "~/utils/enums" {
				return e.TargetNode
			}
		}
		t.Fatalf("no resolved ~/utils/enums edge found for %s", relFile)
		return ""
	}

	aTarget := resolveOne("repo-a/consumer.ts")
	bTarget := resolveOne("repo-b/consumer.ts")

	if aTarget != "repo-a/utils/enums.ts" {
		t.Errorf("repo-a resolved target = %q, want repo-a/utils/enums.ts", aTarget)
	}
	if bTarget != "repo-b/src/utils/enums.ts" {
		t.Errorf("repo-b resolved target = %q, want repo-b/src/utils/enums.ts", bTarget)
	}
	if aTarget == bTarget {
		t.Fatalf("repo-a and repo-b both resolved to %q — cross-repo collision", aTarget)
	}
}

func fixtureRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("testdata/import-fixture")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	return abs
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
