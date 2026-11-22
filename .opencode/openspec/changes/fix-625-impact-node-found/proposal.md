# Distinguish unknown nodes in impact responses (#625)

## Why

Both REST `GraphImpact` and MCP `memory_impact` marshal an uninitialized
impacted slice as `null`. A leaf node and a misspelt node produce the same
response, causing agents to mistake missing index data for no callers.

## What

Always emit `impacted: []` for no results and include `node_found` on both
surfaces, independently of edge filters. Existence is scoped to the selected
workspace and means a matching node exists in either graph endpoints or
indexed documents/symbols. `count` stays a result count, not an existence flag.
