## Context

The CFG builder uses a recursive-descent pattern: `buildBlock` → `buildIf`/`buildReturn`/etc. with predecessor-set propagation. Each `build*` function returns a set of exit nodes that become predecessors for the next statement.

`relabelPreds` strips the decision node from exit sets to prevent self-loop edges. This is correct when a sub-block was actually built (the exit nodes are real step/decision nodes inside the block). But it's incorrect for the no-else case where `{decisionID: true}` IS the correct continuation point.

## Goals / Non-Goals

**Goals:**
- `buildIf` preserves continuation paths for no-else `if` statements
- Guard clauses with early returns no longer disconnect subsequent code

**Non-Goals:**
- Fixing `wrapInBlock` no-op (separate issue — bare returns not recognized as terminals)
- Fixing `thenExits` stripping for empty-then blocks (rare pattern, deferred)

## Decisions

### D1: Move `relabelPreds` inside `if alternative != nil`

The `buildTry` function already uses this pattern for the no-catch case (line 596-598). When there's no catch, `catchExits = {tryID: true}` is NOT relabeled. `buildIf` should do the same for no-else.

**Change:** Move `elseExits = b.relabelPreds(elseExits, decisionID)` inside the `if alternative != nil` block. The no-else case keeps `{decisionID: true}` as-is.

### D2: Branch label "next" for fall-through is correct

The edge from `decisionID` to the next step gets branch `"next"` from the outer context. The existing test `TestJSControlFlowExtractor_IfOnlyBranchLabels` explicitly expects no `"no"` edge when there's no else — only the `"next"` fall-through.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Existing tests might break | Both Metis and Oracle verified tests pass with the fix |
| Edge label accuracy | "next" is semantically correct for fall-through; "no" would be more precise but unnecessary |
