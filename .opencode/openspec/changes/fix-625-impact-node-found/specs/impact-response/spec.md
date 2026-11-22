# Impact Response

## ADDED Requirements

### Requirement: Empty result is an array
Impact responses over REST and MCP SHALL encode zero impacted nodes as
`"impacted":[]` rather than `"impacted":null`.

#### Scenario: Known leaf node
Given a known node without reverse edges,
when impact is requested,
then impacted is an empty array and MCP count is zero.

### Requirement: Node existence is explicit
Impact responses SHALL carry a boolean `node_found` determined within the
selected workspace and independently of the requested edge filter.

#### Scenario: Unknown node
Given a node missing from the selected workspace's graph and index with no
bare-name matches,
when impact is requested,
then node_found is false and impacted is empty.

#### Scenario: Bare edge matches an unrelated qualified node
Given an edge targeting a bare symbol name in another file,
when impact is requested for an absent `missing.go::Symbol` with that suffix,
then node_found is false even when `impacted` contains candidate callers
from the existing bare-name fallback.

#### Scenario: Symbol document outlives its source
Given a symbol document whose parent file document no longer exists,
when impact is requested for that symbol without a surviving graph edge,
then node_found is false.

#### Scenario: Known node with filtered-out edges
Given a node with edges that do not match the requested edge type,
when impact is requested,
then node_found is true and impacted is empty.
