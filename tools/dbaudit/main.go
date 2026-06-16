// Command dbaudit reports redundancy across the whole nano-brain database:
// duplicate documents/chunks, duplicate embeddings, orphans, cross-workspace
// shared content, indexed build/vendor/minified noise, and stale flow docs.
// READ-ONLY (only SELECTs).
//
// Usage:
//
//	go run ./tools/dbaudit
//	DATABASE_URL="postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev" go run ./tools/dbaudit
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultDSN = "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev"

// Noise patterns (mirror tools/ignorecleanup): build/vendor dirs + minified suffixes.
var noiseDirs = []string{"node_modules", ".git", "dist", "build", "out", ".next", ".nuxt", ".output",
	"_nuxt", "_next", ".svelte-kit", ".angular", "vendor", "__pycache__", "venv", ".venv", "coverage",
	"target", ".worktrees", ".pr-reviews", ".opencode", "bower_components"}
var noiseSuffixes = []string{".min.js", ".min.mjs", ".min.css", ".bundle.js", ".chunk.js", ".umd.js", ".map",
	".lock", "package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "bun.lockb",
	"packages.lock.json", "go.sum", "Package.resolved", "gradle.lockfile", ".terraform.lock.hcl"}

func main() {
	dsn := defaultDSN
	if v := os.Getenv("DATABASE_URL"); v != "" {
		dsn = v
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fail("connect", err)
	}
	defer pool.Close()

	fmt.Printf("dsn: %s\n\n", redact(dsn))

	// ---- Global row counts ----
	section("Row counts")
	for _, t := range []string{"documents", "chunks", "embeddings", "graph_edges", "chunk_entities", "graph_context", "graph_pagerank", "collections", "workspaces"} {
		n := tableCount(ctx, pool, t)
		if n < 0 {
			fmt.Printf("  %-16s %12s\n", t, "(absent)")
		} else {
			fmt.Printf("  %-16s %12d\n", t, n)
		}
	}

	// ---- Per-workspace size + chunk/doc ratio + noise ----
	section("Per workspace (docs / chunks / ratio / noise-docs / noise-edges)")
	rows, err := pool.Query(ctx, `
		SELECT w.hash, w.name,
		  (SELECT count(*) FROM documents d WHERE d.workspace_hash=w.hash) docs,
		  (SELECT count(*) FROM chunks c WHERE c.workspace_hash=w.hash) chunks
		FROM workspaces w ORDER BY 4 DESC`)
	if err != nil {
		fail("ws", err)
	}
	type wsRow struct{ hash, name string; docs, chunks int64 }
	var wss []wsRow
	for rows.Next() {
		var r wsRow
		if err := rows.Scan(&r.hash, &r.name, &r.docs, &r.chunks); err != nil {
			fail("scan ws", err)
		}
		wss = append(wss, r)
	}
	rows.Close()
	for _, w := range wss {
		ratio := 0.0
		if w.docs > 0 {
			ratio = float64(w.chunks) / float64(w.docs)
		}
		nd := scalar(ctx, pool, noiseCountSQL("documents", "source_path"), w.hash, noiseDirs, noiseSuffixes)
		ne := scalar(ctx, pool, noiseCountSQL("graph_edges", "source_file"), w.hash, noiseDirs, noiseSuffixes)
		flag := ""
		if ratio >= 4 {
			flag = "  ⚠ high ratio"
		}
		fmt.Printf("  %-26s %8d %10d  %5.1f  noise:%7d docs %8d edges%s\n", trunc(w.name, 26), w.docs, w.chunks, ratio, nd, ne, flag)
	}

	// ---- Redundancy ----
	section("Redundancy")
	fmt.Printf("  duplicate documents (same content_hash+workspace, extra copies): %d\n",
		scalar(ctx, pool, `SELECT coalesce(sum(c-1),0) FROM (SELECT content_hash,workspace_hash,count(*) c FROM documents GROUP BY 1,2 HAVING count(*)>1) t`))
	fmt.Printf("  duplicate chunks   (same content_hash+workspace across docs):    %d\n",
		scalar(ctx, pool, `SELECT coalesce(sum(c-1),0) FROM (SELECT content_hash,workspace_hash,count(*) c FROM chunks GROUP BY 1,2 HAVING count(*)>1) t`))
	fmt.Printf("  duplicate embeddings (chunks with >1 embedding → extra rows):    %d\n",
		scalar(ctx, pool, `SELECT coalesce(sum(c-1),0) FROM (SELECT chunk_id,count(*) c FROM embeddings GROUP BY chunk_id HAVING count(*)>1) t`))
	fmt.Printf("  cross-workspace shared content (content_hash in >1 workspace):   %d\n",
		scalar(ctx, pool, `SELECT count(*) FROM (SELECT content_hash FROM documents GROUP BY content_hash HAVING count(DISTINCT workspace_hash)>1) t`))

	// ---- Orphans / gaps ----
	section("Orphans & gaps")
	fmt.Printf("  documents with 0 chunks:            %d\n",
		scalar(ctx, pool, `SELECT count(*) FROM documents d WHERE NOT EXISTS (SELECT 1 FROM chunks c WHERE c.document_id=d.id)`))
	fmt.Printf("  chunks with 0 embeddings:           %d\n",
		scalar(ctx, pool, `SELECT count(*) FROM chunks c WHERE NOT EXISTS (SELECT 1 FROM embeddings e WHERE e.chunk_id=c.id)`))
	fmt.Printf("  embeddings with missing chunk:      %d\n",
		scalar(ctx, pool, `SELECT count(*) FROM embeddings e WHERE NOT EXISTS (SELECT 1 FROM chunks c WHERE c.id=e.chunk_id)`))

	// ---- Flows collection ----
	section("Flow docs vs http entries")
	fmt.Printf("  documents in collection 'flows':    %d\n", scalar(ctx, pool, `SELECT count(*) FROM documents WHERE collection='flows'`))
	fmt.Printf("  distinct http entry nodes:          %d\n", scalar(ctx, pool, `SELECT count(DISTINCT source_node) FROM graph_edges WHERE edge_type='http'`))

	// ---- What are the 0-chunk documents? ----
	section("Documents with 0 chunks, by collection (top 12)")
	er, err := pool.Query(ctx, `SELECT coalesce(collection,'(none)'), count(*) c
		FROM documents d WHERE NOT EXISTS (SELECT 1 FROM chunks ch WHERE ch.document_id=d.id)
		GROUP BY 1 ORDER BY c DESC LIMIT 12`)
	if err == nil {
		for er.Next() {
			var coll string
			var c int64
			if er.Scan(&coll, &c) == nil {
				fmt.Printf("  %-28s %10d\n", coll, c)
			}
		}
		er.Close()
	}

	fmt.Println("\nNote: high chunk/doc ratios + large noise counts usually mean minified/build files were indexed — run tools/ignorecleanup -all -delete, then reindex.")
}

// noiseCountSQL counts rows whose path matches a noise dir ($2) or suffix ($3).
func noiseCountSQL(table, col string) string {
	return `SELECT count(*) FROM ` + table + ` WHERE workspace_hash=$1 AND (` +
		`EXISTS (SELECT 1 FROM unnest($2::text[]) p WHERE ` + col + ` ILIKE '%/'||p||'/%') OR ` +
		`EXISTS (SELECT 1 FROM unnest($3::text[]) p WHERE ` + col + ` ILIKE '%'||p))`
}

// tableCount returns the row count, or -1 if the table doesn't exist.
func tableCount(ctx context.Context, pool *pgxpool.Pool, t string) int64 {
	var reg *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.'||$1)::text`, t).Scan(&reg); err != nil || reg == nil {
		return -1
	}
	return scalar(ctx, pool, "SELECT count(*) FROM "+t)
}

func scalar(ctx context.Context, pool *pgxpool.Pool, q string, args ...any) int64 {
	var n int64
	if err := pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
		fmt.Fprintf(os.Stderr, "query failed: %v\n  sql: %s\n", err, q)
		return -1
	}
	return n
}

func section(s string) { fmt.Printf("== %s ==\n", s) }
func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}
func redact(dsn string) string {
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
	c := -1
	for i := at; i >= 0; i-- {
		if dsn[i] == ':' {
			c = i
			break
		}
	}
	if c < 0 || c >= at {
		return dsn
	}
	return dsn[:c+1] + "****" + dsn[at:]
}
func fail(stage string, err error) { fmt.Fprintf(os.Stderr, "%s: %v\n", stage, err); os.Exit(1) }
