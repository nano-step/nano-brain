//go:build integration

package mcp_test

import (
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// Issue #544 N3: memory_write's only cleanup mechanism was supersede, which
// leaves a permanent tombstone document. memory_delete permanently removes a
// document (and its chunks, via ON DELETE CASCADE) so a note written in
// error doesn't accrete forever.

func TestMemoryDelete_ByUUID_RemovesDocumentAndChunks(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	doc, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: wsHash,
		ContentHash:   "delete-hash-1",
		Title:         "throwaway note",
		Content:       "oops, wrong note",
		SourcePath:    "notes/throwaway.md",
		Collection:    "memory",
	})
	if err != nil {
		t.Fatalf("upsert document: %v", err)
	}
	if _, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
		DocumentID:        doc.ID,
		WorkspaceHash:     wsHash,
		ContentHash:       "delete-chunk-1",
		Content:           "oops, wrong note",
		ChunkIndex:        0,
		ChunkType:         "text",
		EmbeddingStrategy: "none",
	}); err != nil {
		t.Fatalf("upsert chunk: %v", err)
	}

	result := callTool("memory_delete", map[string]any{"workspace": wsHash, "path": doc.ID.String()})
	if result.IsError {
		t.Fatalf("memory_delete errored: %v", result.Content[0].(*mcpsdk.TextContent).Text)
	}
	resp := unmarshalGraphResp(t, result)
	if deleted, _ := resp["deleted"].(bool); !deleted {
		t.Fatalf("expected deleted=true, got %+v", resp)
	}
	if resp["id"].(string) != doc.ID.String() {
		t.Errorf("id = %v, want %v", resp["id"], doc.ID)
	}

	if _, err := q.GetDocumentByID(ctx, sqlc.GetDocumentByIDParams{ID: doc.ID, WorkspaceHash: wsHash}); err == nil {
		t.Fatal("document still exists after memory_delete")
	}

	// memory_get on the same id must now report a clean not-found, not error noise.
	getResult := callTool("memory_get", map[string]any{"workspace": wsHash, "path": doc.ID.String()})
	if !getResult.IsError {
		t.Fatal("expected memory_get to error for a deleted document")
	}
}

func TestMemoryDelete_ByHashPrefix_ResolvesChunkToParent(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)

	doc, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: wsHash,
		ContentHash:   "delete-hash-2",
		Title:         "note with chunks",
		Content:       "full content",
		SourcePath:    "notes/withchunks.md",
		Collection:    "memory",
	})
	if err != nil {
		t.Fatalf("upsert document: %v", err)
	}
	chunkID, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
		DocumentID:        doc.ID,
		WorkspaceHash:     wsHash,
		ContentHash:       "delete-chunk-2",
		Content:           "full content",
		ChunkIndex:        0,
		ChunkType:         "text",
		EmbeddingStrategy: "none",
	})
	if err != nil {
		t.Fatalf("upsert chunk: %v", err)
	}

	// "#<chunk-id>" — a search result's chunk id must resolve to the parent
	// document, same as memory_get.
	result := callTool("memory_delete", map[string]any{"workspace": wsHash, "path": "#" + chunkID.String()})
	if result.IsError {
		t.Fatalf("memory_delete errored: %v", result.Content[0].(*mcpsdk.TextContent).Text)
	}
	resp := unmarshalGraphResp(t, result)
	if resp["id"].(string) != doc.ID.String() {
		t.Errorf("id = %v, want parent document id %v", resp["id"], doc.ID)
	}

	if _, err := q.GetDocumentByID(ctx, sqlc.GetDocumentByIDParams{ID: doc.ID, WorkspaceHash: wsHash}); err == nil {
		t.Fatal("document still exists after memory_delete via chunk id")
	}
}

func TestMemoryDelete_UnknownPath_ReturnsCleanError(t *testing.T) {
	_, _, wsHash, callTool := setupFindingsMCP(t)

	result := callTool("memory_delete", map[string]any{"workspace": wsHash, "path": "notes/does-not-exist.md"})
	if !result.IsError {
		t.Fatal("expected error result for unknown path")
	}
}

func TestMemoryDelete_WrongWorkspace_DoesNotCrossDelete(t *testing.T) {
	ctx, q, wsHash, callTool := setupFindingsMCP(t)
	_, _, otherWsHash, _ := setupFindingsMCP(t)

	doc, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: wsHash,
		ContentHash:   "delete-hash-3",
		Title:         "scoped note",
		Content:       "content",
		SourcePath:    "notes/scoped.md",
		Collection:    "memory",
	})
	if err != nil {
		t.Fatalf("upsert document: %v", err)
	}

	result := callTool("memory_delete", map[string]any{"workspace": otherWsHash, "path": doc.ID.String()})
	if !result.IsError {
		t.Fatal("expected error: document belongs to a different workspace")
	}

	if _, err := q.GetDocumentByID(ctx, sqlc.GetDocumentByIDParams{ID: doc.ID, WorkspaceHash: wsHash}); err != nil {
		t.Fatalf("document should still exist in its own workspace: %v", err)
	}
}
