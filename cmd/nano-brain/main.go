package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/embed"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/nano-brain/nano-brain/internal/health"
	"github.com/nano-brain/nano-brain/internal/server"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/summarize"
	"github.com/nano-brain/nano-brain/internal/symbol"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

var Version = "dev"

// cliLog is the process-wide logger for CLI command paths.
// Initialized in main() once flags are parsed; remains a no-op logger
// for commands that run before initialization or when logger setup fails.
var cliLog = zerolog.Nop()

// verbose holds the -v/--verbose count: 0=info, 1=debug, 2+=trace.
var verbose int

func main() {
	var configPath string
	var daemonChild bool
	flag.StringVar(&configPath, "config", "", "path to config file (default: ~/.nano-brain/config.yml)")
	flag.BoolVar(&daemonChild, "daemon-child", false, "")
	flag.IntVar(&verbose, "v", 0, "verbosity: 0=info, 1=debug, 2=trace")
	flag.IntVar(&verbose, "verbose", 0, "")
	flag.Parse()

	initCLILog(configPath)

	// Hidden --daemon-child flag: run server directly (called by serve -d)
	if daemonChild {
		defer os.Remove(pidFilePath())
		startServer(configPath)
		return
	}

	if args := flag.Args(); len(args) > 0 {
		switch args[0] {
		case "serve":
			runServeCmd(args[1:], configPath)
			return
		case "stop":
			runStopCmd()
			return
		case "restart":
			runRestartCmd(args[1:], configPath)
			return
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
		case "workspaces":
			runWorkspacesCmd(args[1:])
			return
		case "harvest":
			runHarvestCmd(args[1:])
			return
		case "reindex":
			runReindexCmd(args[1:])
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
		case "config":
			runConfigCmd(args[1:], configPath)
			return
		case "doctor":
			runDoctorCmd(args[1:], configPath)
			return
		case "version":
			runVersionCmd(args[1:])
			return
		case "help":
			printUsage()
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", args[0])
			printUsage()
			os.Exit(1)
		}
	}

	// No args: start server foreground (backward compat)
	startServer(configPath)
}

// initCLILog builds a best-effort logger for CLI commands before they dispatch.
// Failures are non-fatal: cliLog falls back to zerolog.Nop().
func initCLILog(configPath string) {
	path := configPath
	if path == "" {
		path = config.DefaultConfigPath()
	}
	cfg, err := config.Load(path)
	if err != nil {
		return
	}
	applyVerbose(&cfg.Logging)
	logger, err := health.NewLogger(cfg.Logging)
	if err != nil {
		return
	}
	cliLog = logger
}

// applyVerbose maps the -v/--verbose flag count to a log level string.
// Counts above the explicit cases clamp to "trace".
func applyVerbose(cfg *config.LoggingConfig) {
	switch {
	case verbose >= 2:
		cfg.Level = "trace"
	case verbose == 1:
		cfg.Level = "debug"
	}
}

// startServer runs the nano-brain HTTP server (blocking).
func startServer(configPath string) {
	if err := guardBeforeStart(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	if configPath == "" {
		configPath = config.DefaultConfigPath()
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	applyVerbose(&cfg.Logging)

	logger, err := health.NewLogger(cfg.Logging)
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	cliLog = logger

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

	var extractors []symbol.Extractor
	if goE, err := symbol.NewGoExtractor(); err != nil {
		logger.Warn().Err(err).Msg("go symbol extractor init failed, skipping")
	} else {
		extractors = append(extractors, goE)
	}
	if tsE, err := symbol.NewTypeScriptExtractor(); err != nil {
		logger.Warn().Err(err).Msg("typescript symbol extractor init failed, skipping")
	} else {
		extractors = append(extractors, tsE)
	}
	if pyE, err := symbol.NewPythonExtractor(); err != nil {
		logger.Warn().Err(err).Msg("python symbol extractor init failed, skipping")
	} else {
		extractors = append(extractors, pyE)
	}
	if jsE, err := symbol.NewJavaScriptExtractor(); err != nil {
		logger.Warn().Err(err).Msg("javascript symbol extractor init failed, skipping")
	} else {
		extractors = append(extractors, jsE)
	}
	symRegistry := symbol.NewRegistry(extractors...)

	var graphExtractors []graph.Extractor
	if goGE, err := graph.NewGoGraphExtractor(); err != nil {
		logger.Warn().Err(err).Msg("go graph extractor init failed, skipping")
	} else {
		graphExtractors = append(graphExtractors, goGE)
	}
	graphRegistry := graph.NewRegistry(graphExtractors...)

	fw := watcher.New(db, queries, logger, *cfg).
		WithSymbolRegistry(symRegistry).
		WithGraphRegistry(graphRegistry, queries)

	var eq *embed.Queue
	var embedder embed.Embedder
	if cfg.Embedding.Provider != "" {
		e, embedErr := embed.NewFromConfig(cfg.Embedding)
		if embedErr != nil {
			logger.Error().Err(embedErr).Str("provider", cfg.Embedding.Provider).Str("url", cfg.Embedding.URL).Msg("embedding provider init failed")
		} else {
			embedder = e
			eq = embed.NewQueue(embedder, queries, logger, cfg.Embedding.Provider, cfg.Embedding.Model, cfg.Embedding.Concurrency)
			fw.WithEmbedQueue(eq)
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
			cfgExclude, cfgExtensions := cfg.Watcher.ResolveFilter(ws.Path)
			for _, col := range collections {
				if _, statErr := os.Stat(col.Path); os.IsNotExist(statErr) {
					logger.Debug().Str("collection", col.Name).Str("path", col.Path).Msg("skipping watch — path does not exist")
					continue
				}
				excludePatterns := append(cfgExclude, col.ExcludePatterns...)
				allowedExtensions := col.AllowedExtensions
				if len(allowedExtensions) == 0 {
					allowedExtensions = cfgExtensions
				}
				if watchErr := fw.WatchWithFilter(col.Name, col.Path, col.WorkspaceHash, col.GlobPattern, excludePatterns, allowedExtensions); watchErr != nil {
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

	if cfg.Harvester.OpenCode.SessionDir == "" {
		if detected := detectOpenCodeStorageDir(); detected != "" {
			cfg.Harvester.OpenCode.SessionDir = detected
			logger.Info().Str("path", detected).Msg("auto-detected opencode storage dir")
		}
	}

	if cfg.Harvester.OpenCode.DBPath == "" {
		if detected := detectOpenCodeDBPath(); detected != "" {
			cfg.Harvester.OpenCode.DBPath = detected
			logger.Info().Str("path", detected).Msg("auto-detected opencode sqlite db")
		}
	}

	if cfg.Harvester.OpenCode.DBPath != "" {
		wsHash, err := storage.WorkspaceHash(cfg.Harvester.OpenCode.DBPath)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to compute workspace hash for opencode sqlite harvester")
		} else {
			oh := harvest.NewOpenCodeSQLiteHarvester(db, logger, cfg.Harvester.OpenCode.DBPath, wsHash)
			hr = harvest.NewRunner(oh, eq, interval, logger)
			logger.Info().
				Str("db_path", cfg.Harvester.OpenCode.DBPath).
				Dur("interval", interval).
				Msg("opencode sqlite harvester started")
		}
	} else if cfg.Harvester.OpenCode.SessionDir != "" {
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
		logger.Info().Msg("opencode harvester disabled (no db_path or session_dir configured)")
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
		if cfg.Summarization.Enabled {
			llmClient := summarize.New(cfg.Summarization, logger)
			pipeline := summarize.NewPipeline(llmClient, nil, cfg.Summarization.Concurrency, logger)
			persister := summarize.NewPersister(db, cfg.Summarization.OutputDir, "", eq, logger)
			adapter := summarize.NewHarvestSummarizer(pipeline, persister, logger)
			hr.WithSummarizer(adapter)
			logger.Info().
				Str("output_dir", cfg.Summarization.OutputDir).
				Str("model", cfg.Summarization.Model).
				Msg("session summarization enabled")
		}
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

