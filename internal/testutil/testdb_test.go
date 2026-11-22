//go:build integration

package testutil

import (
	"context"
	"testing"
)

func TestTestSchemaSameNameIsUnique(t *testing.T) {
	first := testSchema("TestSharedName")
	second := testSchema("TestSharedName")
	if first == second {
		t.Fatalf("two test invocations use the same schema: %s", first)
	}
	for _, name := range []string{first, second} {
		if len(name) > 63 {
			t.Errorf("schema %s exceeds PostgreSQL's identifier limit", name)
		}
	}
}

func TestSetupTestDBSameTestCannotSeeOtherRows(t *testing.T) {
	first := SetupTestDB(t)
	second := SetupTestDB(t)
	ctx := context.Background()

	var firstSchema, secondSchema string
	if err := first.QueryRow(ctx, "SELECT current_schema()").Scan(&firstSchema); err != nil {
		t.Fatal(err)
	}
	if err := second.QueryRow(ctx, "SELECT current_schema()").Scan(&secondSchema); err != nil {
		t.Fatal(err)
	}
	if firstSchema == secondSchema {
		t.Fatalf("two connections share schema %s", firstSchema)
	}
	if _, err := first.Exec(ctx, "INSERT INTO workspaces (hash, name, path) VALUES ('fixture', 'fixture', '/tmp/fixture')"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := second.QueryRow(ctx, "SELECT count(*) FROM workspaces WHERE hash = 'fixture'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("second test schema sees %d rows inserted by first", count)
	}
}
