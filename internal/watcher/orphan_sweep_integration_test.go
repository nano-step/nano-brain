//go:build integration

package watcher

import (
	"context"
	"database/sql"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

// Issue #535: graph_edges / function_flowcharts have no FK to documents, so
// orphan rows must be swept explicitly. These tests exercise the two new
// reconciliation paths directly against real Postgres (they use only w.db).

func newSweepTestWatcher(t *testing.T) (*Watcher, *sql.DB) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	cfg := config.Config{Storage: config.StorageConfig{MaxFileSize: 1 << 20}}
	w := New(db, sqlc.New(db), zerolog.Nop(), cfg)
	return w, db
}

// resetWorkspace clears any rows for ws so the test is re-runnable even if a
// prior crashed run leaked its per-test schema (SetupTestDB reuses an existing
// schema via CREATE SCHEMA IF NOT EXISTS).
func resetWorkspace(t *testing.T, db *sql.DB, ws string) {
	t.Helper()
	for _, tbl := range []string{"graph_edges", "function_flowcharts"} {
		if _, err := db.Exec(`DELETE FROM `+tbl+` WHERE workspace_hash = $1`, ws); err != nil {
			t.Fatalf("reset %s: %v", tbl, err)
		}
	}
}

func seedEdge(t *testing.T, db *sql.DB, ws, sourceFile, edgeType string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO graph_edges (workspace_hash, source_node, target_node, edge_type, source_file)
		 VALUES ($1, $2, $3, $4, $5)`,
		ws, sourceFile+"::fn", "callee-"+sourceFile+"-"+edgeType, edgeType, sourceFile)
	if err != nil {
		t.Fatalf("seed edge %q: %v", sourceFile, err)
	}
}

func seedFlowchart(t *testing.T, db *sql.DB, ws, sourceFile string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO function_flowcharts (workspace_hash, entry, source_file, start_line, end_line, status, cfg)
		 VALUES ($1, $2, $3, 1, 2, 'complete', '{}')`,
		ws, sourceFile+"::fn", sourceFile)
	if err != nil {
		t.Fatalf("seed flowchart %q: %v", sourceFile, err)
	}
}

func edgeFiles(t *testing.T, db *sql.DB, ws string) []string {
	t.Helper()
	return distinctSourceFiles(t, db, "graph_edges", ws)
}

func flowchartFiles(t *testing.T, db *sql.DB, ws string) []string {
	t.Helper()
	return distinctSourceFiles(t, db, "function_flowcharts", ws)
}

func distinctSourceFiles(t *testing.T, db *sql.DB, table, ws string) []string {
	t.Helper()
	rows, err := db.Query(
		`SELECT DISTINCT source_file FROM `+table+` WHERE workspace_hash = $1 ORDER BY 1`, ws)
	if err != nil {
		t.Fatalf("query %s: %v", table, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan %s: %v", table, err)
		}
		out = append(out, s)
	}
	return out
}

func eq(a, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestSweepOrphanGraphRows: files not in the admitted set are removed; admitted
// files are kept; an empty admitted set is a no-op (never wipes the graph).
func TestSweepOrphanGraphRows(t *testing.T) {
	w, db := newSweepTestWatcher(t)
	ctx := context.Background()
	ws := "ws-sweep-535"
	resetWorkspace(t, db, ws)

	seedEdge(t, db, ws, "keep.ts", "calls")               // admitted
	seedEdge(t, db, ws, "sub/generated/big.ts", "calls")  // gitignored -> orphan
	seedEdge(t, db, ws, "deleted.ts", "imports")          // deleted -> orphan
	seedFlowchart(t, db, ws, "keep.ts")                   // admitted
	seedFlowchart(t, db, ws, "sub/generated/big.ts")      // orphan

	// Empty admitted must NOT delete anything.
	w.sweepOrphanGraphRows(ctx, ws, map[string]bool{})
	if got := edgeFiles(t, db, ws); len(got) != 3 {
		t.Fatalf("empty admitted wiped rows: got %v, want 3 files intact", got)
	}

	admitted := map[string]bool{"keep.ts": true, "/abs/keep.ts": true}
	w.sweepOrphanGraphRows(ctx, ws, admitted)

	if got := edgeFiles(t, db, ws); !eq(got, []string{"keep.ts"}) {
		t.Errorf("edges after sweep = %v, want [keep.ts]", got)
	}
	if got := flowchartFiles(t, db, ws); !eq(got, []string{"keep.ts"}) {
		t.Errorf("flowcharts after sweep = %v, want [keep.ts]", got)
	}
}

// TestDeleteGraphRowsForFile: both stored source_file forms (workspace-relative
// and absolute) of the target file are removed; unrelated files are untouched.
func TestDeleteGraphRowsForFile(t *testing.T) {
	w, db := newSweepTestWatcher(t)
	ctx := context.Background()
	ws := "ws-del-535"
	dir := "/repo/root"
	resetWorkspace(t, db, ws)

	seedEdge(t, db, ws, "pkg/a.ts", "calls")              // relative form
	seedEdge(t, db, ws, "/repo/root/pkg/a.ts", "imports") // absolute form of same file
	seedEdge(t, db, ws, "pkg/other.ts", "calls")          // unrelated -> survives
	seedFlowchart(t, db, ws, "pkg/a.ts")

	col := watchedCollection{workspaceHash: ws, dirPath: dir}
	w.deleteGraphRowsForFile(ctx, col, filepath.Join(dir, "pkg", "a.ts"))

	if got := edgeFiles(t, db, ws); !eq(got, []string{"pkg/other.ts"}) {
		t.Errorf("edges after delete = %v, want [pkg/other.ts]", got)
	}
	if got := flowchartFiles(t, db, ws); len(got) != 0 {
		t.Errorf("flowcharts after delete = %v, want none", got)
	}
}
