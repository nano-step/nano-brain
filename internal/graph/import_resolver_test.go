package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/rs/zerolog"
)

func TestImportClassifiers(t *testing.T) {
	tests := []struct {
		spec       string
		isRelative bool
		isAlias    bool
		isBare     bool
	}{
		{"./foo", true, false, false},
		{"../foo/bar", true, false, false},
		{".", true, false, false},
		{"~/utils/enums", false, true, false},
		{"~utils/enums", false, true, false},
		{"@/components/Btn", false, true, false},
		{"@org/pkg", false, false, true},      // scoped npm package, NOT alias
		{"@org/pkg/sub", false, false, true},  // scoped npm subpath
		{"lodash", false, false, true},        // bare
		{"lodash/fp", false, false, true},     // bare subpath
		{"vue", false, false, true},           // bare
	}
	for _, tc := range tests {
		t.Run(tc.spec, func(t *testing.T) {
			if got := graph.IsRelativeImport(tc.spec); got != tc.isRelative {
				t.Errorf("IsRelativeImport(%q) = %v, want %v", tc.spec, got, tc.isRelative)
			}
			if got := graph.IsAliasImport(tc.spec); got != tc.isAlias {
				t.Errorf("IsAliasImport(%q) = %v, want %v", tc.spec, got, tc.isAlias)
			}
			if got := graph.IsBarePackage(tc.spec); got != tc.isBare {
				t.Errorf("IsBarePackage(%q) = %v, want %v", tc.spec, got, tc.isBare)
			}
		})
	}
}

func TestResolveImportPath(t *testing.T) {
	logger := zerolog.Nop()
	// alias: "~" and "@" both map to workspace root ("").
	alias := graph.AliasMap{"~": "", "@/": ""}

	// fileSet is the set of workspace-relative files that "exist".
	fileSet := map[string]bool{
		"utils/enums.ts":          true,
		"composables/useThing.ts": true,
		"src/c/d.ts":              true,
		"components/Btn/index.ts": true,
		"shared/widget.vue":       true,
	}
	exists := func(rel string) bool { return fileSet[rel] }

	tests := []struct {
		name       string
		spec       string
		sourceFile string
		want       string
	}{
		{
			name:       "alias maps to workspace-relative file",
			spec:       "~/utils/enums",
			sourceFile: "composables/useThing.ts",
			want:       "utils/enums.ts",
		},
		{
			name:       "@ alias resolves to .vue",
			spec:       "@/shared/widget",
			sourceFile: "pages/Home.vue",
			want:       "shared/widget.vue",
		},
		{
			name:       "relative ./ resolves against source dir",
			spec:       "./useThing",
			sourceFile: "composables/index.ts",
			want:       "composables/useThing.ts",
		},
		{
			name:       "relative ../ resolves against source dir",
			spec:       "../c/d",
			sourceFile: "src/a/b.ts",
			want:       "src/c/d.ts",
		},
		{
			name:       "alias to directory resolves via index file",
			spec:       "~/components/Btn",
			sourceFile: "pages/Home.vue",
			want:       "components/Btn/index.ts",
		},
		{
			name:       "scoped npm package passes through (not alias)",
			spec:       "@org/pkg",
			sourceFile: "src/a/b.ts",
			want:       "@org/pkg",
		},
		{
			name:       "scoped npm subpath passes through",
			spec:       "@org/pkg/sub",
			sourceFile: "src/a/b.ts",
			want:       "@org/pkg/sub",
		},
		{
			name:       "bare package passes through",
			spec:       "lodash",
			sourceFile: "src/a/b.ts",
			want:       "lodash",
		},
		{
			name:       "bare package subpath passes through",
			spec:       "lodash/fp",
			sourceFile: "src/a/b.ts",
			want:       "lodash/fp",
		},
		{
			name:       "miss falls back to raw spec (not extensionless half-path)",
			spec:       "~/utils/missing",
			sourceFile: "composables/useThing.ts",
			want:       "~/utils/missing",
		},
		{
			name:       "relative miss falls back to raw spec",
			spec:       "./nope",
			sourceFile: "composables/useThing.ts",
			want:       "./nope",
		},
		{
			name:       "extensionless unresolved alias never returns half-path",
			spec:       "@/does/not/exist",
			sourceFile: "pages/Home.vue",
			want:       "@/does/not/exist",
		},
		{
			name:       "relative climbing above root falls back to raw",
			spec:       "../../outside/thing",
			sourceFile: "a/b.ts",
			want:       "../../outside/thing",
		},
		{
			name:       "bare .. at root does not probe outside workspace",
			spec:       "..",
			sourceFile: "a.ts",
			want:       "..",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := graph.ResolveImportPath(tc.spec, tc.sourceFile, "/ws", alias, exists, logger)
			if got != tc.want {
				t.Errorf("ResolveImportPath(%q, %q) = %q, want %q", tc.spec, tc.sourceFile, got, tc.want)
			}
		})
	}
}

func TestResolveImportPath_AliasPrefixNotInMapFallsBackToRaw(t *testing.T) {
	logger := zerolog.Nop()
	// alias map has only "@/", so a "~" import is a recognized alias shape with
	// no mapping -> must fall back to the raw spec, never a half-path.
	alias := graph.AliasMap{"@/": ""}
	exists := func(string) bool { return true } // even if files exist, no mapping
	got := graph.ResolveImportPath("~/utils/enums", "a/b.ts", "/ws", alias, exists, logger)
	if got != "~/utils/enums" {
		t.Errorf("unmapped alias = %q, want raw \"~/utils/enums\"", got)
	}
}

func TestResolveImportPath_AmbiguousPrefersTSOverJS(t *testing.T) {
	logger := zerolog.Nop()
	alias := graph.AliasMap{"~": ""}
	// Both .ts and .js exist; probe order must pick .ts deterministically.
	fileSet := map[string]bool{
		"shared/util.ts": true,
		"shared/util.js": true,
	}
	exists := func(rel string) bool { return fileSet[rel] }
	got := graph.ResolveImportPath("~/shared/util", "a/b.ts", "/ws", alias, exists, logger)
	if got != "shared/util.ts" {
		t.Errorf("ambiguous .ts/.js probe = %q, want shared/util.ts", got)
	}
}

func TestLoadAliasMap_FromTSConfig(t *testing.T) {
	logger := zerolog.Nop()
	// The committed fixture maps ~/* and @/* to "./*" (workspace root).
	am := graph.LoadAliasMap("testdata/alias-import", logger)
	if got := am["~"]; got != "" {
		t.Errorf("alias ~ base = %q, want \"\" (workspace root)", got)
	}
	if got, ok := am["@/"]; !ok || got != "" {
		t.Errorf("alias @/ base = %q (ok=%v), want \"\" (workspace root)", got, ok)
	}
}

func TestLoadAliasMap_NoConfigYieldsConventionFallback(t *testing.T) {
	logger := zerolog.Nop()
	// testdata/simple has no tsconfig and (presumably) no app/ dir -> root.
	am := graph.LoadAliasMap("testdata/simple", logger)
	if _, ok := am["~"]; !ok {
		t.Errorf("expected a ~ alias from Nuxt convention fallback, got %#v", am)
	}
}
