# Task 2.3 — TypeScript contextual extraction (complete)

TypeScript contextual extraction now routes only
`ExtractEdgesWithImportContext` through the shared per-invocation resolver.
Plain `ExtractEdges` remains unchanged and emits legacy bare call targets.

Supported targets are direct named/default/namespace imports, named imported
class construction, same-file class methods including `this.method`, and
`this.field.method()` only for a simple-named class property or an
access-modified constructor parameter property. The type must be a same-file
class or a direct named/default project-local import. Class methods and direct
default exports receive matching `contains` declaration identities.

Parameters, locals, decorator-based forms, interfaces, unions, generics,
`any`, inferred fields, external/missing imports, computed calls, aliases, and
shadows produce target-only `<unresolved>`.

Nested non-arrow functions reset the class receiver context, so their
`this.field.method()` calls are unresolved. Arrow functions retain lexical
`this` and preserve admitted typed-receiver behavior.

Verification on 2026-08-05:

```text
go test -race ./internal/graph -run 'TestTypeScript.*(Contextual|Call|Receiver|Import|Unresolved)' -count=1 -v  PASS
go test -race ./internal/graph -run 'TestJavaScript.*(Contextual|Call|Class|Import|Unresolved)|TestPlain(JavaScript|TypeScript)ExtractorKeepsBareCallTargets' -count=1  PASS
go test -race -count=1 ./internal/graph -run 'Test(TypeScriptContextual|JavaScriptContextual|SourceScoped|Canonical|JavaScriptAndTypeScript|ResolveImportTarget_Supports)'  PASS
git diff --check  PASS
gofmt -d [task files]  clean
```

Reader safety and watcher lifecycle controls are now included in the issue
branch; focused storage, flow, PageRank, graph-context, handler, and watcher
tests pass. The repository-wide integration command still reports unrelated
baseline failures in workspace-resolution, harvest, summary, and benchmark
packages; those failures do not occur in the issue-related graph/storage/
watcher/flow/codesummarize tests.
