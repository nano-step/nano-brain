package harvest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
	_ "modernc.org/sqlite"
)

type OpenCodeSQLiteHarvester struct {
	pgDB       *sql.DB
	sqdb       *sql.DB
	dbPath     string
	logger     zerolog.Logger
	summarizer SessionSummarizer
}

func (h *OpenCodeSQLiteHarvester) setSummarizer(s SessionSummarizer) { h.summarizer = s }

func (h *OpenCodeSQLiteHarvester) WithSummarizer(s SessionSummarizer) *OpenCodeSQLiteHarvester {
	h.summarizer = s
	return h
}

func NewOpenCodeSQLiteHarvester(pgDB *sql.DB, logger zerolog.Logger, dbPath string) *OpenCodeSQLiteHarvester {
	return &OpenCodeSQLiteHarvester{
		pgDB:   pgDB,
		dbPath: dbPath,
		logger: logger.With().Str("component", "opencode-sqlite-harvester").Logger(),
	}
}

func NewOpenCodeSQLiteHarvesterFromDB(sqdb *sql.DB, pgDB *sql.DB) *OpenCodeSQLiteHarvester {
	return &OpenCodeSQLiteHarvester{
		pgDB:   pgDB,
		sqdb:   sqdb,
		logger: zerolog.Nop(),
	}
}

func (h *OpenCodeSQLiteHarvester) openSQLite(ctx context.Context) (*sql.DB, bool, error) {
	if h.sqdb != nil {
		return h.sqdb, false, nil
	}
	db, err := sql.Open("sqlite", h.dbPath+"?mode=ro")
	if err != nil {
		return nil, false, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, false, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, true, nil
}

func (h *OpenCodeSQLiteHarvester) ListSessions(ctx context.Context) ([]SqSession, error) {
	sqdb, owned, err := h.openSQLite(ctx)
	if err != nil {
		return nil, err
	}
	if owned {
		defer sqdb.Close()
	}
	return h.listSessions(ctx, sqdb)
}

func (h *OpenCodeSQLiteHarvester) RenderSession(ctx context.Context, sessionID, title string, createdAt time.Time) (string, error) {
	sqdb, owned, err := h.openSQLite(ctx)
	if err != nil {
		return "", err
	}
	if owned {
		defer sqdb.Close()
	}
	msgs, err := h.listMessages(ctx, sqdb, sessionID)
	if err != nil {
		return "", err
	}
	return renderSQLiteMarkdown(SqSession{ID: sessionID, Title: title, CreatedAt: createdAt}, msgs), nil
}

func (h *OpenCodeSQLiteHarvester) HarvestAll(ctx context.Context, enqueuer ChunkEnqueuer) (harvested, skipped, errCount int) {
	h.logger.Info().Str("db", h.dbPath).Msg("opening opencode sqlite db")

	sqdb, owned, err := h.openSQLite(ctx)
	if err != nil {
		h.logger.Error().Err(err).Str("db", h.dbPath).Msg("failed to open opencode sqlite db")
		return 0, 0, 1
	}
	if owned {
		defer sqdb.Close()
	}

	sessions, err := h.listSessions(ctx, sqdb)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list opencode sessions")
		return 0, 0, 1
	}

	h.logger.Info().Int("count", len(sessions)).Msg("found opencode sessions")

	q := sqlc.New(h.pgDB)
	wsCache := make(map[string]string) // worktree → wsHash

	var (
		summarySuccess  int
		summaryFallback int
		activeCount     int
	)

	for _, sess := range sessions {
		if isActiveSession(sess) {
			activeCount++
			continue
		}

		// Derive workspace hash for this session's project
		worktree := sess.Worktree
		wsHash, ok := wsCache[worktree]
		if !ok {
			var hashErr error
			if worktree == "" {
				h.logger.Warn().Str("session_id", sess.ID).Msg("session has no project row, using fallback workspace")
				wsHash, hashErr = storage.WorkspaceHash(h.dbPath)
			} else {
				wsHash, hashErr = storage.WorkspaceHash(worktree)
			}
			if hashErr != nil {
				h.logger.Warn().Err(hashErr).Str("session", sess.ID).Msg("workspace hash failed, skipping")
				errCount++
				continue
			}
			if worktree != "" {
				rq := sqlc.New(h.pgDB)
				if _, uErr := rq.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
					Hash: wsHash,
					Path: worktree,
				}); uErr != nil {
					h.logger.Warn().Err(uErr).Str("worktree", worktree).Msg("upsert workspace failed")
				}
			}
			wsCache[worktree] = wsHash
		}

		sourcePath := "summary://opencode/" + sess.ID
		existing, lookupErr := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
			SourcePath:    sourcePath,
			WorkspaceHash: wsHash,
		})
		if lookupErr == nil && existing.ContentHash != "" {
			skipped++
			continue
		}

		msgs, err := h.listMessages(ctx, sqdb, sess.ID)
		if err != nil {
			h.logger.Warn().Err(err).Str("session", sess.ID).Msg("failed to list messages")
			errCount++
			continue
		}
		if len(msgs) == 0 {
			skipped++
			continue
		}

		md := renderSQLiteMarkdown(sess, msgs)

		title := sess.Title
		if title == "" {
			title = "OpenCode session " + sess.ID[:8]
		}

		if h.summarizer != nil {
			smeta := SummaryMeta{
				Source:        "opencode",
				SessionID:     sess.ID,
				Title:         title,
				CreatedAt:     sess.CreatedAt,
				WorkspaceHash: wsHash,
			}
			if sumErr := h.summarizer.SummarizeAndPersist(ctx, md, smeta); sumErr != nil {
				h.logger.Warn().Err(sumErr).Str("session", sess.ID).Msg("summarization failed, falling back to raw")
				if fbErr := h.writeRawFallback(ctx, sess, md, wsHash, title, sourcePath, len(msgs), enqueuer); fbErr != nil {
					h.logger.Error().Err(fbErr).Str("session", sess.ID).Msg("raw fallback failed, skipping session")
					errCount++
					continue
				}
				summaryFallback++
			} else {
				summarySuccess++
			}
		} else {
			if fbErr := h.writeRawFallback(ctx, sess, md, wsHash, title, sourcePath, len(msgs), enqueuer); fbErr != nil {
				h.logger.Error().Err(fbErr).Str("session", sess.ID).Msg("raw fallback failed, skipping session")
				errCount++
				continue
			}
			summaryFallback++
		}
	}

	harvested = summarySuccess + summaryFallback
	h.logger.Info().
		Str("source", "opencode").
		Int("summary_success", summarySuccess).
		Int("summary_fallback", summaryFallback).
		Int("skipped", skipped).
		Int("active", activeCount).
		Int("errors", errCount).
		Msg("harvest cycle complete")
	return
}

type SqSession struct {
	ID        string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
	Worktree  string
}

func isActiveSession(sess SqSession) bool {
	return !sess.UpdatedAt.IsZero() && time.Since(sess.UpdatedAt) < 10*time.Minute
}

type sqMessage struct {
	role      string
	content   string
	createdAt time.Time
}

func (h *OpenCodeSQLiteHarvester) listSessions(ctx context.Context, sqdb *sql.DB) ([]SqSession, error) {
	rows, err := sqdb.QueryContext(ctx, `
		SELECT s.id, COALESCE(s.title, ''), COALESCE(s.time_created, 0),
		       COALESCE(s.time_updated, s.time_created, 0), COALESCE(p.worktree, '')
		FROM session s
		LEFT JOIN project p ON s.project_id = p.id
		ORDER BY s.time_created DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SqSession
	for rows.Next() {
		var s SqSession
		var createdMs, updatedMs int64
		if err := rows.Scan(&s.ID, &s.Title, &createdMs, &updatedMs, &s.Worktree); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		if createdMs > 0 {
			s.CreatedAt = time.UnixMilli(createdMs).UTC()
		}
		if updatedMs > 0 {
			s.UpdatedAt = time.UnixMilli(updatedMs).UTC()
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

type msgDataJSON struct {
	Role string `json:"role"`
}

type partDataJSON struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (h *OpenCodeSQLiteHarvester) listMessages(ctx context.Context, sqdb *sql.DB, sessionID string) ([]sqMessage, error) {
	rows, err := sqdb.QueryContext(ctx, `
		SELECT m.id, m.time_created, m.data, p.data
		FROM message m
		LEFT JOIN part p ON p.message_id = m.id
		WHERE m.session_id = ?
		ORDER BY m.time_created ASC, p.rowid ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	type msgAccum struct {
		role      string
		createdMs int64
		texts     []string
	}
	order := []string{}
	accum := map[string]*msgAccum{}

	for rows.Next() {
		var msgID string
		var createdMs int64
		var msgDataRaw string
		var partDataRaw sql.NullString
		if err := rows.Scan(&msgID, &createdMs, &msgDataRaw, &partDataRaw); err != nil {
			return nil, fmt.Errorf("scan message row: %w", err)
		}

		if _, seen := accum[msgID]; !seen {
			var md msgDataJSON
			_ = json.Unmarshal([]byte(msgDataRaw), &md)
			role := md.Role
			if role == "" {
				role = "unknown"
			}
			accum[msgID] = &msgAccum{role: role, createdMs: createdMs}
			order = append(order, msgID)
		}

		if partDataRaw.Valid {
			var pd partDataJSON
			if err := json.Unmarshal([]byte(partDataRaw.String), &pd); err == nil {
				if pd.Type == "text" && strings.TrimSpace(pd.Text) != "" {
					accum[msgID].texts = append(accum[msgID].texts, pd.Text)
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var msgs []sqMessage
	for _, id := range order {
		a := accum[id]
		content := strings.Join(a.texts, "")
		if strings.TrimSpace(content) == "" {
			continue
		}
		msgs = append(msgs, sqMessage{
			role:      a.role,
			content:   content,
			createdAt: time.UnixMilli(a.createdMs).UTC(),
		})
	}
	return msgs, nil
}

// sanitizeText removes characters that PostgreSQL UTF-8 encoding rejects,
// specifically null bytes (0x00) which cause "invalid byte sequence" errors.
func sanitizeText(s string) string {
	return strings.ReplaceAll(s, "\x00", "")
}

func renderSQLiteMarkdown(sess SqSession, msgs []sqMessage) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "session_id: %s\n", sess.ID)
	b.WriteString("source: opencode\n")
	fmt.Fprintf(&b, "message_count: %d\n", len(msgs))
	fmt.Fprintf(&b, "created_at: %s\n", sess.CreatedAt.Format(time.RFC3339))
	if sess.Title != "" {
		fmt.Fprintf(&b, "title: %q\n", sanitizeText(sess.Title))
	}
	b.WriteString("---\n")

	for _, msg := range msgs {
		ts := msg.createdAt.UTC().Format(time.RFC3339)
		fmt.Fprintf(&b, "\n## %s (%s)\n\n", msg.role, ts)
		b.WriteString(sanitizeText(msg.content))
		b.WriteString("\n")
	}
	return b.String()
}

// writeRawFallback persists raw rendered markdown at the unified summary:// source_path
// with collection="sessions" and metadata.fallback=true. This is used when the summarizer
// is nil or returns an error.
//
// Because UpsertDocumentBySourcePath uses (source_path, workspace_hash) as the upsert key,
// calling this after a failed SummarizeAndPersist will OVERWRITE any partial summary
// that Persister.Save may have committed before the error. This is correct behavior —
// the fallback doc replaces any partial/corrupt summary at the same path.
func (h *OpenCodeSQLiteHarvester) writeRawFallback(
	ctx context.Context,
	sess SqSession,
	md string,
	wsHash string,
	title string,
	sourcePath string,
	messageCount int,
	enqueuer ChunkEnqueuer,
) error {
	sum := sha256.Sum256([]byte(md))
	contentHash := hex.EncodeToString(sum[:])

	meta, _ := marshalJSON(map[string]any{
		"source":        "opencode",
		"session_id":    sess.ID,
		"message_count": messageCount,
		"created_at":    sess.CreatedAt.Format(time.RFC3339),
		"fallback":      true,
	})

	chunks := chunk.Split(md, chunk.DefaultConfig())
	params := sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: wsHash,
		ContentHash:   contentHash,
		Title:         title,
		Content:       md,
		SourcePath:    sourcePath,
		Collection:    "sessions",
		Tags:          []string{"opencode", "session", "fallback"},
		Metadata:      pqtype.NullRawMessage{RawMessage: meta, Valid: true},
	}

	tx, err := h.pgDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	tq := sqlc.New(tx)
	docRow, err := tq.UpsertDocumentBySourcePath(ctx, params)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("upsert document: %w", err)
	}

	if err := tq.DeleteChunksByDocumentID(ctx, sqlc.DeleteChunksByDocumentIDParams{
		DocumentID:    docRow.ID,
		WorkspaceHash: wsHash,
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete old chunks: %w", err)
	}

	var chunkIDs []uuid.UUID
	for i, c := range chunks {
		chunkHash := sha256.Sum256([]byte(c.Content))
		chunkID, err := tq.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:    docRow.ID,
			WorkspaceHash: wsHash,
			ContentHash:   hex.EncodeToString(chunkHash[:]),
			Content:       c.Content,
			ChunkIndex:    int32(i),
			StartLine:     sql.NullInt32{Int32: int32(c.StartLine), Valid: true},
			EndLine:       sql.NullInt32{Int32: int32(c.EndLine), Valid: true},
			Metadata:      pqtype.NullRawMessage{},
		})
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("chunk upsert %d: %w", i, err)
		}
		chunkIDs = append(chunkIDs, chunkID)
	}

	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("commit: %w", err)
	}

	if enqueuer != nil {
		for _, id := range chunkIDs {
			enqueuer.Enqueue(id)
		}
	}

	h.logger.Info().Str("session", sess.ID).Bool("fallback", true).Int("chunks", len(chunkIDs)).Msg("raw fallback persisted")
	return nil
}

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}
