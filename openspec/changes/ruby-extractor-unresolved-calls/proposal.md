## Why

The Ruby call graph extractor (`ruby_extractor.go`) drops method calls where the target is not defined in the same file. This means controller files produce 0 call edges — all their calls go to models/services in other files. The cross-file resolver has nothing to rewrite → flows stop at 3 nodes (entry → handler → func).

The cross-file resolver (PR #469) was designed to resolve bare call targets to qualified cross-file paths. But it can only rewrite edges that exist. The extractor must emit the edges first.

## What Changes

- Emit unresolved call edges for ALL method invocations (not just same-file ones)
- The resolver rewrites bare targets to qualified cross-file targets
- Controller files will produce call edges → resolver resolves them → flows reach 5+ nodes

## Impact

- **Code affected**: `internal/graph/ruby_extractor.go` — `extractCalls` method (~5 lines changed)
- **Risk**: Low — existing same-file behavior preserved; new edges are additional
- **Verification**: rails-app flows should show controller→service→model chains (5+ nodes)
