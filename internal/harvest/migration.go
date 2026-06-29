package harvest

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rs/zerolog"
)

// MigrateSessionSummaryToSessions renames all documents in the legacy
// "session-summary" collection to the canonical "sessions" collection.
//
// This is idempotent: running it a second time returns (0, nil) because
// no rows match collection = 'session-summary' after the first run.
// Chunks do not need migration — they are keyed by document_id, not collection.
//
// The migration runs synchronously and must complete before the harvest Runner
// starts its first cycle so that new writes (to "sessions") do not race with
// stale reads (from "session-summary").
func MigrateSessionSummaryToSessions(ctx context.Context, db *sql.DB, logger zerolog.Logger) (int64, error) {
	res, err := db.ExecContext(ctx,
		`UPDATE documents SET collection = 'sessions' WHERE collection = 'session-summary'`,
	)
	if err != nil {
		return 0, fmt.Errorf("migrate session-summary→sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		logger.Info().Int64("migrated", n).Msg("migrated session-summary docs to sessions collection")
	}
	return n, nil
}
