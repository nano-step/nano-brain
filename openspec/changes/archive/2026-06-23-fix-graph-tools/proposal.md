## Why

The code intelligence graph tools (`memory_graph`, `memory_flowchart`, `memory_impact`, `memory_flow`) are not working correctly for Ruby on Rails projects. Associations (has_many, belongs_to) are not extracted as edges, the flowchart tool rejects valid Ruby method formats, impact analysis returns 0 results, and flow diagrams are shallow. This blocks Rails developers from using nano-brain's code intelligence features.

## What Changes

- **Fix association edge extraction**: `has_many`, `belongs_to`, `has_one`, `has_and_belongs_to_many` will be extracted as graph edges with proper target resolution (model class names instead of symbol arguments)
- **Fix callback edge extraction**: `before_action`, `after_action`, etc. will create edges to the method names they reference
- **Fix memory_flowchart for Ruby**: Accept `file.rb::ClassName#method` format in addition to `file::startLine-endLine`
- **Fix memory_impact**: Ensure graph edges exist so impact analysis can traverse them
- **Improve memory_flow**: Trace deeper into controller method bodies, not just entry → handler

## Capabilities

### New Capabilities

- `rails-association-edges`: Extract has_many/belongs_to/has_one as graph edges with model class resolution
- `rails-callback-edges`: Extract before_action/after_action callbacks as edges to referenced methods
- `ruby-flowchart-format`: Support Class#method format for Ruby flowchart lookups

### Modified Capabilities

- `graph-traversal`: Ensure graph edges exist for association and callback relationships

## Impact

- **Affected code**: `internal/graph/rails_dsl_extractor.go`, `internal/mcp/flowchart.go`, `internal/mcp/tools.go`
- **API changes**: `memory_graph`, `memory_flowchart`, `memory_impact`, `memory_flow` tools will return correct results
- **Dependencies**: No new dependencies
- **Systems**: All Rails workspaces need re-indexing to extract association edges
