## Context

The Ruby call graph extractor (`internal/graph/ruby_extractor.go`) is per-file, same-file-only. The `extractCalls` method builds a `methodNames` map from same-file method declarations, then drops any callee not in that map. The result: `POST /signup → TokensController#signup` (2 nodes).

The FlowBuilder already has partial resolution via `bySymbol` index (bare name → all source nodes defining that symbol). But it's limited by `maxReconcileFiles=8` and same-file preference.

## Goals / Non-Goals

**Goals:**
- Enrich `contains` query to capture class/module definitions
- Build class→file index from enriched contains edges
- Resolve cross-file qualified calls (`ClassName.method`, `ClassName.new.method`)
- Emit qualified `TargetNode` format for resolved calls
- Emit bare edge + metadata for unresolved calls
- Handle ActiveRecord `ClassName.method` patterns (where, create, etc.)

**Non-Goals (deferred to v2):**
- ActiveRecord dynamic finders (`find_by_*`) as fully resolved
- Concern/extend/include resolution
- `before_action`/`after_action` callback chains
- Metaprogramming (`method_missing`, `define_method`, `send`)
- Non-Rails Ruby (keep `RequiresFrameworks: ["rails"]` gate)

## Decisions

### Decision 1: Post-Extraction Resolution Pass

**Choice**: Resolver runs AFTER `scanCollection` completes for all .rb files in a workspace. NOT per-file.

**Rationale**: The resolver needs a complete class→file index built from ALL files' `contains` edges. Per-file resolution would have an incomplete index (classes in not-yet-indexed files would be missed). After-collection resolution gives the full picture.

**Integration**: New `resolveRubyEdges(ctx, workspaceHash)` function called from the watcher after the file loop completes. Builds class→file index from existing contains edges, then resolves all unresolved call edges.

**Alternatives considered**:
- Extraction-time resolution: Rejected — extractors become stateful, require cross-file context
- Per-file resolution: Rejected — incomplete index, misses classes in not-yet-indexed files
- Query-time resolution: Rejected — every query pays the cost, inconsistent edge visibility

### Decision 2: Qualified TargetNode for Resolved Calls

**Choice**: Resolved cross-file calls emit `TargetNode = "file.rb::method"` (qualified). Unresolved calls emit bare `TargetNode = "method"` + metadata `{"unresolved": true}`.

**Rationale**: Qualified format bypasses the flow builder's `bySymbol` reconciliation (which caps at 8 files for generic names like `save`, `where`). The flow builder's BFS already handles qualified names via `bySource` exact match.

**Alternatives considered**:
- Emit both bare and qualified: Rejected — doubles edge count, creates query ambiguity
- Bare only + query-time resolution: Rejected — no query-time resolution exists

### Decision 3: Contains Edge Enrichment

**Choice**: Add `(class name: (constant) @name)` and `(module name: (constant) @name)` to the Ruby `contains` tree-sitter query.

**Rationale**: The symbol extractor (`internal/symbol/ruby_extractor.go`) already captures class/module names, but the graph extractor doesn't. Both need to agree. Enriching the graph extractor's `contains` query is the cleanest path.

**Alternatives considered**:
- Build index from symbol extractor output: Rejected — different package, different storage path, harder to wire
- New `defines` edge type: Rejected — overengineering, `contains` already implies "defines"

### Decision 4: Rails Naming Convention Fallback

**Choice**: When a class name isn't found in the contains index, apply Rails autoloading conventions (Zeitwerk):
- `User` → `app/models/user.rb`
- `PaymentProcessor` → `app/services/payment_processor.rb`
- `UsersController` → `app/controllers/users_controller.rb`

**Rationale**: Many Rails classes follow naming conventions. The fallback catches cases where the class is defined in a gem or external dependency (not indexed).

**Alternatives considered**:
- No fallback: Rejected — too many false negatives
- Full Zeitwerk resolver: Deferred to v2 — complex, requires understanding app structure

### Decision 5: Emit Qualified TargetNode Only (No Bare Duplicate)

**Choice**: Resolved calls emit ONLY the qualified format. No bare edge alongside.

**Rationale**: Prevents double-counting in "who calls X" queries. Clean migration path. The flow builder handles qualified names natively.

### Decision 6: FlowBuilder Handler Reconciliation (NEW — Momus finding)

**Choice**: The resolver emits `reconcile` edges that bridge the HTTP handler format (`Controller#action`) to the file::method format.

**Problem**: The flow builder's BFS starts with `bySymbol["UsersController#create"]` (from HTTP edge TargetNode). But Ruby source nodes use `SourceNode = "file.rb::method"`, so `symbolPart` returns `"method"`, not `"Controller#action"`. They never match → BFS terminates at handler → no call chain.

**Solution**: The resolver produces reconciliation edges:
```
SourceNode: "UsersController#create"     (matches HTTP edge TargetNode)
TargetNode: "app/controllers/users_controller.rb::create"  (matches contains edge)
Kind: "reconcile"  (new edge kind, only used by flow builder)
```

The flow builder's BFS can then follow: `Controller#action` → reconcile edge → `file.rb::method` → outgoing calls → next handler.

**Implementation**: Add `EdgeReconcile` to EdgeKind enum. Flow builder treats reconcile edges as transparent pass-through during BFS.

## Risks / Trade-offs

### Risk 1: Class Reopening
**Risk**: Ruby allows multiple files to define the same class ( reopen). Multiple index entries for the same class name.
**Mitigation**: Store all candidates. Emit edges to all matching files. Add metadata `"ambiguous": true`.

### Risk 2: Contains Parser Accuracy
**Risk**: If class/module capture in `contains` query is wrong, the entire index is wrong.
**Mitigation**: Comprehensive test fixtures. Verify against real Rails repos.

### Risk 3: Scope Creep into ActiveRecord
**Risk**: AR's `method_missing` creates genuinely unresolvable calls.
**Mitigation**: Hard boundary — treat AR dynamic methods as opaque. Emit bare + metadata. Document limitation.

### Risk 4: Reconcile Edge Overhead
**Risk**: Every controller action produces a reconcile edge, doubling edge count for handlers.
**Mitigation**: Reconcile edges are cheap (2 fields). They add minimal storage and enable the flow builder without changes.

### Trade-off: Completeness vs Reliability
We resolve what we can statically determine (qualified method calls, ActiveRecord class-level methods). Dynamic dispatch is acknowledged as a known limitation.
