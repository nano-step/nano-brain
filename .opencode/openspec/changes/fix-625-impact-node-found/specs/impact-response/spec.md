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
Given a node missing from the selected workspace's graph and index,
when impact is requested,
then node_found is false and impacted is empty.

#### Scenario: Known node with filtered-out edges
Given a node with edges that do not match the requested edge type,
when impact is requested,
then node_found is true and impacted is empty.
