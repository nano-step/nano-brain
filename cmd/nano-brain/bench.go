package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/nano-brain/nano-brain/internal/bench"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/embed"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog"
)

func runBenchCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain bench <generate|run|compare> [flags]")
		os.Exit(1)
	}
	switch args[0] {
	case "generate":
		runBenchGenerate(args[1:])
	case "run":
		runBenchRun(args[1:])
	case "compare":
		runBenchCompare(args[1:])
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("pgx", cfg.Database.URL)
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

func runBenchRun(args []string) {
	args = splitEqualsArgs(args)
	var dataset, save string
	var jsonFlag bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dataset":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--dataset requires a value")
				os.Exit(1)
			}
			i++
			dataset = args[i]
		case "--save":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--save requires a value")
				os.Exit(1)
			}
			i++
			save = args[i]
		case "--json":
			jsonFlag = true
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	if dataset == "" {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain bench run --dataset=FILE [--save=FILE] [--json]")
		os.Exit(1)
	}

	datasetBytes, err := os.ReadFile(dataset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read dataset: %v\n", err)
		os.Exit(1)
	}
	var ds bench.BenchmarkDataset
	if err := json.Unmarshal(datasetBytes, &ds); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse dataset: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("pgx", cfg.Database.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	embedder, err := embed.NewFromConfig(cfg.Embedding)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create embedder: %v\n", err)
		os.Exit(1)
	}

	queries := sqlc.New(db)
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	svc := search.NewSearchService(queries, embedder, cfg.Search, logger)

	results, err := bench.Run(ctx, &ds, svc, Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "benchmark failed: %v\n", err)
		os.Exit(1)
	}

	var resultJSON []byte
	if save != "" || jsonFlag {
		var err error
		resultJSON, err = json.MarshalIndent(results, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to marshal results: %v\n", err)
			os.Exit(1)
		}
	}

	if save != "" {
		if err := os.WriteFile(save, resultJSON, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write results: %v\n", err)
			os.Exit(1)
		}
	}

	if jsonFlag {
		fmt.Println(string(resultJSON))
		return
	}

	fmt.Fprintf(os.Stderr, "Benchmark Results (%d queries, workspace %s)\n", results.QueryCount, results.WorkspaceHash)
	fmt.Fprintf(os.Stderr, "  P@5:      %.3f\n", results.PrecisionAt5)
	fmt.Fprintf(os.Stderr, "  R@10:     %.3f\n", results.RecallAt10)
	fmt.Fprintf(os.Stderr, "  MRR:      %.3f\n", results.MRR)
	fmt.Fprintf(os.Stderr, "  P50(ms):  %.1f\n", results.QueryP50ms)
	fmt.Fprintf(os.Stderr, "  P95(ms):  %.1f\n", results.QueryP95ms)
}

func runBenchCompare(args []string) {
	var jsonFlag bool
	var positionalArgs []string

	for _, arg := range args {
		if arg == "--json" {
			jsonFlag = true
		} else {
			positionalArgs = append(positionalArgs, arg)
		}
	}

	if len(positionalArgs) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain bench compare <new.json> <baseline.json> [--json]")
		os.Exit(1)
	}

	newFile := positionalArgs[0]
	baselineFile := positionalArgs[1]

	newBytes, err := os.ReadFile(newFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read new results: %v\n", err)
		os.Exit(1)
	}
	var newResults bench.BenchmarkResults
	if err := json.Unmarshal(newBytes, &newResults); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse new results: %v\n", err)
		os.Exit(1)
	}

	baselineBytes, err := os.ReadFile(baselineFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read baseline results: %v\n", err)
		os.Exit(1)
	}
	var baselineResults bench.BenchmarkResults
	if err := json.Unmarshal(baselineBytes, &baselineResults); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse baseline results: %v\n", err)
		os.Exit(1)
	}

	result := bench.Compare(&newResults, &baselineResults)

	if jsonFlag {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to marshal result: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else {
		fmt.Fprintln(os.Stderr, "Benchmark Comparison")
		fmt.Fprintln(os.Stderr, "====================")
		fmt.Fprintf(os.Stderr, "Status: ")
		if result.Passed {
			fmt.Fprintln(os.Stderr, "PASS")
		} else {
			fmt.Fprintln(os.Stderr, "FAIL")
		}

		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Deltas:")
		fmt.Fprintf(os.Stderr, "  %-15s %12s %12s %12s\n", "Metric", "Baseline", "New", "Change")
		for _, metric := range []string{"P@5", "R@10", "MRR", "Query P50", "Query P95"} {
			if delta, ok := result.Deltas[metric]; ok {
				sign := ""
				if delta.Change >= 0 {
					sign = "+"
				}
				fmt.Fprintf(os.Stderr, "  %-15s %12.4f %12.4f %12s%.4f\n", metric, delta.Baseline, delta.New, sign, delta.Change)
			}
		}

		if len(result.Regressions) > 0 {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Regressions:")
			for _, reg := range result.Regressions {
				fmt.Fprintf(os.Stderr, "  %s\n", reg.Message)
			}
		}
	}

	if !result.Passed {
		os.Exit(1)
	}
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
