package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

func runResetEmbeddingsCmd(args []string) {
	cliLog.Info().Str("cmd", "reset-embeddings").Msg("cli command started")

	var workspaceHash, workspacePath string
	var dryRun bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dry-run":
			dryRun = true
		case strings.HasPrefix(arg, "--workspace="):
			workspaceHash = strings.TrimPrefix(arg, "--workspace=")
		case arg == "--workspace":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--workspace requires a value\n")
				os.Exit(1)
			}
			i++
			workspaceHash = args[i]
		case strings.HasPrefix(arg, "--workspace-path="):
			workspacePath = strings.TrimPrefix(arg, "--workspace-path=")
		case arg == "--workspace-path":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--workspace-path requires a value\n")
				os.Exit(1)
			}
			i++
			workspacePath = args[i]
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", arg)
			os.Exit(1)
		}
	}

	if workspaceHash == "" && workspacePath == "" {
		fmt.Fprintf(os.Stderr, "must specify --workspace=<hash> or --workspace-path=<path>\n")
		os.Exit(1)
	}

	if workspacePath != "" {
		h, err := storage.WorkspaceHash(workspacePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not derive workspace hash from path %q: %v\n", workspacePath, err)
			os.Exit(1)
		}
		workspaceHash = h
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

	if dryRun {
		pendingCount, err := queries.CountPendingChunks(ctx, workspaceHash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error counting pending chunks: %v\n", err)
			os.Exit(1)
		}
		failedCount, err := queries.CountEmbedFailedChunks(ctx, workspaceHash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error counting failed chunks: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Dry run for workspace %s:\n", workspaceHash)
		fmt.Printf("  Chunks pending: %d\n", pendingCount)
		fmt.Printf("  Chunks embed_failed: %d\n", failedCount)
		fmt.Printf("  No changes written.\n")
		cliLog.Info().Str("cmd", "reset-embeddings").Str("workspace", workspaceHash).Bool("dry_run", true).Msg("cli command completed")
		return
	}

	_, err = queries.DeleteEmbeddingsByWorkspace(ctx, workspaceHash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting embeddings: %v\n", err)
		os.Exit(1)
	}

	n, err := queries.ResetEmbedStatus(ctx, workspaceHash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resetting embed status: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Reset %d chunks for workspace %s.\n", n, workspaceHash)
	cliLog.Info().Str("cmd", "reset-embeddings").Str("workspace", workspaceHash).Int64("chunks_reset", n).Msg("cli command completed")
}
