// Command ignorecleanup removes already-indexed rows that the watcher now
// excludes: files under build/vendor directories (defaultExcludeDirs in
// internal/watcher/filter.go) and minified / generated bundles (handled by
// isMinified in internal/graph/registry.go). Use it to bring an existing index
// in line with the current ignore rules without a full reindex.
//
// SAFE BY DEFAULT: it only reports (dry-run). Pass -delete to actually remove
// rows. Deleting documents cascades to their chunks + embeddings (ON DELETE
// CASCADE); graph_edges are deleted directly (they have no child rows).
//
// It matches three ways against source_file / source_path:
//   - directory   : '%/NAME/%'  (segment-exact, e.g. node_modules, .output, .svelte-kit)
//   - suffix      : '%SUFFIX'   (e.g. .min.js, .bundle.js, .chunk.js, .map)
//   - contains    : '%FRAG%'    (e.g. jquery, /_nuxt/, /_next/)
//
// Usage:
//
//	go run ./tools/ignorecleanup -ws <hash>            # dry-run, one workspace
//	go run ./tools/ignorecleanup -all                  # dry-run, every workspace
//	go run ./tools/ignorecleanup -all -delete          # execute across all repos
//	go run ./tools/ignorecleanup -ws <hash> -extra .agents
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultDSN = "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev"

// excludeDirs: directory names matched segment-exact ('%/NAME/%'). Mirrors the
// watcher's defaultExcludeDirs plus common framework build/vendor output dirs.
var excludeDirs = []string{
	"node_modules", ".git", ".hg", ".svn", "dist", "build", "out",
	".next", ".nuxt", ".output", "_nuxt", "_next",
	".svelte-kit", ".angular", ".astro", ".turbo", ".vercel", ".netlify", ".parcel-cache",
	"bower_components", "jspm_packages", "web_modules",
	"vendor", "__pycache__", ".pytest_cache", ".mypy_cache", ".tox",
	"venv", ".venv", "env", ".cache", "coverage", ".terraform", "target",
	".worktrees", ".pr-reviews", ".opencode",
}

// minifiedSuffixes: filename suffixes matched as '%SUFFIX'. Includes generated
// lock / resolved manifest files (path ends with the name), plus `.lock` which
// catches yarn.lock, Cargo.lock, Gemfile.lock, composer.lock, poetry.lock,
// Pipfile.lock, pubspec.lock, mix.lock, flake.lock, Podfile.lock, …
var minifiedSuffixes = []string{
	".min.js", ".min.mjs", ".min.cjs", ".min.css",
	".bundle.js", ".chunk.js", ".chunk.mjs", ".umd.js", ".map",
	".lock",
	"package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "bun.lockb",
	"packages.lock.json", "go.sum", "Package.resolved", "gradle.lockfile",
	".terraform.lock.hcl",
}

// vendorContains: filename fragments matched as '%FRAG%'. Kept narrow and
// path-anchored to avoid matching real source (e.g. polyfills.ts, runtime.ts).
var vendorContains = []string{
	"jquery", "/_nuxt/", "/_next/",
}

func main() {
	ws := flag.String("ws", "", "workspace hash (single workspace)")
	all := flag.Bool("all", false, "process every workspace in the database")
	dsn := flag.String("dsn", envOr("DATABASE_URL", defaultDSN), "postgres DSN")
	extra := flag.String("extra", "", "comma-separated extra directory names to match")
	doDelete := flag.Bool("delete", false, "actually delete (default: dry-run)")
	flag.Parse()
	if *ws == "" && !*all {
		fmt.Fprintln(os.Stderr, "error: pass -ws <workspace_hash> or -all")
		os.Exit(2)
	}

	dirs := append([]string{}, excludeDirs...)
	for _, e := range strings.Split(*extra, ",") {
		if e = strings.TrimSpace(e); e != "" {
			dirs = append(dirs, e)
		}
	}
	sufs := minifiedSuffixes
	cont := vendorContains

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		fail("connect", err)
	}
	defer pool.Close()

	// Resolve target workspaces.
	var workspaces []string
	if *all {
		workspaces = listWorkspaces(ctx, pool)
	} else {
		workspaces = []string{*ws}
	}

	mode := "DRY-RUN (nothing will be deleted)"
	if *doDelete {
		mode = "DELETE"
	}
	fmt.Printf("dsn:        %s\nmode:       %s\nworkspaces: %d\n\n", redact(*dsn), mode, len(workspaces))

	// In single-workspace mode, show the per-pattern breakdown so you can spot
	// anything unexpected. In -all mode that would be far too verbose.
	if len(workspaces) == 1 {
		fmt.Printf("  %-22s %10s %10s\n", "pattern", "edges", "docs")
		previewCat(ctx, pool, workspaces[0], "dir", dirs)
		previewCat(ctx, pool, workspaces[0], "suffix", sufs)
		previewCat(ctx, pool, workspaces[0], "contains", cont)
		fmt.Println()
	}

	// Per-workspace totals + grand total.
	fmt.Printf("  %-16s %10s %10s\n", "workspace", "edges", "docs")
	var grandEdges, grandDocs, grandChunks int64
	for _, w := range workspaces {
		e, d, ch := countFor(ctx, pool, w, dirs, sufs, cont)
		grandEdges += e
		grandDocs += d
		grandChunks += ch
		if e > 0 || d > 0 {
			fmt.Printf("  %-16s %10d %10d\n", short(w), e, d)
		}
	}
	fmt.Printf("  %-16s %10d %10d\n", "TOTAL", grandEdges, grandDocs)
	fmt.Printf("  documents delete cascades to ~%d chunks (+ their embeddings)\n", grandChunks)

	if !*doDelete {
		fmt.Println("\nDry-run only. Re-run with -delete to remove these rows.")
		return
	}

	var delEdges, delDocs int64
	for _, w := range workspaces {
		e, d := deleteFor(ctx, pool, w, dirs, sufs, cont)
		delEdges += e
		delDocs += d
	}
	fmt.Printf("\n✅ deleted %d graph_edges and %d documents across %d workspace(s) (chunks + embeddings cascaded)\n",
		delEdges, delDocs, len(workspaces))
	fmt.Println("Re-run tools/edgecheck to confirm the noise is gone.")
}

// listWorkspaces returns every workspace hash present in the data (graph_edges
// or documents), so unregistered/orphaned workspaces are cleaned too.
func listWorkspaces(ctx context.Context, pool *pgxpool.Pool) []string {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT workspace_hash FROM (
			SELECT workspace_hash FROM graph_edges
			UNION
			SELECT workspace_hash FROM documents
		) t WHERE workspace_hash <> '' ORDER BY 1`)
	if err != nil {
		fail("list workspaces", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			fail("scan workspace", err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		fail("rows", err)
	}
	return out
}

func countFor(ctx context.Context, pool *pgxpool.Pool, ws string, dirs, sufs, cont []string) (edges, docs, chunks int64) {
	_ = pool.QueryRow(ctx, countSQL("graph_edges", "source_file"), ws, dirs, sufs, cont).Scan(&edges)
	_ = pool.QueryRow(ctx, countSQL("documents", "source_path"), ws, dirs, sufs, cont).Scan(&docs)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM chunks WHERE document_id IN (
		SELECT id FROM documents WHERE workspace_hash=$1 AND `+predicate("source_path")+`)`, ws, dirs, sufs, cont).Scan(&chunks)
	return edges, docs, chunks
}

func deleteFor(ctx context.Context, pool *pgxpool.Pool, ws string, dirs, sufs, cont []string) (edges, docs int64) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		fail("begin", err)
	}
	defer tx.Rollback(ctx)

	et, err := tx.Exec(ctx, `DELETE FROM graph_edges WHERE workspace_hash=$1 AND `+predicate("source_file"), ws, dirs, sufs, cont)
	if err != nil {
		fail("delete edges", err)
	}
	dt, err := tx.Exec(ctx, `DELETE FROM documents WHERE workspace_hash=$1 AND `+predicate("source_path"), ws, dirs, sufs, cont)
	if err != nil {
		fail("delete docs", err)
	}
	if err := tx.Commit(ctx); err != nil {
		fail("commit", err)
	}
	return et.RowsAffected(), dt.RowsAffected()
}

// predicate ORs the three match categories over $2 (dirs), $3 (suffixes),
// $4 (contains) for the given column.
func predicate(col string) string {
	return `(` +
		`EXISTS (SELECT 1 FROM unnest($2::text[]) p WHERE ` + col + ` ILIKE '%/'||p||'/%') OR ` +
		`EXISTS (SELECT 1 FROM unnest($3::text[]) p WHERE ` + col + ` ILIKE '%'||p) OR ` +
		`EXISTS (SELECT 1 FROM unnest($4::text[]) p WHERE ` + col + ` ILIKE '%'||p||'%')` +
		`)`
}

func countSQL(table, col string) string {
	return `SELECT count(*) FROM ` + table + ` WHERE workspace_hash=$1 AND ` + predicate(col)
}

// previewCat prints per-pattern edge/doc counts for one match category.
func previewCat(ctx context.Context, pool *pgxpool.Pool, ws, kind string, pats []string) {
	for _, p := range pats {
		var like string
		switch kind {
		case "dir":
			like = "%/" + p + "/%"
		case "suffix":
			like = "%" + p
		default:
			like = "%" + p + "%"
		}
		var e, d int64
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM graph_edges WHERE workspace_hash=$1 AND source_file ILIKE $2`, ws, like).Scan(&e)
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM documents WHERE workspace_hash=$1 AND source_path ILIKE $2`, ws, like).Scan(&d)
		if e > 0 || d > 0 {
			fmt.Printf("  %-22s %10d %10d\n", kind+":"+p, e, d)
		}
	}
}

func short(ws string) string {
	if len(ws) > 12 {
		return ws[:12] + "…"
	}
	return ws
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func redact(dsn string) string {
	at := strings.LastIndexByte(dsn, '@')
	if at < 0 {
		return dsn
	}
	colon := strings.LastIndexByte(dsn[:at], ':')
	if colon < 0 {
		return dsn
	}
	return dsn[:colon+1] + "****" + dsn[at:]
}

func fail(stage string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", stage, err)
	os.Exit(1)
}
