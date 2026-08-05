package graph

import (
	"errors"
	"testing"
)

func TestCallResolverPredicateMatchesContract(t *testing.T) {
	if !IsCanonicalJSCallTarget("src/api.ts::Api.run") {
		t.Fatal("canonical TypeScript class method was rejected")
	}
	if IsCanonicalJSCallTarget("Api::V2::StoriesController#sync") {
		t.Fatal("Ruby namespace was treated as a canonical JS/TS target")
	}
}

func TestLexicalScopeUsesInnermostBindingAndFunctionHoisting(t *testing.T) {
	module := newLexicalScope(nil, true)
	module.declareLexical("imported", "src/api.ts::run")
	module.declareFunction("hoisted", "src/local.ts::hoisted")

	function := module.childFunction()
	block := function.childBlock()
	block.declareLexical("imported", "")
	block.declareParameter("parameter")
	function.declareVar("legacy")

	cases := []struct {
		name  string
		scope *lexicalScope
		key   string
		want  string
	}{
		{"block shadows import", block, "imported", unresolvedCallTarget},
		{"parameter is unresolved", block, "parameter", unresolvedCallTarget},
		{"var is hoisted to function", block, "legacy", unresolvedCallTarget},
		{"function declaration is hoisted", block, "hoisted", "src/local.ts::hoisted"},
		{"unshadowed import reaches module", function, "imported", "src/api.ts::run"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.scope.lookup(tc.key)
			if tc.want == unresolvedCallTarget {
				if !got.unresolved {
					t.Fatalf("lookup(%q) = %#v, want unresolved", tc.key, got)
				}
				return
			}
			if got.target != tc.want || got.unresolved {
				t.Fatalf("lookup(%q) = %#v, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestDirectExportCatalogCachesOnlyThisExtraction(t *testing.T) {
	contents := map[string][]byte{
		"src/api.ts":      []byte("export function run() {} export class Api { run() {} }"),
		"src/reexport.ts": []byte("export { run } from './api'"),
	}
	reads := map[string]int{}
	imports := ImportContext{
		AliasMap: map[string]string{"@/": "src"},
		Exists:   func(path string) bool { _, ok := contents[path]; return ok },
		ReadFile: func(path string) ([]byte, error) {
			reads[path]++
			content, ok := contents[path]
			if !ok {
				return nil, errors.New("missing")
			}
			return content, nil
		},
	}

	resolver := newCallResolver("src/consumer.ts", imports)
	if got := resolver.resolveImported("./api", "Api", "run"); got != "src/api.ts::Api.run" {
		t.Fatalf("class method target = %q", got)
	}
	if got := resolver.resolveImported("./api", "run", ""); got != "src/api.ts::run" {
		t.Fatalf("function target = %q", got)
	}
	if got := resolver.resolveImported("@/api", "run", ""); got != "src/api.ts::run" {
		t.Fatalf("alias function target = %q", got)
	}
	if reads["src/api.ts"] != 1 {
		t.Fatalf("api module parsed %d times, want once", reads["src/api.ts"])
	}
	if got := resolver.resolveImported("./reexport", "run", ""); got != unresolvedCallTarget {
		t.Fatalf("re-export target = %q, want unresolved", got)
	}

	second := newCallResolver("src/consumer.ts", imports)
	if got := second.resolveImported("./api", "run", ""); got != "src/api.ts::run" {
		t.Fatalf("second extraction target = %q", got)
	}
	if reads["src/api.ts"] != 2 {
		t.Fatalf("cross-run cache retained module after %d reads", reads["src/api.ts"])
	}
}

func TestDirectReceiverOnlyResolvesCatalogedFieldMethod(t *testing.T) {
	receiver := directReceiver{fields: map[string]map[string]string{
		"api": {"run": "src/api.ts::Api.run"},
	}}
	if got := receiver.resolve("api", "run"); got != "src/api.ts::Api.run" {
		t.Fatalf("supported receiver target = %q", got)
	}
	if got := receiver.resolve("alias", "run"); got != unresolvedCallTarget {
		t.Fatalf("receiver alias target = %q, want unresolved", got)
	}
	if got := receiver.resolve("api", "dynamic"); got != unresolvedCallTarget {
		t.Fatalf("unsupported receiver member target = %q, want unresolved", got)
	}
}
