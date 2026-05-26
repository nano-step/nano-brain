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
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
	_ "modernc.org/sqlite"
)

type OpenCodeSQLiteHarvester struct {
	pgDB      *sql.DB
	sqdb      *sql.DB
	dbPath    string
	workspace string
	logger    zerolog.Logger
}

func NewOpenCodeSQLiteHarvester(pgDB *sql.DB, logger zerolog.Logger, dbPath, workspace string) *OpenCodeSQLiteHarvester {
	return &OpenCodeSQLiteHarvester{
		pgDB:      pgDB,
		dbPath:    dbPath,
		workspace: workspace,
		logger:    logger.With().Str("component", "opencode-sqlite-harvester").Logger(),
	}
}

func NewOpenCodeSQLiteHarvesterFromDB(sqdb *sql.DB, pgDB *sql.DB, workspace string) *OpenCodeSQLiteHarvester {
	return &OpenCodeSQLiteHarvester{
		pgDB:      pgDB,
		sqdb:      sqdb,
		workspace: workspace,
		logger:    zerolog.Nop(),
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

func (h *OpenCodeSQLiteHarvester) ListSessions(ctx context.Context) ([]sqSession, error) {
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
	return renderSQLiteMarkdown(sqSession{id: sessionID, title: title, createdAt: createdAt}, msgs), nil
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
	for _, sess := range sessions {
		sourcePath := "opencode://session/" + sess.id
		existing, lookupErr := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
			SourcePath:    sourcePath,
			WorkspaceHash: h.workspace,
		})
		if lookupErr == nil && existing.ContentHash != "" {
			skipped++
			continue
		}

		msgs, err := h.listMessages(ctx, sqdb, sess.id)
		if err != nil {
			h.logger.Warn().Err(err).Str("session", sess.id).Msg("failed to list messages")
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

		title := sess.title
		if title == "" {
			title = "OpenCode session " + sess.id[:8]
		}

		meta, _ := marshalJSON(map[string]any{
			"source":        "opencode",
			"session_id":    sess.id,
			"message_count": len(msgs),
			"created_at":    sess.createdAt.Format(time.RFC3339),
		})

		chunks := chunk.Split(md, chunk.DefaultConfig())
		params := sqlc.UpsertDocumentBySourcePathParams{
			WorkspaceHash: h.workspace,
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
			h.logger.Warn().Err(err).Str("session", sess.id).Msg("begin tx failed")
			errCount++
			continue
		}

		tq := sqlc.New(tx)
		docRow, err := tq.UpsertDocumentBySourcePath(ctx, params)
		if err != nil {
			tx.Rollback() //nolint:errcheck
			h.logger.Warn().Err(err).Str("session", sess.id).Msg("upsert document failed")
			errCount++
			continue
		}

		var chunkIDs []uuid.UUID
		for i, c := range chunks {
			chunkHash := sha256.Sum256([]byte(c.Content))
			chunkID, err := tq.UpsertChunk(ctx, sqlc.UpsertChunkParams{
				DocumentID:    docRow.ID,
				WorkspaceHash: h.workspace,
				ContentHash:   hex.EncodeToString(chunkHash[:]),
				Content:       c.Content,
				ChunkIndex:    int32(i),
				StartLine:     sql.NullInt32{Int32: int32(c.StartLine), Valid: true},
				EndLine:       sql.NullInt32{Int32: int32(c.EndLine), Valid: true},
				Metadata:      pqtype.NullRawMessage{},
			})
			if err != nil {
				h.logger.Warn().Err(err).Str("session", sess.id).Int("chunk", i).Msg("chunk upsert failed")
				continue
			}
			chunkIDs = append(chunkIDs, chunkID)
		}

		if err := tx.Commit(); err != nil {
			tx.Rollback() //nolint:errcheck
			h.logger.Warn().Err(err).Str("session", sess.id).Msg("commit failed")
			errCount++
			continue
		}

		if enqueuer != nil {
			for _, id := range chunkIDs {
				enqueuer.Enqueue(id)
			}
		}

		h.logger.Info().Str("session", sess.id).Int("chunks", len(chunks)).Msg("harvested opencode session")
		harvested++
	}

	h.logger.Info().Int("harvested", harvested).Int("skipped", skipped).Int("errors", errCount).Msg("opencode sqlite harvest complete")
	return
}

type sqSession struct {
	id        string
	title     string
	createdAt time.Time
}

type sqMessage struct {
	role      string
	content   string
	createdAt time.Time
}

func (h *OpenCodeSQLiteHarvester) listSessions(ctx context.Context, sqdb *sql.DB) ([]sqSession, error) {
	rows, err := sqdb.QueryContext(ctx, `
		SELECT id, COALESCE(title, ''), COALESCE(time_created, 0)
		FROM session
		ORDER BY time_created DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []sqSession
	for rows.Next() {
		var s sqSession
		var createdMs int64
		if err := rows.Scan(&s.id, &s.title, &createdMs); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		if createdMs > 0 {
			s.createdAt = time.UnixMilli(createdMs).UTC()
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

func renderSQLiteMarkdown(sess sqSession, msgs []sqMessage) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "session_id: %s\n", sess.id)
	b.WriteString("source: opencode\n")
	fmt.Fprintf(&b, "message_count: %d\n", len(msgs))
	fmt.Fprintf(&b, "created_at: %s\n", sess.createdAt.Format(time.RFC3339))
	if sess.title != "" {
		fmt.Fprintf(&b, "title: %q\n", sess.title)
	}
	b.WriteString("---\n")

	for _, msg := range msgs {
		ts := msg.createdAt.UTC().Format(time.RFC3339)
		fmt.Fprintf(&b, "\n## %s (%s)\n\n", msg.role, ts)
		b.WriteString(msg.content)
		b.WriteString("\n")
	}
	return b.String()
}

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}
