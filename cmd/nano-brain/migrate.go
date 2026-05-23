package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/migrate"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/sqlc-dev/pqtype"
)

func runDBMigrateCmd(args []string) {
	var fromV1 string
	var workspace string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from-v1":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--from-v1 requires a path\n")
				os.Exit(1)
			}
			i++
			fromV1 = args[i]
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--workspace requires a value\n")
				os.Exit(1)
			}
			i++
			workspace = args[i]
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	if fromV1 == "" {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain db:migrate --from-v1 <sqlite-path> [--workspace <hash>]")
		os.Exit(1)
	}

	if workspace == "" {
		workspace = "default"
	}

	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Database.URL == "" {
		fmt.Fprintln(os.Stderr, "Error: database.url not configured")
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	queries := sqlc.New(db)

	writer := &sqlcWriter{q: queries}

	m, err := migrate.NewV1Migrator(fromV1, writer)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening v1 database: %v\n", err)
		os.Exit(1)
	}
	defer m.Close()

	fmt.Printf("Migrating from %s to PostgreSQL (workspace: %s)...\n", fromV1, workspace)

	res, err := m.Migrate(ctx, workspace, func(current, total int) {
		fmt.Printf("\rMigrated %d / %d documents", current, total)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nMigration failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("Migration complete: %d migrated, %d skipped, %d failed, %d total\n", res.Migrated, res.Skipped, res.Failed, res.Total)
	if len(res.Errors) > 0 {
		fmt.Printf("Errors (%d):\n", len(res.Errors))
		for _, e := range res.Errors {
			fmt.Printf("  - %s\n", e)
		}
	}
	fmt.Println("Run 'nano-brain embed' to regenerate embeddings.")
}

type sqlcWriter struct {
	q *sqlc.Queries
}

func (w *sqlcWriter) UpsertDocument(ctx context.Context, p migrate.UpsertParams) error {
	_, err := w.q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: p.WorkspaceHash,
		ContentHash:   p.ContentHash,
		Title:         p.Title,
		Content:       p.Content,
		SourcePath:    p.SourcePath,
		Collection:    p.Collection,
		Tags:          p.Tags,
		Metadata:      pqtype.NullRawMessage{},
		SupersedesID:  uuid.NullUUID{},
	})
	return err
}


