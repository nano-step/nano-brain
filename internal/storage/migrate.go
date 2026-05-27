package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
)

// RunMigrations applies all pending goose migrations from the embedded migrations directory.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, logger zerolog.Logger) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	current, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	logger.Info().Int64("current_version", current).Msg("running database migrations")

	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	after, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to get post-migration version: %w", err)
	}

	logger.Info().Int64("version", after).Msg("database migrations complete")
	return nil
}

// GetCurrentVersion returns the current goose migration version from the database.
func GetCurrentVersion(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return 0, fmt.Errorf("failed to set goose dialect: %w", err)
	}
	return goose.GetDBVersionContext(ctx, db)
}
