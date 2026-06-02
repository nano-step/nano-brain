package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

func runCleanupOrphanWorkspacesCmd(args []string) {
	cliLog.Info().Str("cmd", "cleanup-orphan-workspaces").Msg("cli command started")

	var dryRun bool
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "-h", "--help":
			printCleanupOrphanWorkspacesUsage()
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\n\n", arg)
			printCleanupOrphanWorkspacesUsage()
			os.Exit(2)
		}
	}

	warnIfServerRunning()

	configPath := config.ResolveConfigPath("")
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %s\n", storage.RedactError(err))
		os.Exit(1)
	}
	defer pool.Close()

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	q := sqlc.New(db)

	orphans, err := q.ListOrphanDocumentWorkspaces(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing orphan workspaces: %v\n", err)
		os.Exit(1)
	}

	if len(orphans) == 0 {
		fmt.Println("No orphan documents found. DB is clean.")
		cliLog.Info().Str("cmd", "cleanup-orphan-workspaces").Int("orphan_workspaces", 0).Msg("cli command completed")
		return
	}

	totalDocs := int64(0)
	for _, o := range orphans {
		totalDocs += o.DocCount
	}

	chunkCount, err := q.CountOrphanChunks(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error counting orphan chunks: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d documents (and %d chunks) under %d unregistered workspace_hash values:\n", totalDocs, chunkCount, len(orphans))
	for _, o := range orphans {
		fmt.Printf("  %s → %d docs\n", o.WorkspaceHash, o.DocCount)
	}

	if dryRun {
		fmt.Println()
		fmt.Println("Run without --dry-run to apply.")
		cliLog.Info().
			Str("cmd", "cleanup-orphan-workspaces").
			Int("orphan_workspaces", len(orphans)).
			Int64("orphan_docs", totalDocs).
			Int64("orphan_chunks", chunkCount).
			Bool("dry_run", true).
			Msg("cli command completed")
		return
	}

	deletedDocs, err := q.DeleteOrphanDocuments(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting orphan documents: %v\n", err)
		os.Exit(1)
	}
	deletedChunks, err := q.DeleteOrphanChunks(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting orphan chunks: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deleted %d documents + %d chunks across %d unregistered workspaces.\n", deletedDocs, deletedChunks, len(orphans))
	fmt.Println("(Embeddings cascade-deleted via existing chunks(id) -> embeddings FK.)")
	cliLog.Info().
		Str("cmd", "cleanup-orphan-workspaces").
		Int("orphan_workspaces", len(orphans)).
		Int64("deleted_docs", deletedDocs).
		Int64("deleted_chunks", deletedChunks).
		Msg("cli command completed")
}

func warnIfServerRunning() {
	for _, port := range []int{3100, 8899} {
		url := fmt.Sprintf("http://localhost:%d/health", port)
		client := http.Client{Timeout: 500 * time.Millisecond}
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Fprintf(os.Stderr, "WARNING: nano-brain server appears to be running on port %d. Concurrent harvests could re-create orphans. Stop the server before cleanup for safety.\n\n", port)
				return
			}
		}
	}
}

func printCleanupOrphanWorkspacesUsage() {
	fmt.Println(`Usage: nano-brain cleanup-orphan-workspaces [--dry-run]

Delete documents and chunks whose workspace_hash is not present in the
workspaces table. Required before migration 00011 (issue #238), which adds
foreign-key constraints from documents/chunks to workspaces.

This is a one-way operation: orphan documents (typically LLM-generated
session summaries) are PERMANENTLY deleted. Save the --dry-run output as
a backup before applying.

Flags:
  --dry-run    Report counts only; do not delete anything.`)
}
