package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nano-brain/nano-brain/internal/chunker"
	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/codesummarize"
	"github.com/nano-brain/nano-brain/internal/flow"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/embed"
	"github.com/nano-brain/nano-brain/internal/eventbus"
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
	flag.BoolVar(&unsafeNoAuth, "unsafe-no-auth", false, "allow binding to non-loopback without auth")
	flag.BoolVar(&serveOnlyFlag, "serve-only", false, "disable background workers (embed queue, watcher, harvester) — issue #282")
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
		case "context":
			runContextCmd(args[1:])
			return
		case "code-impact":
			runCodeImpactCmd(args[1:])
			return
		case "detect-changes":
			runDetectChangesCmd(args[1:])
			return
		case "reset-embeddings":
			runResetEmbeddingsCmd(args[1:])
			return
		case "backfill-summaries":
			runBackfillSummariesCmd(args[1:])
			return
		case "cleanup-stale-raw":
			runCleanupStaleRawCmd(args[1:])
			return
		case "cleanup-orphan-workspaces":
			runCleanupOrphanWorkspacesCmd(args[1:])
			return
		case "wake-up":
			runWakeUpCmd(args[1:])
			return
		case "get":
			runGetCmd(args[1:])
			return
		case "tags":
			runTagsCmd(args[1:])
			return
		case "multi-get":
			runMultiGetCmd(args[1:])
			return
		case "auth":
			runAuthCmd(args[1:])
			return
		case "mcp-url":
			runMCPURLCmd(args[1:])
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
// Logs are written to stderr (not stdout) to avoid polluting --json output.
func initCLILog(configPath string) {
	path := config.ResolveConfigPath(configPath)
	cfg, err := config.Load(path)
	if err != nil {
		return
	}
	applyVerbose(&cfg.Logging)
	
	// Parse log level
	level := zerolog.InfoLevel
	switch strings.ToLower(strings.TrimSpace(cfg.Logging.Level)) {
	case "trace":
		level = zerolog.TraceLevel
	case "debug":
		level = zerolog.DebugLevel
	case "info":
		level = zerolog.InfoLevel
	case "warn":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	}
	
	// CLI logger writes to stderr to avoid polluting stdout (where JSON responses go)
	cliLog = zerolog.New(os.Stderr).
		With().
		Timestamp().
		Logger().
		Level(level)
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

	var configWarning string
	configPath, configWarning = config.ResolveConfigPathStrict(configPath)
	if configWarning != "" {
		fmt.Fprintln(os.Stderr, configWarning)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	applyVerbose(&cfg.Logging)

	if serveOnlyFlag {
		cfg.Server.ServeOnly = true
	}

	if err := checkBindSafety(cfg.Server.Host, cfg.Server.Auth.Enabled); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	logger, err := health.NewLogger(cfg.Logging)
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	cliLog = logger

	if unsafeNoAuth && !isLoopback(cfg.Server.Host) && !cfg.Server.Auth.Enabled {
		logger.Warn().Str("host", cfg.Server.Host).Msg("bound to non-loopback without auth (--unsafe-no-auth set)")
	}

	if cfg.Server.ServeOnly {
		logger.Info().Msg("serve_only mode enabled — embed queue + file watcher + harvester will NOT start (issue #282)")
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

	migrationVersion, err := storage.GetCurrentVersion(ctx, pool)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to get migration version for status")
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
	if rbE, err := symbol.NewRubySymbolExtractor(); err != nil {
		logger.Warn().Err(err).Msg("ruby symbol extractor init failed, skipping")
	} else {
		extractors = append(extractors, rbE)
	}
	symRegistry := symbol.NewRegistry(extractors...)

	var graphExtractors []graph.Extractor
	if goGE, err := graph.NewGoGraphExtractor(); err != nil {
		logger.Warn().Err(err).Msg("go graph extractor init failed, skipping")
	} else {
		graphExtractors = append(graphExtractors, goGE)
	}
	if tsGE, err := graph.NewTypeScriptGraphExtractor(); err != nil {
		logger.Warn().Err(err).Msg("typescript graph extractor init failed, skipping")
	} else {
		graphExtractors = append(graphExtractors, tsGE)
	}
	if jsGE, err := graph.NewJavaScriptGraphExtractor(); err != nil {
		logger.Warn().Err(err).Msg("javascript graph extractor init failed, skipping")
	} else {
		graphExtractors = append(graphExtractors, jsGE)
	}
	if pyGE, err := graph.NewPythonGraphExtractor(); err != nil {
		logger.Warn().Err(err).Msg("python graph extractor init failed, skipping")
	} else {
		graphExtractors = append(graphExtractors, pyGE)
	}
	if cfg.Flow.Enabled {
		if echoGE, err := graph.NewEchoRouteExtractor(); err != nil {
			logger.Warn().Err(err).Msg("echo route extractor init failed, skipping")
		} else {
			graphExtractors = append(graphExtractors, echoGE)
			logger.Info().Msg("execution-flow: echo route extractor enabled")
		}
		if nethttpGE, err := graph.NewNetHTTPExtractor(); err != nil {
			logger.Warn().Err(err).Msg("net/http route extractor init failed, skipping")
		} else {
			graphExtractors = append(graphExtractors, nethttpGE)
			logger.Info().Msg("execution-flow: net/http route extractor enabled")
		}
		if ginGE, err := graph.NewGinExtractor(); err != nil {
			logger.Warn().Err(err).Msg("gin route extractor init failed, skipping")
		} else {
			graphExtractors = append(graphExtractors, ginGE)
			logger.Info().Msg("execution-flow: gin route extractor enabled")
		}
		if intGE, err := graph.NewIntegrationExtractor(); err != nil {
			logger.Warn().Err(err).Msg("integration extractor init failed, skipping")
		} else {
			graphExtractors = append(graphExtractors, intGE)
			logger.Info().Msg("execution-flow: integration extractor enabled")
		}
		if jsIntGE, err := graph.NewJSIntegrationExtractor(); err != nil {
			logger.Warn().Err(err).Msg("js integration extractor init failed, skipping")
		} else {
			graphExtractors = append(graphExtractors, jsIntGE)
			logger.Info().Msg("execution-flow: js integration extractor enabled")
		}
		if pyIntGE, err := graph.NewPythonIntegrationExtractor(); err != nil {
			logger.Warn().Err(err).Msg("python integration extractor init failed, skipping")
		} else {
			graphExtractors = append(graphExtractors, pyIntGE)
			logger.Info().Msg("execution-flow: python integration extractor enabled")
		}
		if expressGE, err := graph.NewExpressExtractor(logger); err != nil {
			logger.Warn().Err(err).Msg("express route extractor init failed, skipping")
		} else {
			graphExtractors = append(graphExtractors, expressGE)
			logger.Info().Msg("execution-flow: express route extractor enabled")
		}
		if nestjsGE, err := graph.NewNestJSExtractor(logger); err != nil {
			logger.Warn().Err(err).Msg("nestjs route extractor init failed, skipping")
		} else {
			graphExtractors = append(graphExtractors, nestjsGE)
			logger.Info().Msg("execution-flow: nestjs route extractor enabled")
		}
		if nuxtGE, err := graph.NewNuxtExtractor(logger); err != nil {
			logger.Warn().Err(err).Msg("nuxt route extractor init failed, skipping")
		} else {
			graphExtractors = append(graphExtractors, nuxtGE)
			logger.Info().Msg("execution-flow: nuxt route extractor enabled")
		}
		if vueGE, err := graph.NewVueSFCExtractor(); err != nil {
			logger.Warn().Err(err).Msg("vue sfc extractor init failed, skipping")
		} else {
			graphExtractors = append(graphExtractors, vueGE)
			logger.Info().Msg("execution-flow: vue sfc extractor enabled")
		}
		if railsGE, err := graph.NewRailsExtractor(logger); err != nil {
			logger.Warn().Err(err).Msg("rails route extractor init failed, skipping")
		} else {
			graphExtractors = append(graphExtractors, railsGE)
			logger.Info().Msg("execution-flow: rails route extractor enabled")
		}
		if rbGraphGE, err := graph.NewRubyGraphExtractor(); err != nil {
			logger.Warn().Err(err).Msg("ruby graph extractor init failed, skipping")
		} else {
			graphExtractors = append(graphExtractors, rbGraphGE)
			logger.Info().Msg("execution-flow: ruby graph extractor enabled")
		}
		if railsDSLEdge, err := graph.NewRailsDSLEdgeExtractor(); err != nil {
			logger.Warn().Err(err).Msg("rails dsl edge extractor init failed, skipping")
		} else {
			graphExtractors = append(graphExtractors, railsDSLEdge)
			logger.Info().Msg("execution-flow: rails dsl edge extractor enabled")
		}
	}
	graphRegistry := graph.NewRegistry(graphExtractors...)

	if cfg.Flow.Enabled {
		if jsCFG, err := graph.NewJSControlFlowExtractor(); err != nil {
			logger.Warn().Err(err).Msg("js control-flow extractor init failed, skipping")
		} else {
			graphRegistry.RegisterControlFlowExtractor(jsCFG)
			logger.Info().Msg("execution-flow: js control-flow extractor enabled")
		}
		if rbCFG, err := graph.NewRubyControlFlowExtractor(logger); err != nil {
			logger.Warn().Err(err).Msg("ruby control-flow extractor init failed, skipping")
		} else {
			graphRegistry.RegisterControlFlowExtractor(rbCFG)
			logger.Info().Msg("execution-flow: ruby control-flow extractor enabled")
		}
	}

	var frameworkDetector *graph.FrameworkDetector
	if cfg.Flow.Enabled {
		frameworkDetector = graph.NewFrameworkDetector(graph.DefaultRules)
	}

	fixedChunker := chunker.NewFixedChunker()
	headingChunker := chunker.NewHeadingChunker()
	symbolChunker, scErr := chunker.NewSymbolAwareChunker(fixedChunker, logger)
	if scErr != nil {
		logger.Error().Err(scErr).Msg("symbol-aware chunker init failed, using fixed chunker for all files")
	}
	var dispatcher *chunker.Dispatcher
	if symbolChunker != nil {
		dispatcher = chunker.NewDispatcher(symbolChunker, headingChunker, fixedChunker)
	} else {
		dispatcher = chunker.NewDispatcher(fixedChunker, headingChunker, fixedChunker)
	}

	fw := watcher.New(db, queries, logger, *cfg).
		WithSymbolRegistry(symRegistry).
		WithGraphRegistry(graphRegistry, queries).
		WithFrameworkDetector(frameworkDetector).
		WithDispatcher(dispatcher)

	if homeDir, hErr := os.UserHomeDir(); hErr == nil {
		if gi, path, lErr := watcher.LoadGlobalIgnore(homeDir); lErr != nil {
			logger.Warn().Err(lErr).Str("path", path).Msg("global .nano-brainignore failed to load; using empty matcher")
		} else if gi != nil {
			logger.Info().Str("path", path).Msg("loaded global .nano-brainignore")
			fw.SetGlobalIgnore(gi)
		} else {
			logger.Debug().Str("path", path).Msg(".nano-brainignore not found, skipping (issue #263)")
		}
	}

	var eq *embed.Queue
	var embedder embed.Embedder
	if cfg.Embedding.Provider != "" {
		e, embedErr := embed.NewFromConfig(cfg.Embedding)
		if embedErr != nil {
			logger.Error().Err(embedErr).Str("provider", cfg.Embedding.Provider).Str("url", cfg.Embedding.URL).Msg("embedding provider init failed")
		} else {
			embedder = e
			if cfg.Server.ServeOnly {
				logger.Info().Msg("serve_only mode — embedder constructed but queue skipped (write handler will be no-op for enqueue)")
			} else {
				eq = embed.NewQueue(embedder, queries, logger, cfg.Embedding.Provider, cfg.Embedding.Model, cfg.Embedding.Concurrency).
					WithMaxChars(cfg.Embedding.MaxChars)
				fw.WithEmbedQueue(eq)
			}
		}
	}

	bus := eventbus.New(ctx)

	if eq != nil {
		eq.WithPublisher(bus)
	}
	fw.WithPublisher(bus)

	srv := server.New(cfg, configPath, pool, db, queries, fw, eq, embedder, bus, logger, Version, migrationVersion, graphRegistry)

	if cfg.Server.ServeOnly {
		logger.Info().Msg("serve_only mode — skipping collection watcher registration")
	} else if workspaces, err := queries.ListWorkspaces(ctx); err == nil {
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

	if cfg.Server.ServeOnly {
		logger.Info().Msg("serve_only mode — file watcher goroutine disabled")
	} else {
		g.Go(func() error {
			return fw.Run(gctx)
		})
	}

	if cfg.Server.ServeOnly {
		logger.Info().Msg("serve_only mode — embed queue worker disabled")
	} else if eq != nil {
		g.Go(func() error {
			return eq.Run(gctx)
		})
	}

	if cfg.Server.ServeOnly {
		logger.Info().Msg("serve_only mode — harvester + summarizer disabled")
	} else {
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

		if cfg.Harvester.OpenCode.DBRoot == "" {
			if detected := detectOpenCodeDBRoot(); detected != "" {
				cfg.Harvester.OpenCode.DBRoot = detected
				logger.Info().Str("path", detected).Msg("auto-detected opencode db root")
			}
		}

		var harvestSummarizer *summarize.HarvestSummarizer
		if cfg.Summarization.Enabled {
			harvestSummarizer = buildHarvestSummarizer(cfg, db, eq, logger)
			if harvestSummarizer == nil {
				logger.Warn().Msg("summarization enabled but init failed — falling back to raw harvest")
			} else {
				logger.Info().Str("model", cfg.Summarization.Model).Msg("session summarization enabled")
			}
		}

		ocHarvesters, ocMode := buildOpenCodeHarvesters(ctx, cfg, db, queries, logger)
		for i, oh := range ocHarvesters {
			if i == 0 {
				hr = harvest.NewRunner(oh, eq, interval, logger)
			} else {
				hr.AddHarvester(oh)
			}
		}
		srv.SetHarvestStatus(ocMode, cfg.Harvester.OpenCode.DBRoot, cfg.Harvester.OpenCode.DBPath, cfg.Harvester.OpenCode.SessionDir, len(ocHarvesters))

		if ch, ccErr := initClaudeCodeHarvester(ctx, cfg.Harvester.ClaudeCode, db, logger); ccErr != nil {
			logger.Error().Err(ccErr).Msg("claude code harvester init failed")
		} else if ch != nil {
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

		if hr != nil {
			hr.WithPublisher(bus)
			if harvestSummarizer != nil {
				hr.WithSummarizer(harvestSummarizer)
				srv.SetSummarizer(harvestSummarizer)
				if res, ext := srv.LinkDeps(); res != nil && ext != nil {
					harvestSummarizer.SetLinkExtractor(res, ext)
				}
			}
			srv.SetHarvestRunner(hr)
			g.Go(func() error {
				return hr.Run(gctx)
			})
		}
	}

	// Wire code summarization service (if enabled)
	if cfg.CodeSummarization.Enabled && cfg.CodeSummarization.ProviderURL != "" {
		csProvider := codesummarize.NewLLMProvider(cfg.CodeSummarization, logger)
		csBudget := codesummarize.NewBudgetTracker(queries)
		csSvc := codesummarize.NewService(cfg.CodeSummarization, csProvider, csBudget, queries, eq, logger).
			WithWorkspaceLister(queries)
		srv.SetCodeSummarizer(csSvc)
		fw.WithSummarizeNotify(csSvc.Notify)
		if !cfg.Server.ServeOnly {
			g.Go(func() error {
				csSvc.StartWorkerPool(gctx)
				return nil
			})
		}
		logger.Info().Str("model", cfg.CodeSummarization.Model).Msg("code summarization service configured")
	}

	// Wire flow materializer (if enabled).
	if cfg.Flow.Enabled {
		var enqueueFn func(uuid.UUID)
		if eq != nil {
			enqueueFn = func(id uuid.UUID) { eq.Enqueue(id) }
		}

		var flowSummarizer flow.FlowSummarizer
		if cfg.Flow.SummaryEnabled {
			llmClient := summarize.New(cfg.Summarization, logger)
			flowSummarizer = flow.NewLLMFlowSummarizer(llmClient, logger)
		}

		summaryTimeout := time.Duration(cfg.Flow.SummaryTimeout) * time.Second
		if summaryTimeout <= 0 {
			summaryTimeout = 10 * time.Minute
		}
		mat := flow.NewMaterializer(queries, enqueueFn, cfg.Flow.MaxDepth, cfg.Flow.MaxFanout, summaryTimeout, flowSummarizer, logger)
		srv.SetFlowMaterializer(mat)
		fw.WithFlowNotify(func(wsHash string) {
			go mat.Trigger(gctx, wsHash)
		})
		logger.Info().Int("max_depth", cfg.Flow.MaxDepth).Int("max_fanout", cfg.Flow.MaxFanout).Bool("summary_enabled", cfg.Flow.SummaryEnabled).Msg("flow materialization enabled")
	}

	g.Go(func() error {
		<-gctx.Done()
		logger.Info().Msg("shutdown signal received")
		bus.Close()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})

	if err := g.Wait(); err != nil {
		logger.Fatal().Err(err).Msg("server exited with error")
	}
}

// buildOpenCodeHarvesters constructs OpenCode harvesters in priority order:
//
//  1. db_root — scan for per-project SQLite DBs matching registered workspaces
//  2. db_path — single SQLite DB (existing behavior)
//  3. session_dir — legacy JSON session harvester
//
// Returns a slice of harvesters (possibly empty). The caller wires the first
// into harvest.NewRunner and the rest via hr.AddHarvester.
//
// TODO: live rescan on tick — see docs/HARNESS_BACKLOG.md
func buildOpenCodeHarvesters(ctx context.Context, cfg *config.Config, db *sql.DB, queries *sqlc.Queries, logger zerolog.Logger) ([]harvest.Harvester, string) {
	dbRootExplicit := os.Getenv("OPENCODE_DB_ROOT") != "" || cfg.Harvester.OpenCode.DBRoot != ""

	if cfg.Harvester.OpenCode.DBRoot != "" {
		workspaces, err := queries.ListWorkspaces(ctx)
		if err != nil {
			logger.Warn().Err(err).Msg("db_root mode: failed to list workspaces, falling through")
		} else {
			registered := make(map[string]string, len(workspaces))
			for _, ws := range workspaces {
				registered[ws.Path] = ws.Hash
			}
			discovered := harvest.ScanOpenCodeDBRoot(ctx, cfg.Harvester.OpenCode.DBRoot, registered, logger)
			if len(discovered) > 0 {
				worktreeCounts := make(map[string]int, len(discovered))
				for _, d := range discovered {
					worktreeCounts[d.Worktree]++
				}
				var harvesters []harvest.Harvester
				for _, d := range discovered {
					h := harvest.NewOpenCodeSQLiteHarvester(db, logger, d.DBPath)
					harvesters = append(harvesters, h)
					logger.Info().
						Str("db_path", d.DBPath).
						Str("worktree", d.Worktree).
						Str("workspace_hash", d.WorkspaceHash).
						Msg("opencode per-project db harvester registered")
				}
				for worktree, n := range worktreeCounts {
					if n > 1 {
						logger.Info().Str("worktree", worktree).Int("db_count", n).
							Msg("multiple per-project DBs map to the same worktree — content-hash dedup will collapse duplicates")
					}
				}
				return harvesters, "db_root"
			}
			if dbRootExplicit {
				logger.Warn().Str("db_root", cfg.Harvester.OpenCode.DBRoot).
					Msg("db_root configured but no per-project DBs matched registered workspaces — falling through")
			} else {
				logger.Info().Str("db_root", cfg.Harvester.OpenCode.DBRoot).
					Msg("auto-detected db_root produced no matches — falling through")
			}
		}
	}

	if cfg.Harvester.OpenCode.DBPath != "" {
		oh := harvest.NewOpenCodeSQLiteHarvester(db, logger, cfg.Harvester.OpenCode.DBPath)
		logger.Info().
			Str("db_path", cfg.Harvester.OpenCode.DBPath).
			Msg("opencode sqlite harvester started")
		return []harvest.Harvester{oh}, "db_path"
	}

	if cfg.Harvester.OpenCode.SessionDir != "" {
		oh, err := initOpenCodeFileHarvester(ctx, cfg.Harvester.OpenCode, db, logger)
		if err != nil || oh == nil {
			return nil, "disabled"
		}
		logger.Info().
			Str("session_dir", cfg.Harvester.OpenCode.SessionDir).
			Msg("opencode session harvester started")
		return []harvest.Harvester{oh}, "session_dir"
	}

	logger.Info().Msg("opencode harvester disabled (no db_root, db_path, or session_dir configured)")
	return nil, "disabled"
}

// buildHarvestSummarizer constructs the harvest summarizer with graceful degradation.
// Returns nil + logs warn if any init step fails or panics. This prevents a misconfigured
// summarization block from crashing the server — harvest falls back to raw storage instead.
func buildHarvestSummarizer(cfg *config.Config, db *sql.DB, eq *embed.Queue, logger zerolog.Logger) (result *summarize.HarvestSummarizer) {
	defer func() {
		if r := recover(); r != nil {
			logger.Warn().Interface("panic", r).Msg("summarizer init panicked, disabling summarization")
			result = nil
		}
	}()
	llmClient := summarize.New(cfg.Summarization, logger)
	if llmClient == nil {
		logger.Warn().Msg("LLM client init returned nil, disabling summarization")
		return nil
	}
	pipeline := summarize.NewPipeline(llmClient, nil, cfg.Summarization.Concurrency, logger)
	if pipeline == nil {
		logger.Warn().Msg("pipeline init returned nil, disabling summarization")
		return nil
	}
	// Avoid nil-interface trap: a nil *embed.Queue stored in a PersisterEnqueuer
	// interface produces a non-nil interface with nil dynamic value, bypassing
	// the `p.enqueuer != nil` check in Persister.Save and panicking on Enqueue.
	var enqueuer summarize.PersisterEnqueuer
	if eq != nil {
		enqueuer = eq
	}
	persister := summarize.NewPersister(
		db,
		enqueuer,
		cfg.Summarization.IsWriteToDiskEnabled(),
		cfg.Summarization.OutputDir,
		logger,
	)
	if persister == nil {
		logger.Warn().Msg("persister init returned nil, disabling summarization")
		return nil
	}
	return summarize.NewHarvestSummarizer(pipeline, persister, logger)
}

