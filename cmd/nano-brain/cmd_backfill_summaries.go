package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/summarize"
)

func runBackfillSummariesCmd(args []string) {
	var outputDir, workspace, sinceStr string
	var dryRun bool

	for _, arg := range args {
		switch {
		case arg == "--dry-run":
			dryRun = true
		case arg == "-h" || arg == "--help":
			printBackfillSummariesUsage()
			return
		case strings.HasPrefix(arg, "--output-dir="):
			outputDir = strings.TrimPrefix(arg, "--output-dir=")
		case strings.HasPrefix(arg, "--workspace="):
			workspace = strings.TrimPrefix(arg, "--workspace=")
		case strings.HasPrefix(arg, "--since="):
			sinceStr = strings.TrimPrefix(arg, "--since=")
		default:
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\n\n", arg)
			printBackfillSummariesUsage()
			os.Exit(2)
		}
	}

	warnIfBackfillServerRunning()

	configPath := config.ResolveConfigPath("")
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	effectiveDir := outputDir
	if effectiveDir == "" {
		effectiveDir = cfg.Summarization.OutputDir
	}
	if effectiveDir == "" {
		fmt.Fprintln(os.Stderr, "No output directory configured (set summarization.output_dir or --output-dir).")
		os.Exit(1)
	}
	expanded, err := summarize.ExpandTilde(effectiveDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error expanding output dir path: %v\n", err)
		os.Exit(1)
	}
	effectiveDir = expanded

	var sinceTime time.Time
	if sinceStr != "" {
		sinceTime, err = time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			sinceTime, err = time.Parse("2006-01-02", sinceStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid --since (use RFC3339 or YYYY-MM-DD): %v\n", err)
				os.Exit(1)
			}
		}
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
	q := sqlc.New(db)

	wsHashFilter := ""
	if workspace != "" {
		wsHashFilter = resolveBackfillWorkspaceFilter(ctx, q, workspace)
		if wsHashFilter == "" {
			fmt.Fprintf(os.Stderr, "Workspace %q not found.\n", workspace)
			os.Exit(1)
		}
	}

	docs, err := q.ListSummaryDocumentsForBackfill(ctx, sqlc.ListSummaryDocumentsForBackfillParams{
		Column1: wsHashFilter,
		Column2: sinceTime,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying summaries: %v\n", err)
		os.Exit(1)
	}

	if len(docs) == 0 {
		fmt.Println("No summary documents found matching filters.")
		return
	}

	var written, skipped, overwritten, failed int

	for _, doc := range docs {
		wsName := ""
		if ws, err := q.GetWorkspaceByHash(ctx, doc.WorkspaceHash); err == nil {
			wsName = ws.Name
		}

		sessionID := extractBackfillSessionID(doc.Metadata.RawMessage, doc.SourcePath)
		source := extractBackfillSource(doc.Tags)

		titleWithoutPrefix := strings.TrimPrefix(doc.Title, "Summary: ")

		targetPath := summarize.BuildDiskPath(effectiveDir, wsName, doc.WorkspaceHash, source, titleWithoutPrefix, doc.CreatedAt)

		if dryRun {
			fmt.Println(targetPath)
			written++
			continue
		}

		if err := summarize.EnsureDir(targetPath); err != nil {
			fmt.Fprintf(os.Stderr, "ensureDir %s: %v\n", targetPath, err)
			failed++
			continue
		}

		existingContent, readErr := os.ReadFile(targetPath)
		if readErr == nil {
			if string(existingContent) == doc.Content {
				skipped++
				continue
			}
			finalPath, collErr := summarize.ResolveCollision(targetPath, []byte(doc.Content), sessionID)
			if collErr != nil {
				fmt.Fprintf(os.Stderr, "resolveCollision %s: %v\n", targetPath, collErr)
				failed++
				continue
			}
			if err := summarize.WriteFileAtomic(finalPath, []byte(doc.Content)); err != nil {
				fmt.Fprintf(os.Stderr, "write %s: %v\n", finalPath, err)
				failed++
				continue
			}
			overwritten++
		} else if os.IsNotExist(readErr) {
			if err := summarize.WriteFileAtomic(targetPath, []byte(doc.Content)); err != nil {
				fmt.Fprintf(os.Stderr, "write %s: %v\n", targetPath, err)
				failed++
				continue
			}
			written++
		} else {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", targetPath, readErr)
			failed++
		}
	}

	if dryRun {
		fmt.Printf("\n[dry-run] Would write %d files. Run without --dry-run to apply.\n", written)
	} else {
		fmt.Printf("Found %d summaries. Written %d files (%d skipped — identical content, %d overwritten as new path, %d failed).\n",
			len(docs), written, skipped, overwritten, failed)
	}
}

func resolveBackfillWorkspaceFilter(ctx context.Context, q *sqlc.Queries, workspace string) string {
	wsList, err := q.ListWorkspaces(ctx)
	if err != nil {
		return ""
	}
	for _, ws := range wsList {
		if ws.Name == workspace || ws.Hash == workspace {
			return ws.Hash
		}
	}
	return ""
}

func extractBackfillSessionID(metaJSON []byte, sourcePath string) string {
	if len(metaJSON) > 0 {
		var m map[string]any
		if err := json.Unmarshal(metaJSON, &m); err == nil {
			if v, ok := m["session_id"].(string); ok && v != "" {
				return v
			}
		}
	}
	base := filepath.Base(sourcePath)
	return base
}

func extractBackfillSource(tags []string) string {
	for _, t := range tags {
		if t == "opencode" || t == "claude" {
			return t
		}
	}
	return "opencode"
}

func warnIfBackfillServerRunning() {
	for _, port := range []int{3100, 8899} {
		url := fmt.Sprintf("http://localhost:%d/health", port)
		client := http.Client{Timeout: 500 * time.Millisecond}
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Fprintf(os.Stderr, "WARNING: nano-brain server appears to be running on port %d. Concurrent writes could conflict. Consider stopping the server before backfill.\n\n", port)
				return
			}
		}
	}
}

func printBackfillSummariesUsage() {
	fmt.Println(`Usage: nano-brain backfill-summaries [--dry-run] [--output-dir=<path>] [--workspace=<name|hash>] [--since=<RFC3339|YYYY-MM-DD>]

Export existing summary documents from PostgreSQL to disk as .md files.
Used to populate ~/.nano-brain/summaries/ with summaries that predate
the disk-persistence feature.

Flags:
  --output-dir=<path>      Override summarization.output_dir from config.
  --workspace=<name|hash>  Filter by workspace name or hash (default: all).
  --since=<date>           Only summaries created at or after this date
                           (RFC3339 or YYYY-MM-DD format).
  --dry-run                List file paths without writing anything.`)
}
