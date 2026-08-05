package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func TestTypeScriptContextualCallImportsAndMethods(t *testing.T) {
	root := t.TempDir()
	writeContractFixture(t, root, "repo-a/lib/api.ts", "export function run() {}\nexport function from() {}\nexport const callback = () => {};\nexport class Api { run() {} }\n")
	writeContractFixture(t, root, "repo-a/lib/default.ts", "export default class DefaultApi { run() {} }\n")
	writeContractFixture(t, root, "repo-a/lib/anonymous-default.ts", "export default class { run() {} }\n")
	writeContractFixture(t, root, "repo-a/lib/multi.ts", "export const first = () => {}, second = () => {};\n")
	writeContractFixture(t, root, "repo-b/lib/api.ts", "export function run() {}\n")

	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "same-root-collision-named-alias",
			source: "import { run as execute } from \"./lib/api\"; function caller() { execute(); }",
			want:   []string{"repo-a/lib/api.ts::run"},
		},
		{
			name:   "direct-default-method",
			source: "import service from \"./lib/default\"; function caller() { service.run(); }",
			want:   []string{"repo-a/lib/default.ts::default.run"},
		},
		{
			name:   "direct-const-export",
			source: "import { callback } from \"./lib/api\"; function caller() { callback(); }",
			want:   []string{"repo-a/lib/api.ts::callback"},
		},
		{
			name:   "multi-declarator-value-export",
			source: "import { second } from \"./lib/multi\"; function caller() { second(); }",
			want:   []string{"repo-a/lib/multi.ts::second"},
		},
		{
			name:   "imported-name-from-does-not-confuse-clause-parser",
			source: "import { from as execute } from \"./lib/api\"; function caller() { execute(); }",
			want:   []string{"repo-a/lib/api.ts::from"},
		},
		{
			name:   "namespace-direct-export",
			source: "import * as api from \"./lib/api\"; function caller() { api.run(); }",
			want:   []string{"repo-a/lib/api.ts::run"},
		},
		{
			name:   "named-class-method",
			source: "import { Api } from \"./lib/api\"; function caller() { new Api().run(); }",
			want:   []string{"repo-a/lib/api.ts::Api.run"},
		},
		{
			name:   "external-package-is-unresolved",
			source: "import { run } from \"external-pkg\"; function caller() { run(); }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "missing-project-import-is-unresolved",
			source: "import { run } from \"./lib/missing\"; function caller() { run(); }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "anonymous-default-class-method-is-unresolved",
			source: "import service from \"./lib/anonymous-default\"; function caller() { service.run(); }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "call-in-variable-initializer",
			source: "function caller() { const value = run(); } function run() {}",
			want:   []string{"repo-a/consumer.ts::run"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireCallTargets(t, contextualCallTargets(t, "repo-a/consumer.ts", tc.source, root), tc.want)
		})
	}
}

func TestTypeScriptContextualCallDeclarationIdentities(t *testing.T) {
	extractor, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatalf("NewTypeScriptGraphExtractor: %v", err)
	}
	cases := []struct {
		name   string
		file   string
		source string
		want   []string
	}{
		{
			name:   "direct-named-exports-and-class-method",
			file:   "repo-a/lib/api.ts",
			source: "export function run() {} export const callback = () => {}; export class Api { run() {} }",
			want:   []string{"repo-a/lib/api.ts::run", "repo-a/lib/api.ts::callback", "repo-a/lib/api.ts::Api.run"},
		},
		{
			name:   "direct-default-export-and-method",
			file:   "repo-a/lib/default.ts",
			source: "export default class DefaultApi { run() {} }",
			want:   []string{"repo-a/lib/default.ts::default", "repo-a/lib/default.ts::default.run"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			edges, err := extractor.ExtractEdges(tc.file, []byte(tc.source))
			if err != nil {
				t.Fatalf("ExtractEdges: %v", err)
			}
			contained := make(map[string]bool)
			for _, edge := range edges {
				if edge.Kind == graph.EdgeContains {
					contained[edge.TargetNode] = true
				}
			}
			for _, want := range tc.want {
				if !contained[want] {
					t.Errorf("missing declaration identity %q; got %v", want, contained)
				}
			}
		})
	}
}

func TestJavaScriptContextualCallImportsAndExclusions(t *testing.T) {
	root := t.TempDir()
	apiSource := "export function run() {} export function from() {} export class Api { run() {} }"
	defaultSource := "export default class DefaultApi { run() {} }"
	anonymousDefaultSource := "export default class { run() {} }"
	writeContractFixture(t, root, "repo-a/lib/api.js", apiSource)
	writeContractFixture(t, root, "repo-a/lib/default.js", defaultSource)
	writeContractFixture(t, root, "repo-a/lib/anonymous-default.js", anonymousDefaultSource)
	writeContractFixture(t, root, "repo-a/lib/multi.js", "export const first = () => {}, second = () => {};\n")

	cases := []struct {
		name              string
		source            string
		want              []string
		declarationFile   string
		declarationSource string
		declarationTarget string
	}{
		{
			name:              "named-direct-export",
			source:            "import { run as execute } from \"./lib/api\"; function caller() { execute(); }",
			want:              []string{"repo-a/lib/api.js::run"},
			declarationFile:   "repo-a/lib/api.js",
			declarationSource: apiSource,
			declarationTarget: "repo-a/lib/api.js::run",
		},
		{
			name:              "imported-name-from-does-not-confuse-clause-parser",
			source:            "import { from as execute } from \"./lib/api\"; function caller() { execute(); }",
			want:              []string{"repo-a/lib/api.js::from"},
			declarationFile:   "repo-a/lib/api.js",
			declarationSource: apiSource,
			declarationTarget: "repo-a/lib/api.js::from",
		},
		{
			name:              "default-class-method",
			source:            "import service from \"./lib/default\"; function caller() { service.run(); }",
			want:              []string{"repo-a/lib/default.js::default.run"},
			declarationFile:   "repo-a/lib/default.js",
			declarationSource: defaultSource,
			declarationTarget: "repo-a/lib/default.js::default.run",
		},
		{
			name:   "multi-declarator-value-export",
			source: "import { second } from \"./lib/multi\"; function caller() { second(); }",
			want:   []string{"repo-a/lib/multi.js::second"},
		},
		{
			name:   "anonymous-default-class-method-is-unresolved",
			source: "import service from \"./lib/anonymous-default\"; function caller() { service.run(); }",
			want:   []string{"<unresolved>"},
		},
		{
			name:              "namespace-direct-export",
			source:            "import * as api from \"./lib/api\"; function caller() { api.run(); }",
			want:              []string{"repo-a/lib/api.js::run"},
			declarationFile:   "repo-a/lib/api.js",
			declarationSource: apiSource,
			declarationTarget: "repo-a/lib/api.js::run",
		},
		{
			name:              "named-class-method",
			source:            "import { Api } from \"./lib/api\"; function caller() { new Api().run(); }",
			want:              []string{"repo-a/lib/api.js::Api.run"},
			declarationFile:   "repo-a/lib/api.js",
			declarationSource: apiSource,
			declarationTarget: "repo-a/lib/api.js::Api.run",
		},
		{
			name:   "parameter-shadow-is-unresolved",
			source: "import { run } from \"./lib/api\"; function caller(run) { run(); }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "namespace-parameter-shadow-is-unresolved",
			source: "import * as api from \"./lib/api\"; function caller(api) { api.run(); }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "default-local-shadow-is-unresolved",
			source: "import service from \"./lib/default\"; function caller() { const service = {}; service.run(); }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "named-class-parameter-shadow-is-unresolved",
			source: "import { Api } from \"./lib/api\"; function caller(Api) { new Api().run(); }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "catch-namespace-shadow-is-unresolved",
			source: "import * as api from \"./lib/api\"; function caller() { try {} catch (api) { api.run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "nested-default-parameter-shadow-is-unresolved",
			source: "import service from \"./lib/default\"; function caller() { function nested(service) { service.run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "computed-member-is-unresolved",
			source: "import * as api from \"./lib/api\"; function caller() { api[\"run\"](); }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "unknown-local-class-method-is-unresolved",
			source: "class Api { run() {} } function caller() { new Api().missing(); }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "nested-function-does-not-inherit-class-this",
			source: "class Controller { run() {} caller() { function nested() { this.run(); } } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "call-in-variable-initializer",
			source: "function caller() { const value = run(); } function run() {}",
			want:   []string{"repo-a/consumer.js::run"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireCallTargets(t, javascriptContextualCallTargets(t, "repo-a/consumer.js", tc.source, root), tc.want)
			if tc.declarationTarget != "" {
				requireJavaScriptDeclarationIdentity(t, tc.declarationFile, tc.declarationSource, tc.declarationTarget)
			}
		})
	}
}

func requireJavaScriptDeclarationIdentity(t *testing.T, filePath, source, want string) {
	t.Helper()
	extractor, err := graph.NewJavaScriptGraphExtractor()
	if err != nil {
		t.Fatalf("NewJavaScriptGraphExtractor: %v", err)
	}
	edges, err := extractor.ExtractEdges(filePath, []byte(source))
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}
	for _, edge := range edges {
		if edge.Kind == graph.EdgeContains && edge.TargetNode == want {
			return
		}
	}
	t.Fatalf("missing declaration identity %q", want)
}

func TestJavaScriptContextualCallDeclarationIdentities(t *testing.T) {
	extractor, err := graph.NewJavaScriptGraphExtractor()
	if err != nil {
		t.Fatalf("NewJavaScriptGraphExtractor: %v", err)
	}
	cases := []struct {
		name   string
		file   string
		source string
		want   []string
	}{
		{
			name:   "named-export-and-class-method",
			file:   "repo-a/lib/api.js",
			source: "export function run() {} export class Api { run() {} }",
			want:   []string{"repo-a/lib/api.js::run", "repo-a/lib/api.js::Api.run"},
		},
		{
			name:   "default-export-and-method",
			file:   "repo-a/lib/default.js",
			source: "export default class DefaultApi { run() {} }",
			want:   []string{"repo-a/lib/default.js::default", "repo-a/lib/default.js::default.run"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			edges, err := extractor.ExtractEdges(tc.file, []byte(tc.source))
			if err != nil {
				t.Fatalf("ExtractEdges: %v", err)
			}
			contained := make(map[string]bool)
			for _, edge := range edges {
				if edge.Kind == graph.EdgeContains {
					contained[edge.TargetNode] = true
				}
			}
			for _, want := range tc.want {
				if !contained[want] {
					t.Errorf("missing declaration identity %q; got %v", want, contained)
				}
			}
		})
	}
}

func TestTypeScriptContextualCallReceiversAndScopes(t *testing.T) {
	root := t.TempDir()
	writeContractFixture(t, root, "repo-a/lib/api.ts", "export class Api { run() {} }\nexport function run() {}\n")
	writeContractFixture(t, root, "repo-a/lib/default.ts", "export default class DefaultApi { run() {} }\n")

	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "same-class-this-method",
			source: "class Controller { run() {} caller() { this.run(); } }",
			want:   []string{"repo-a/consumer.ts::Controller.run"},
		},
		{
			name:   "unknown-this-method-is-unresolved",
			source: "class Controller { caller() { this.run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "same-file-typed-class-property",
			source: "class Api { run() {} } class Controller { api: Api; caller() { this.api.run(); } }",
			want:   []string{"repo-a/consumer.ts::Api.run"},
		},
		{
			name:   "forward-declared-same-file-typed-class-property",
			source: "class Controller { api: Api; caller() { this.api.run(); } } class Api { run() {} }",
			want:   []string{"repo-a/consumer.ts::Api.run"},
		},
		{
			name:   "direct-named-import-typed-class-property",
			source: "import { Api } from \"./lib/api\"; class Controller { api: Api; caller() { this.api.run(); } }",
			want:   []string{"repo-a/lib/api.ts::Api.run"},
		},
		{
			name:   "direct-default-import-typed-class-property",
			source: "import Service from \"./lib/default\"; class Controller { service: Service; caller() { this.service.run(); } }",
			want:   []string{"repo-a/lib/default.ts::default.run"},
		},
		{
			name:   "access-modified-constructor-parameter-property",
			source: "class Api { run() {} } class Controller { constructor(private api: Api) {} caller() { this.api.run(); } }",
			want:   []string{"repo-a/consumer.ts::Api.run"},
		},
		{
			name:   "plain-constructor-parameter-is-unresolved",
			source: "class Api { run() {} } class Controller { constructor(api: Api) {} caller() { this.api.run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "interface-typed-property-is-unresolved",
			source: "interface Api { run(): void; } class Controller { api: Api; caller() { this.api.run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "union-typed-property-is-unresolved",
			source: "class Api { run() {} } class Other { run() {} } class Controller { api: Api | Other; caller() { this.api.run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "generic-typed-property-is-unresolved",
			source: "class Api<T> { run() {} } class Controller { api: Api<string>; caller() { this.api.run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "any-typed-property-is-unresolved",
			source: "class Controller { api: any; caller() { this.api.run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "inferred-property-is-unresolved",
			source: "class Api { run() {} } class Controller { api = new Api(); caller() { this.api.run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "decorated-property-is-unresolved",
			source: "class Api { run() {} } class Controller { @Inject() api: Api; caller() { this.api.run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "decorated-parameter-property-is-unresolved",
			source: "class Api { run() {} } class Controller { constructor(@Inject() private api: Api) {} caller() { this.api.run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "typed-local-receiver-is-unresolved",
			source: "class Api { run() {} } class Controller { caller() { const api: Api = new Api(); api.run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "nested-function-receiver-is-unresolved",
			source: "class Api { run() {} } class Controller { api: Api; caller() { function nested() { this.api.run(); } } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "nested-arrow-receiver-retains-lexical-this",
			source: "class Api { run() {} } class Controller { api: Api; caller() { const nested = () => { this.api.run(); }; } }",
			want:   []string{"repo-a/consumer.ts::Api.run"},
		},
		{
			name:   "function-declaration-hoisting",
			source: "function caller() { run(); } function run() {}",
			want:   []string{"repo-a/consumer.ts::run"},
		},
		{
			name:   "parameter-shadow-is-unresolved",
			source: "import { run } from \"./lib/api\"; function caller(run: () => void) { run(); }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "block-let-shadow-is-unresolved",
			source: "import { run } from \"./lib/api\"; function caller() { { let run = () => {}; run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "catch-binding-shadow-is-unresolved",
			source: "import * as api from \"./lib/api\"; function caller() { try {} catch (api) { api.run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "var-binding-is-unresolved",
			source: "import { run } from \"./lib/api\"; function caller() { var run; run(); }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "nested-function-boundary-is-unresolved",
			source: "import { run } from \"./lib/api\"; function caller() { function nested(run: () => void) { run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "receiver-alias-is-unresolved",
			source: "class Api { run() {} } class Controller { api: Api; caller() { const receiver = this.api; receiver.run(); } }",
			want:   []string{"<unresolved>"},
		},
		{
			name:   "computed-member-is-unresolved",
			source: "import * as api from \"./lib/api\"; function caller() { api[\"run\"](); }",
			want:   []string{"<unresolved>"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireCallTargets(t, contextualCallTargets(t, "repo-a/consumer.ts", tc.source, root), tc.want)
		})
	}
}

func TestTypeScriptContextualCallExporterChange(t *testing.T) {
	root := t.TempDir()
	consumer := "import { run } from \"./lib/api\"; function caller() { run(); }"
	exporter := filepath.Join(root, "repo-a", "lib", "api.ts")
	writeContractFixture(t, root, "repo-a/lib/api.ts", "export function run() {}\n")

	requireCallTargets(t, contextualCallTargets(t, "repo-a/consumer.ts", consumer, root), []string{"repo-a/lib/api.ts::run"})
	if err := os.Remove(exporter); err != nil {
		t.Fatalf("Remove(exporter): %v", err)
	}
	requireCallTargets(t, contextualCallTargets(t, "repo-a/consumer.ts", consumer, root), []string{"<unresolved>"})
}

func TestPlainTypeScriptExtractorKeepsBareCallTargets(t *testing.T) {
	extractor, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatalf("NewTypeScriptGraphExtractor: %v", err)
	}
	edges, err := extractor.ExtractEdges("consumer.ts", []byte("function caller() { run(); }"))
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}
	for _, edge := range edges {
		if edge.Kind == graph.EdgeCalls && edge.TargetNode == "run" {
			return
		}
	}
	t.Fatal("plain ExtractEdges did not retain the legacy bare target run")
}

func TestPlainJavaScriptExtractorKeepsBareCallTargets(t *testing.T) {
	extractor, err := graph.NewJavaScriptGraphExtractor()
	if err != nil {
		t.Fatalf("NewJavaScriptGraphExtractor: %v", err)
	}
	edges, err := extractor.ExtractEdges("consumer.js", []byte("function caller() { run(); }"))
	if err != nil {
		t.Fatalf("ExtractEdges: %v", err)
	}
	for _, edge := range edges {
		if edge.Kind == graph.EdgeCalls && edge.TargetNode == "run" {
			return
		}
	}
	t.Fatal("plain ExtractEdges did not retain the legacy bare target run")
}
