package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

func runCleanupStaleRawCmd(args []string) {
	cliLog.Info().Str("cmd", "cleanup-stale-raw").Msg("cli command started")

	var dryRun bool
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "-h", "--help":
			printCleanupStaleRawUsage()
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\n\n", arg)
			printCleanupStaleRawUsage()
			os.Exit(2)
		}
	}

	configPath := config.DefaultConfigPath()
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
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

	count, err := queries.CountStaleRawOpenCodeDocs(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error counting stale raw docs: %v\n", err)
		os.Exit(1)
	}

	if dryRun {
		fmt.Printf("Dry run: %d stale raw OpenCode session document(s) would be deleted.\n", count)
		fmt.Printf("(Raw docs at 'opencode://session/*' superseded by 'summary://opencode/*' in collection 'session-summary'.)\n")
		fmt.Printf("No changes written.\n")
		cliLog.Info().Str("cmd", "cleanup-stale-raw").Int("stale_count", int(count)).Bool("dry_run", true).Msg("cli command completed")
		return
	}

	if count == 0 {
		fmt.Println("No stale raw OpenCode session documents found. Nothing to do.")
		cliLog.Info().Str("cmd", "cleanup-stale-raw").Int("stale_count", 0).Msg("cli command completed")
		return
	}

	n, err := queries.DeleteStaleRawOpenCodeDocs(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting stale raw docs: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deleted %d stale raw OpenCode session document(s).\n", n)
	cliLog.Info().Str("cmd", "cleanup-stale-raw").Int64("deleted_count", n).Msg("cli command completed")
}

func printCleanupStaleRawUsage() {
	fmt.Println(`Usage: nano-brain cleanup-stale-raw [--dry-run]

Delete stale raw OpenCode session documents (source_path 'opencode://session/*'
in collection 'sessions') that have been superseded by a summarized version
('summary://opencode/*' in collection 'session-summary').

These raw docs are no longer written by the post-PR#192 harvest pipeline.

Flags:
  --dry-run    Report counts only; do not delete anything.`)
}
