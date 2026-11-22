# Design

Add `ImpactNodeExists` to `internal/storage/queries/graph.sql` (regenerate
sqlc). Lookup checks graph edge endpoints, a file document's source path,
or a symbol document for the requested `file::Symbol`. Match relative and
workspace-root-joined paths where applicable. Do not apply `edge_type` to
existence: a known node with no edges of that type remains `node_found: true`.
For calls-edge bare targets, a nonempty impact result also proves existence.

Use one query per impact request (not per traversal level) on the two API
surfaces. Set empty result slices non-nil. Test leaf, misspelt, and filtered
known node on REST and MCP.
