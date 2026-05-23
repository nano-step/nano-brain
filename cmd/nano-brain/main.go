package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/health"
	"github.com/nano-brain/nano-brain/internal/server"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/sync/errgroup"
)

var Version = "dev"

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file (default: ~/.nano-brain/config.yml)")
	flag.Parse()

	if configPath == "" {
		configPath = config.DefaultConfigPath()
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger, err := health.NewLogger(cfg.Logging)
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}

	logger.Info().
		Str("version", Version).
		Int("port", cfg.Server.Port).
		Msg("nano-brain starting")

	ctx := context.Background()

	pool, err := storage.NewPool(ctx, cfg.Database, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer storage.ClosePool(pool)

	if err := storage.RunMigrations(ctx, pool, logger); err != nil {
		logger.Fatal().Err(err).Msg("failed to run migrations")
	}

	db := stdlib.OpenDBFromPool(pool)
	queries := sqlc.New(db)

	srv := server.New(cfg.Server, pool, db, queries, logger, Version)

	g, gctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	g.Go(func() error {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
		select {
		case sig := <-quit:
			logger.Info().Str("signal", sig.String()).Msg("shutdown signal received")
		case <-gctx.Done():
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})

	if err := g.Wait(); err != nil {
		logger.Fatal().Err(err).Msg("server exited with error")
	}
}

