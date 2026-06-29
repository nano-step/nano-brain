package summarize

import (
	"context"
	"database/sql"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// DBRelationshipLookup implements RelationshipLookup by querying the documents
// table to resolve parent session titles. Child session discovery is deferred:
// the metadata JSON does not yet store parent_id, so Children remains empty on
// this first iteration (pipeline.go:252 handles nil Children gracefully).
type DBRelationshipLookup struct {
	db     *sql.DB
	logger zerolog.Logger
}

// NewDBRelationshipLookup constructs a DBRelationshipLookup.
func NewDBRelationshipLookup(db *sql.DB, logger zerolog.Logger) *DBRelationshipLookup {
	return &DBRelationshipLookup{
		db:     db,
		logger: logger.With().Str("component", "db-relationship-lookup").Logger(),
	}
}

// Lookup enriches meta with parent session title if a ParentID is set.
// All lookups are best-effort: failures are logged at Warn level and do not
// propagate to the caller (matching the pipeline.go:97-100 pattern).
func (l *DBRelationshipLookup) Lookup(ctx context.Context, meta *SessionMetadata) error {
	if meta.ParentID == "" {
		return nil
	}

	q := sqlc.New(l.db)

	// Resolve parent title from the summary document for this session's source.
	parentSourcePath := "summary://" + string(meta.Source) + "/" + meta.ParentID
	parentDoc, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    parentSourcePath,
		WorkspaceHash: meta.WorkspaceHash,
	})
	if err != nil {
		if err != sql.ErrNoRows {
			l.logger.Warn().
				Err(err).
				Str("parent_source_path", parentSourcePath).
				Msg("relationship: parent doc lookup failed")
		}
		// Non-fatal: ParentTitle stays empty; formatHeader falls back to ParentID.
		return nil
	}

	// Strip the "Summary: " prefix that Persister.Save prepends to titles.
	title := parentDoc.Title
	const prefix = "Summary: "
	if len(title) > len(prefix) && title[:len(prefix)] == prefix {
		title = title[len(prefix):]
	}
	meta.ParentTitle = title

	// Children: deferred — metadata JSON does not yet carry parent_id, so we
	// cannot reliably find children without a full-table scan. Leave empty.
	// Future iteration: store parent_id in metadata JSON and add a targeted query.

	return nil
}
