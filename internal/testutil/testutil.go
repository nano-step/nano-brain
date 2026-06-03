// Package testutil provides testing utilities.
package testutil

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func NopLogger() zerolog.Logger {
	return zerolog.Nop()
}

// SeedDocumentWithTimestamps inserts a document and chunk into the test database
// with custom created_at, updated_at timestamps, and optional tags. Returns the chunk ID.
// Helper for integration tests that need to verify time-range filtering and cursor invalidation.
func SeedDocumentWithTimestamps(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	wsHash, title, content string,
	tags []string,
	createdAt, updatedAt time.Time,
) uuid.UUID {
	t.Helper()

	docID := uuid.New()
	chunkID := uuid.New()

	// Compute content hash
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(title+content)))

	// Insert document with custom timestamps and tags
	var tagArray interface{}
	if len(tags) > 0 {
		// Convert tags slice to PostgreSQL array string
		tagArray = "{" + fmt.Sprintf("%q", tags[0])
		for _, tag := range tags[1:] {
			tagArray = fmt.Sprintf("%s,%q", tagArray, tag)
		}
		tagArray = tagArray.(string) + "}"
	} else {
		tagArray = "{}"
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO documents (id, workspace_hash, content_hash, title, source_path, collection, tags, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::text[], $8, $9)`,
		docID, wsHash, hash, title, "/tmp/"+title+".md", "memory", tagArray, createdAt, updatedAt)
	if err != nil {
		t.Fatalf("insert doc %q: %v", title, err)
	}

	// Compute chunk hash
	chunkHash := fmt.Sprintf("%x", sha256.Sum256([]byte(content+chunkID.String())))

	// Insert chunk and populate search_vector for BM25 indexing
	_, err = db.ExecContext(ctx,
		`INSERT INTO chunks (id, document_id, workspace_hash, content_hash, content, chunk_index, search_vector)
		 VALUES ($1, $2, $3, $4, $5, 0, to_tsvector('english', $5))`,
		chunkID, docID, wsHash, chunkHash, content)
	if err != nil {
		t.Fatalf("insert chunk for %q: %v", title, err)
	}

	return chunkID
}
