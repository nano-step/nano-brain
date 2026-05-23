package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nano-brain/nano-brain/internal/bench"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	_ "github.com/lib/pq"
)

func runBenchCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain bench <generate> [flags]")
		os.Exit(1)
	}
	switch args[0] {
	case "generate":
		runBenchGenerate(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown bench subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func splitEqualsArgs(args []string) []string {
	var out []string
	for _, a := range args {
		if k, v, ok := strings.Cut(a, "="); ok && strings.HasPrefix(k, "--") {
			out = append(out, k, v)
		} else {
			out = append(out, a)
		}
	}
	return out
}

func runBenchGenerate(args []string) {
	args = splitEqualsArgs(args)
	var workspace, output string
	var scale int
	var jsonFlag bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--scale":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--scale requires a value")
				os.Exit(1)
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				fmt.Fprintln(os.Stderr, "--scale must be a positive integer")
				os.Exit(1)
			}
			scale = n
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--workspace requires a value")
				os.Exit(1)
			}
			i++
			workspace = args[i]
		case "--output":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--output requires a value")
				os.Exit(1)
			}
			i++
			output = args[i]
		case "--json":
			jsonFlag = true
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	if workspace == "" || scale == 0 {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain bench generate --scale=N --workspace=HASH [--output=FILE] [--json]")
		os.Exit(1)
	}

	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	db, err := sql.Open("postgres", cfg.Database.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	queries := sqlc.New(db)
	store := &sqlcAdapter{q: queries}

	dataset, err := bench.Generate(ctx, store, workspace, scale)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(dataset, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal dataset: %v\n", err)
		os.Exit(1)
	}

	if output != "" {
		if err := os.WriteFile(output, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write output file: %v\n", err)
			os.Exit(1)
		}
		if !jsonFlag {
			fmt.Printf("Dataset written to %s (%d entries)\n", output, len(dataset.Entries))
		}
		return
	}

	fmt.Println(string(data))
}

type sqlcAdapter struct {
	q *sqlc.Queries
}

func (a *sqlcAdapter) ListDocumentsByWorkspace(ctx context.Context, workspaceHash string) ([]bench.DocumentRow, error) {
	rows, err := a.q.ListDocumentsByWorkspace(ctx, workspaceHash)
	if err != nil {
		return nil, err
	}
	result := make([]bench.DocumentRow, len(rows))
	for i, r := range rows {
		result[i] = bench.DocumentRow{
			ID:            r.ID,
			WorkspaceHash: r.WorkspaceHash,
			ContentHash:   r.ContentHash,
			Title:         r.Title,
			SourcePath:    r.SourcePath,
			Collection:    r.Collection,
		}
	}
	return result, nil
}
