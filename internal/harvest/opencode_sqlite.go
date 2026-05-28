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
sessionLoop:
	for _, sess := range sessions {
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

		sourcePath := "opencode://session/" + sess.ID
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
		sum := sha256.Sum256([]byte(md))
		contentHash := hex.EncodeToString(sum[:])

		title := sess.Title
		if title == "" {
			title = "OpenCode session " + sess.ID[:8]
		}

		meta, _ := marshalJSON(map[string]any{
			"source":        "opencode",
			"session_id":    sess.ID,
			"message_count": len(msgs),
			"created_at":    sess.CreatedAt.Format(time.RFC3339),
		})

		chunks := chunk.Split(md, chunk.DefaultConfig())
		params := sqlc.UpsertDocumentBySourcePathParams{
			WorkspaceHash: wsHash,
			ContentHash:   contentHash,
			Title:         title,
			Content:       md,
			SourcePath:    sourcePath,
			Collection:    "sessions",
			Tags:          []string{"opencode", "session"},
			Metadata:      pqtype.NullRawMessage{RawMessage: meta, Valid: true},
		}

		tx, err := h.pgDB.BeginTx(ctx, nil)
		if err != nil {
			h.logger.Warn().Err(err).Str("session", sess.ID).Msg("begin tx failed")
			errCount++
			continue
		}

		tq := sqlc.New(tx)
		docRow, err := tq.UpsertDocumentBySourcePath(ctx, params)
		if err != nil {
			tx.Rollback() //nolint:errcheck
			h.logger.Warn().Err(err).Str("session", sess.ID).Msg("upsert document failed")
			errCount++
			continue
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
				h.logger.Warn().Err(err).Str("session", sess.ID).Int("chunk", i).Msg("chunk upsert failed, rolling back")
				errCount++
				continue sessionLoop
			}
			chunkIDs = append(chunkIDs, chunkID)
		}

		if err := tx.Commit(); err != nil {
			tx.Rollback() //nolint:errcheck
			h.logger.Warn().Err(err).Str("session", sess.ID).Msg("commit failed")
			errCount++
			continue
		}

		if enqueuer != nil {
			for _, id := range chunkIDs {
				enqueuer.Enqueue(id)
			}
		}

		h.logger.Info().Str("session", sess.ID).Int("chunks", len(chunks)).Msg("harvested opencode session")
		harvested++

		if h.summarizer != nil {
			smeta := SummaryMeta{
				Source:    "opencode",
				SessionID: sess.ID,
				Title:     title,
				CreatedAt: sess.CreatedAt,
			}
			if err := h.summarizer.SummarizeAndPersist(ctx, md, smeta); err != nil {
				h.logger.Warn().Err(err).Str("session", sess.ID).Msg("summarization failed, session still harvested")
			}
		}
	}

	h.logger.Info().Int("harvested", harvested).Int("skipped", skipped).Int("errors", errCount).Msg("opencode sqlite harvest complete")
	return
}

type SqSession struct {
	ID        string
	Title     string
	CreatedAt time.Time
	Worktree  string
}

type sqMessage struct {
	role      string
	content   string
	createdAt time.Time
}

func (h *OpenCodeSQLiteHarvester) listSessions(ctx context.Context, sqdb *sql.DB) ([]SqSession, error) {
	rows, err := sqdb.QueryContext(ctx, `
		SELECT s.id, COALESCE(s.title, ''), COALESCE(s.time_created, 0),
		       COALESCE(p.worktree, '')
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
		var createdMs int64
		if err := rows.Scan(&s.ID, &s.Title, &createdMs, &s.Worktree); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		if createdMs > 0 {
			s.CreatedAt = time.UnixMilli(createdMs).UTC()
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

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}
