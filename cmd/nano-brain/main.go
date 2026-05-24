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
	"github.com/nano-brain/nano-brain/internal/embed"
	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/nano-brain/nano-brain/internal/health"
	"github.com/nano-brain/nano-brain/internal/server"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/sync/errgroup"
)

var Version = "dev"

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file (default: ~/.nano-brain/config.yml)")
	flag.Parse()

	if args := flag.Args(); len(args) > 0 {
		switch args[0] {
		case "collection":
			runCollectionCmd(args[1:])
			return
		case "init":
			runInitCmd(args[1:], configPath)
			return
		case "write":
			runWriteCmd(args[1:])
			return
		case "query":
			runQueryCmd(args[1:])
			return
		case "search":
			runSearchCmd(args[1:])
			return
		case "vsearch":
			runVSearchCmd(args[1:])
			return
		case "harvest":
			runHarvestCmd(args[1:])
			return
		case "bench":
			runBenchCmd(args[1:])
			return
		case "db:migrate":
			runDBMigrateCmd(args[1:])
			return
		case "logs":
			runLogsCmd(args[1:])
			return
		case "docker":
			runDockerCmd(args[1:])
			return
		case "status":
			runStatusCmd(args[1:])
			return
		case "doctor":
			runDoctorCmd(args[1:], configPath)
			return
		}
	}

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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

	fw := watcher.New(db, queries, logger, *cfg)

	var eq *embed.Queue
	var embedder embed.Embedder
	if cfg.Embedding.Provider != "" {
		e, embedErr := embed.NewFromConfig(cfg.Embedding)
		if embedErr != nil {
			logger.Warn().Err(embedErr).Msg("embedding disabled — provider not configured")
		} else {
			embedder = e
			eq = embed.NewQueue(embedder, queries, logger, cfg.Embedding.Provider, cfg.Embedding.Model, cfg.Embedding.Concurrency)
		}
	}

	srv := server.New(cfg, configPath, pool, db, queries, fw, eq, embedder, logger, Version)

	if workspaces, err := queries.ListWorkspaces(ctx); err == nil {
		for _, ws := range workspaces {
			collections, err := queries.ListCollections(ctx, ws.Hash)
			if err != nil {
				logger.Warn().Err(err).Str("workspace", ws.Hash).Msg("failed to list collections for watcher")
				continue
			}
			for _, col := range collections {
				if watchErr := fw.Watch(col.Name, col.Path, col.WorkspaceHash, col.GlobPattern); watchErr != nil {
					logger.Warn().Err(watchErr).Str("collection", col.Name).Msg("failed to watch collection")
				}
			}
		}
	}

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	g.Go(func() error {
		return fw.Run(gctx)
	})

	if eq != nil {
		g.Go(func() error {
			return eq.Run(gctx)
		})
	}

	var hr *harvest.Runner
	interval := time.Duration(cfg.Intervals.SessionPoll) * time.Second

	if cfg.Harvester.OpenCode.SessionDir != "" {
		wsHash, err := storage.WorkspaceHash(cfg.Harvester.OpenCode.SessionDir)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to compute workspace hash for opencode harvester")
		} else {
			oh := harvest.NewOpenCodeHarvester(db, logger, cfg.Harvester.OpenCode.SessionDir, wsHash)
			hr = harvest.NewRunner(oh, eq, interval, logger)
			logger.Info().
				Str("session_dir", cfg.Harvester.OpenCode.SessionDir).
				Dur("interval", interval).
				Msg("opencode session harvester started")
		}
	} else {
		logger.Info().Msg("opencode session harvester disabled (no session_dir configured)")
	}

	if cfg.Harvester.ClaudeCode.Enabled {
		if _, err := os.Stat(cfg.Harvester.ClaudeCode.SessionDir); os.IsNotExist(err) {
			logger.Warn().
				Str("session_dir", cfg.Harvester.ClaudeCode.SessionDir).
				Msg("claude code harvester enabled but session_dir does not exist, skipping")
		} else {
			wsHash, err := storage.WorkspaceHash(cfg.Harvester.ClaudeCode.SessionDir)
			if err != nil {
				logger.Warn().Err(err).Msg("failed to compute workspace hash for claude code harvester")
			} else {
				ch := harvest.NewClaudeCodeHarvester(db, logger, cfg.Harvester.ClaudeCode.SessionDir, wsHash)
				if hr == nil {
					hr = harvest.NewRunner(ch, eq, interval, logger)
				} else {
					hr.AddHarvester(ch)
				}
				logger.Info().
					Str("session_dir", cfg.Harvester.ClaudeCode.SessionDir).
					Dur("interval", interval).
					Msg("claude code session harvester started")
			}
		}
	}

	if hr != nil {
		srv.SetHarvestRunner(hr)
		g.Go(func() error {
			return hr.Run(gctx)
		})
	}

	g.Go(func() error {
		<-gctx.Done()
		logger.Info().Msg("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})

	if err := g.Wait(); err != nil {
		logger.Fatal().Err(err).Msg("server exited with error")
	}
}

