# Design

Add `ImpactNodeExists` to `internal/storage/queries/graph.sql` (regenerate
sqlc). Lookup checks graph edge endpoints, a file document's source path,
or a symbol document for the requested `file::Symbol`. Match relative and
workspace-root-joined paths where applicable. Do not apply `edge_type` to
existence: a known node with no edges of that type remains `node_found: true`.
Existence is checked before traversal. A file-qualified symbol expands to
its bare name for impact recall; a bare-name hit in another file does not
prove that the requested qualified node exists. A symbol document only
proves existence while its parent file document remains indexed; otherwise
the stale symbol must be treated as absent.
Preserve the existing bare-target traversal even when the qualified node
is not found: legacy graph edges may contain only bare target names. Such
impact entries are candidates, not evidence that the exact requested node
exists. `node_found` must reflect only the exact workspace-scoped lookup.

Use one query per impact request (not per traversal level) on the two API
surfaces. Set empty result slices non-nil. Test leaf, misspelt, and filtered
known node on REST and MCP.
Independent reviewers reproduced a missing qualified node reported as found
via a bare calls edge. A follow-up regression captures this on both surfaces;
another covers a stale symbol document after parent deletion. Their
unmeasured performance concern remains for a larger test-data benchmark
(the observed query took 1.48 ms over 1,021 test documents).
Full integration tests exposed an important compatibility constraint: bare
calls edges must still produce candidate impacts for a qualified request,
even without an exact graph endpoint or document. Those tests pass with
`node_found:false` and candidate results preserved.
