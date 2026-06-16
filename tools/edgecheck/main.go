// Command edgecheck inspects graph_edges / documents for a workspace and reports
// total counts, a breakdown by edge_type, and how much comes from noisy build /
// tooling directories (.output, _nuxt, node_modules, .worktrees, .pr-reviews,
// .opencode). Read-only: it issues only SELECTs.
//
// Usage:
//
//	go run ./tools/edgecheck -ws <workspace_hash>
//	DATABASE_URL="postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev" \
//	  go run ./tools/edgecheck -ws d1915ee1...
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultDSN = "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev"

// noisePatterns are matched against source_file / source_path with ILIKE '%pat%'.
var noisePatterns = []string{".output", "_nuxt", "node_modules", ".worktrees", ".pr-reviews", ".opencode", ".next", ".nuxt", "dist/", "build/"}

func main() {
	ws := flag.String("ws", "", "workspace hash (required)")
	dsn := flag.String("dsn", envOr("DATABASE_URL", defaultDSN), "postgres DSN")
	top := flag.Int("top", 20, "number of top source_file rows to list")
	flag.Parse()
	if *ws == "" {
		fmt.Fprintln(os.Stderr, "error: -ws <workspace_hash> is required")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		fail("connect", err)
	}
	defer pool.Close()

	fmt.Printf("workspace: %s\ndsn:       %s\n\n", *ws, redact(*dsn))

	// 1. Totals.
	var totalEdges, totalDocs int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM graph_edges WHERE workspace_hash=$1`, *ws).Scan(&totalEdges); err != nil {
		fail("count edges", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM documents WHERE workspace_hash=$1`, *ws).Scan(&totalDocs); err != nil {
		fail("count docs", err)
	}
	fmt.Printf("graph_edges: %d\ndocuments:   %d\n\n", totalEdges, totalDocs)

	if totalEdges == 0 {
		fmt.Println("⚠️  ZERO graph edges — flows will be empty. Reindex this workspace with the new binary.")
	}

	// 2. Edges by type.
	fmt.Println("edges by type:")
	printAgg(ctx, pool,
		`SELECT edge_type, count(*) FROM graph_edges WHERE workspace_hash=$1 GROUP BY edge_type ORDER BY 2 DESC`, *ws)

	// 3. Noise breakdown (edges + docs).
	fmt.Println("\nnoise breakdown (ILIKE match on path):")
	fmt.Printf("  %-16s %10s %10s\n", "pattern", "edges", "docs")
	var noisyEdges, noisyDocs int64
	for _, p := range noisePatterns {
		var e, d int64
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM graph_edges WHERE workspace_hash=$1 AND source_file ILIKE '%'||$2||'%'`, *ws, p).Scan(&e)
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM documents WHERE workspace_hash=$1 AND source_path ILIKE '%'||$2||'%'`, *ws, p).Scan(&d)
		if e > 0 || d > 0 {
			fmt.Printf("  %-16s %10d %10d\n", p, e, d)
		}
		noisyEdges += e
		noisyDocs += d
	}
	pct := 0.0
	if totalEdges > 0 {
		pct = 100 * float64(noisyEdges) / float64(totalEdges)
	}
	fmt.Printf("  %-16s %10d %10d   (%.1f%% of edges)\n", "TOTAL", noisyEdges, noisyDocs, pct)
	if noisyEdges == 0 && noisyDocs == 0 {
		fmt.Println("  clean ✅  no edges/docs from excluded dirs")
	}

	// 4. Top source files by edge count (eyeball remaining noise).
	fmt.Printf("\ntop %d source_file by edge count:\n", *top)
	printAgg(ctx, pool,
		`SELECT source_file, count(*) FROM graph_edges WHERE workspace_hash=$1 GROUP BY source_file ORDER BY 2 DESC LIMIT `+itoa(*top), *ws)
}

func printAgg(ctx context.Context, pool *pgxpool.Pool, q, ws string) {
	rows, err := pool.Query(ctx, q, ws)
	if err != nil {
		fail("query", err)
	}
	defer rows.Close()
	for rows.Next() {
		var label string
		var n int64
		if err := rows.Scan(&label, &n); err != nil {
			fail("scan", err)
		}
		fmt.Printf("  %8d  %s\n", n, label)
	}
	if err := rows.Err(); err != nil {
		fail("rows", err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

func redact(dsn string) string {
	// hide password between ':' and '@'
	at := -1
	for i := len(dsn) - 1; i >= 0; i-- {
		if dsn[i] == '@' {
			at = i
			break
		}
	}
	if at < 0 {
		return dsn
	}
	colon := -1
	for i := at; i >= 0; i-- {
		if dsn[i] == ':' {
			colon = i
			break
		}
	}
	if colon < 0 || colon >= at {
		return dsn
	}
	return dsn[:colon+1] + "****" + dsn[at:]
}

func fail(stage string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", stage, err)
	os.Exit(1)
}
