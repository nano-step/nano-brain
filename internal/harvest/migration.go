package harvest

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
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

const backfillBatchSize = 500

// BackfillSessionTicketTags scans existing session documents that have no
// ticket tags and applies ticket extraction to their stored content.
//
// Pre-phase-8 harvests stored session docs without ticket tags because the
// TicketExtractor did not yet exist. The harvester dedup-skips docs whose
// content hash has not changed, so they are never re-processed. This backfill
// runs once at startup to reconcile those historical docs.
//
// Idempotent: docs that already carry any "ticket:*" tag are excluded from the
// SELECT so a second run is a no-op (tagged=0, err=nil). Processing is batched
// (backfillBatchSize rows per round-trip) to avoid loading the full collection
// into memory. Branch is derived from the content front-matter on a
// best-effort basis; parentTags are not inherited (backfill is cross-session).
//
// Non-fatal in all callers: callers should log a warning and continue on error.
func BackfillSessionTicketTags(ctx context.Context, db *sql.DB, extractor *TicketExtractor, logger zerolog.Logger) (int, error) {
	var tagged int
	offset := 0
	for {
		rows, err := db.QueryContext(ctx,
			`SELECT id, content, tags FROM documents
			 WHERE collection = 'sessions'
			 ORDER BY id
			 LIMIT $1 OFFSET $2`,
			backfillBatchSize, offset,
		)
		if err != nil {
			return tagged, fmt.Errorf("backfill session ticket tags: query batch (offset=%d): %w", offset, err)
		}

		type docRow struct {
			id      string
			content string
			tags    []string
		}
		var batch []docRow
		pageSize := 0
		for rows.Next() {
			pageSize++
			var r docRow
			if err := rows.Scan(&r.id, &r.content, pq.Array(&r.tags)); err != nil {
				rows.Close()
				return tagged, fmt.Errorf("backfill session ticket tags: scan row: %w", err)
			}
			// Filter in Go: skip docs that already have any ticket: tag.
			hasTicket := false
			for _, t := range r.tags {
				if strings.HasPrefix(t, "ticket:") {
					hasTicket = true
					break
				}
			}
			if !hasTicket {
				batch = append(batch, r)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return tagged, fmt.Errorf("backfill session ticket tags: iterate batch (offset=%d): %w", offset, err)
		}

		for _, r := range batch {
			branch := extractBranchFromContent(r.content)
			tickets := extractor.Extract(r.content, branch, nil)
			if len(tickets) == 0 {
				continue
			}
			newTags := appendTicketTags(r.tags, extractor.AsTags(tickets))
			_, err := db.ExecContext(ctx,
				`UPDATE documents SET tags = $1 WHERE id = $2`,
				pq.Array(newTags), r.id,
			)
			if err != nil {
				return tagged, fmt.Errorf("backfill session ticket tags: update doc %s: %w", r.id, err)
			}
			tagged++
		}

		if pageSize < backfillBatchSize {
			// Raw page was smaller than the batch size — we have reached the end.
			break
		}
		offset += backfillBatchSize
	}

	logger.Info().Int("tagged", tagged).Msg("backfill session ticket tags complete")
	return tagged, nil
}

// extractBranchFromContent attempts to parse a "Branch: <value>" line from the
// rendered front-matter at the top of a session document. Returns "" if not found.
func extractBranchFromContent(content string) string {
	for _, line := range strings.SplitN(content, "\n", 20) {
		line = strings.TrimSpace(line)
		if val, ok := strings.CutPrefix(line, "Branch:"); ok {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

// appendTicketTags merges newTags into existing, deduplicating by value.
// Existing tags are preserved in order; new tags are appended at the end.
func appendTicketTags(existing, newTags []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(newTags))
	result := make([]string, 0, len(existing)+len(newTags))
	for _, t := range existing {
		if _, dup := seen[t]; !dup {
			seen[t] = struct{}{}
			result = append(result, t)
		}
	}
	for _, t := range newTags {
		if _, dup := seen[t]; !dup {
			seen[t] = struct{}{}
			result = append(result, t)
		}
	}
	return result
}
