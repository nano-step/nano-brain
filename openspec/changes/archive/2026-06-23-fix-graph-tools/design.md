## Context

The code intelligence graph tools work for Go/TypeScript/JavaScript but fail for Ruby on Rails because:

1. **Association edges not extracted**: `has_many :users` extracts `:users` as target (symbol), not `User` (model class)
2. **Callback edges not extracted**: `before_action :authenticate` extracts `:authenticate` as target, not `UsersController#authenticate`
3. **Flowchart format mismatch**: Tool expects `file::startLine-endLine` but Ruby uses `file.rb::ClassName#method`
4. **Impact analysis depends on edges**: If edges don't exist, impact returns 0

Current state:
- `RailsDSLEdgeExtractor` exists (451 lines, 16 tests)
- Associations are extracted but target is symbol name, not model class
- Callbacks are extracted but target is method name, not qualified method

## Goals / Non-Goals

**Goals:**
- Fix association edge targets to use model class names (e.g., `User` instead of `:users`)
- Fix callback edge targets to use qualified method names (e.g., `UsersController#authenticate`)
- Support `file.rb::ClassName#method` format in `memory_flowchart`
- Ensure graph edges exist for impact analysis
- Improve flow depth to trace into controller method bodies

**Non-Goals:**
- Add new edge types (e.g., `has_many :through` intermediate tables)
- Support Rails DSLs beyond associations/callbacks (e.g., `validates`, `scope`)
- Change existing edge format for Go/TypeScript/JavaScript

## Decisions

### 1. Association Target Resolution

**Decision**: Use a simple mapping approach: `has_many :users` → target = `User` (capitalize + singularize)

**Rationale**: 
- Simple, predictable, covers 80% of cases
- No need for full Rails environment or model file parsing
- Edge metadata stores the original symbol for reference

**Alternatives considered**:
- Parse model files to verify class exists → Too complex, requires full project context
- Use inflection library for singularization → External dependency, adds complexity
- Store symbol as-is → Doesn't help graph traversal

### 2. Callback Target Resolution

**Decision**: `before_action :authenticate` → target = `UsersController#authenticate` (current controller + method name)

**Rationale**:
- Callbacks reference methods in the same controller or included concerns
- Simple string interpolation, no AST analysis needed
- Edge metadata stores the callback type for filtering

**Alternatives considered**:
- Search all controllers for the method → Too slow, requires cross-file resolution
- Store method name only → Ambiguous, multiple controllers could have same method
- Ignore callbacks → Loses important Rails DSL information

### 3. Flowchart Format Support

**Decision**: Accept both `file.rb::ClassName#method` and `file::startLine-endLine` formats

**Rationale**:
- Backward compatible with existing JS/TS format
- Ruby uses Class#method as entry point, not line numbers
- Store both formats in database for lookup

**Alternatives considered**:
- Convert Class#method to line numbers at query time → Requires AST parsing
- Store only line numbers → Loses Ruby semantics
- Store only Class#method → Breaks existing JS/TS support

### 4. Edge Extraction Approach

**Decision**: Extend `RailsDSLEdgeExtractor` with target resolution logic

**Rationale**:
- Reuse existing infrastructure (tree-sitter parsing, edge storage)
- No new extractors needed
- Metadata preserves original values for debugging

**Alternatives considered**:
- Create new `RailsAssociationEdgeExtractor` → Overkill, same logic
- Modify `RubyGraphExtractor` → Mixes concerns, harder to maintain
- Add post-processing step → Adds complexity, error-prone

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Association target resolution may be wrong for unconventional names | Store original symbol in metadata, log warnings for manual review |
| Callbacks referencing included concerns may not resolve | Document limitation, add support later if needed |
| Re-indexing required for existing Rails workspaces | Provide migration script, document in changelog |
| Flow depth increase may impact performance | Limit to 5 levels, cache results |

## Migration Plan

1. **Rebuild index**: Run `POST /api/v1/reindex` for all Rails workspaces
2. **Verify edges**: Check `memory_graph` returns associations/callbacks
3. **Test tools**: Run `memory_flowchart`, `memory_impact`, `memory_flow` on known methods
4. **Monitor**: Check logs for resolution failures, adjust if needed

## Open Questions

- Should we support `has_many :through` with intermediate table resolution?
- Should we add `validates` and `scope` as edge types?
- How to handle concerns that are included in multiple controllers?
