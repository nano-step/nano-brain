## Context

The Ruby call graph extractor (`ruby_extractor.go`) drops method calls where the target is not defined in the same file (line 186-188). Controller files produce 0 call edges because all their calls go to models/services in other files. The cross-file resolver passes through all input edges unchanged, so it has nothing to work with for controller files.

## Decisions

### Decision 1: Remove the methodNames guard

**Choice**: Remove the `if !methodNames[callee] { continue }` block entirely.

**Rationale**: Same-file calls already get resolved edges through the existing path (methodNames check is true → emit edge). Cross-file calls get emitted as additional edges (methodNames check is false → now emit edge instead of dropping). The two paths are mutually exclusive.

**Alternatives considered**:
- Emit with unresolved metadata: Rejected — adds complexity for no benefit. The resolver passes through all edges unchanged. Metadata isn't needed.
- Selective filtering: Rejected — too complex. Emit all, let resolver handle.

### Decision 2: Invert test assertion

**Choice**: Invert `TestRubyGraphExtractor_NoCrossFileCalls` to assert cross-file calls DO appear.

**Rationale**: The test currently asserts zero cross-file call edges. After the fix, cross-file calls produce edges. The test must match the new behavior.

## Risks / Trade-offs

- **Edge count increase**: Controller files go from 0 to N call edges. Acceptable — they were silently dropping useful information.
- **Bare method calls**: `render`, `redirect_to`, `params` will have unresolved edges. Acceptable — they capture the call relationship.
