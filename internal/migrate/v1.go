// Package migrate provides tooling for migrating data from v1 (Node.js/SQLite)
// nano-brain databases to the v2 PostgreSQL schema.
package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Writer abstracts the v2 PostgreSQL document upsert.
type Writer interface {
	UpsertDocument(ctx context.Context, params UpsertParams) error
}

// UpsertParams holds the fields needed to upsert a document into v2.
type UpsertParams struct {
	WorkspaceHash string
	ContentHash   string
	Title         string
	Content       string
	SourcePath    string
	Collection    string
	Tags          []string
}

// MigrateResult summarises a migration run.
type MigrateResult struct {
	Total    int
	Migrated int
	Skipped  int
	Errors   []string
}

// ProgressFunc is called after each document is processed.
type ProgressFunc func(current, total int)

// V1Migrator reads documents from a v1 SQLite file and writes them to v2.
type V1Migrator struct {
	srcDB  *sql.DB
	writer Writer
}

// NewV1Migrator opens the v1 SQLite database at sqlitePath.
func NewV1Migrator(sqlitePath string, writer Writer) (*V1Migrator, error) {
	db, err := sql.Open("sqlite", sqlitePath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open v1 sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping v1 sqlite: %w", err)
	}
	return &V1Migrator{srcDB: db, writer: writer}, nil
}

// Close releases the v1 SQLite connection.
func (m *V1Migrator) Close() error {
	return m.srcDB.Close()
}

// Migrate reads all documents from the v1 SQLite database and upserts them
// into the v2 store via the Writer interface.
func (m *V1Migrator) Migrate(ctx context.Context, workspaceHash string, progress ProgressFunc) (*MigrateResult, error) {
	if !m.hasTable("documents") {
		return nil, fmt.Errorf("v1 database has no 'documents' table")
	}

	rows, err := m.srcDB.QueryContext(ctx, `SELECT source_path, title, content, tags, collection FROM documents`)
	if err != nil {
		return nil, fmt.Errorf("query v1 documents: %w", err)
	}
	defer rows.Close()

	type v1Row struct {
		sourcePath string
		title      string
		content    string
		rawTags    sql.NullString
		collection sql.NullString
	}

	var docs []v1Row
	for rows.Next() {
		var r v1Row
		if err := rows.Scan(&r.sourcePath, &r.title, &r.content, &r.rawTags, &r.collection); err != nil {
			return nil, fmt.Errorf("scan v1 row: %w", err)
		}
		docs = append(docs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate v1 rows: %w", err)
	}

	res := &MigrateResult{Total: len(docs)}

	for i, doc := range docs {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}

		if doc.content == "" {
			res.Skipped++
			if progress != nil {
				progress(i+1, res.Total)
			}
			continue
		}

		hash := contentHash(doc.content)
		tags := parseTags(doc.rawTags.String)
		col := doc.collection.String
		if col == "" {
			col = "v1-import"
		}

		err := m.writer.UpsertDocument(ctx, UpsertParams{
			WorkspaceHash: workspaceHash,
			ContentHash:   hash,
			Title:         doc.title,
			Content:       doc.content,
			SourcePath:    doc.sourcePath,
			Collection:    col,
			Tags:          tags,
		})
		if err != nil {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("source_path=%s: %v", doc.sourcePath, err))
		} else {
			res.Migrated++
		}

		if progress != nil {
			progress(i+1, res.Total)
		}
	}

	return res, nil
}

func (m *V1Migrator) hasTable(name string) bool {
	var n int
	err := m.srcDB.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name,
	).Scan(&n)
	return err == nil && n > 0
}

func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func parseTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err == nil && len(arr) > 0 {
		return arr
	}

	parts := strings.Split(raw, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
