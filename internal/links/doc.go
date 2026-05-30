// Package links parses [[wikilinks]] in document content, resolves them to
// document IDs via workspace-scoped queries, and upserts 'references' edges
// in the knowledge graph.
//
// Import policy: production files in this package MUST NOT import any
// internal/ package. Tests may import internal/testutil and internal/storage/sqlc.
package links
