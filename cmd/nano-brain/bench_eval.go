package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/evalbench"
)

const defaultProfilesPath = ".omc/bench-eval.local.json"
const defaultReplayGlob = "~/.nano-brain-observe/logs/calls-*.jsonl"

func runBenchEval(args []string, configPath string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if code := executeBenchEval(ctx, args, configPath, os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func printBenchEvalHelp(w io.Writer) {
	help := `Usage: nano-brain bench eval [flags]

Run the 100% read-only multi-workspace capability, ranking, latency, token-cost (#529),
and observe-traffic replay evaluation suite. Zero mutations guaranteed via ReadOnlyTransport
and pre/post workspace snapshot guards.

Flags:
  --url <url>              Target nano-brain URL (default: http://$NANO_BRAIN_HOST:$NANO_BRAIN_PORT or http://localhost:3100)
  --transport <mcp|rest>   Transport protocol for capability suites (default: mcp)
  --suite <list>           Comma-separated suites: all,go,ts,vue,rails,replay (default: all)
  --profiles <file>        Optional JSON mapping suite -> {workspace, dataset, vars} (default: .omc/bench-eval.local.json if present)
  --workspace <id>         Override workspace hash/name when running a single suite
  --dataset <file>         Override dataset JSON path when running a single suite
  --var <KEY=VAL>          Template variable substitution (repeatable, e.g. --var PROJECT=app)
  --replay-logs <glob>     Glob/path to nano-brain-observe JSONL logs (default: ~/.nano-brain-observe/logs/calls-*.jsonl)
  --replay-limit <N>       Max read-only calls to replay (default: 200, 0 = unlimited)
  --strict-immutable       Fail if evaluated workspace doc/chunk counts change during run (default: true)
  --verbose-private        Include unredacted replay call arguments in report details (default: false)
  --save <file>            Write JSON report to file
  --markdown <file>        Write Markdown scorecard to file
  --compare <baseline>     Compare current run against baseline JSON report (exit 1 on regression)
  --json                   Print JSON report to stdout instead of Markdown summary
  -h, --help               Show this help
`
	fmt.Fprint(w, help)
}

type benchEvalOptions struct {
	serverURL       string
	transport       string
	suites          []string
	profilesPath    string
	profilesSet     bool
	workspace       string
	datasetPath     string
	vars            map[string]string
	replayLogs      []string
	replayLimit     int
	strictImmutable bool
	verbosePrivate  bool
	savePath        string
	markdownPath    string
	comparePath     string
	jsonOutput      bool
	showHelp        bool
}

func parseBenchEvalArgs(rawArgs []string) (benchEvalOptions, error) {
	args := splitEqualsArgs(rawArgs)
	opts := benchEvalOptions{
		transport:       evalbench.TransportMCP,
		suites:          []string{"all"},
		vars:            make(map[string]string),
		replayLimit:     200,
		strictImmutable: true,
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			opts.showHelp = true
			return opts, nil
		case "--url":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--url requires a value")
			}
			i++
			opts.serverURL = strings.TrimSpace(args[i])
		case "--transport":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--transport requires a value (mcp|rest)")
			}
			i++
			t := strings.ToLower(strings.TrimSpace(args[i]))
			if t != evalbench.TransportMCP && t != evalbench.TransportREST {
				return opts, fmt.Errorf("--transport must be 'mcp' or 'rest', got %q", args[i])
			}
			opts.transport = t
		case "--suite":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--suite requires a value")
			}
			i++
			parts := strings.Split(args[i], ",")
			opts.suites = opts.suites[:0]
			for _, p := range parts {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					opts.suites = append(opts.suites, trimmed)
				}
			}
			if len(opts.suites) == 0 {
				return opts, fmt.Errorf("--suite must list at least one suite")
			}
		case "--profiles":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--profiles requires a file path")
			}
			i++
			opts.profilesPath = strings.TrimSpace(args[i])
			opts.profilesSet = true
		case "--workspace":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--workspace requires a value")
			}
			i++
			opts.workspace = strings.TrimSpace(args[i])
		case "--dataset":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--dataset requires a file path")
			}
			i++
			opts.datasetPath = strings.TrimSpace(args[i])
		case "--var":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--var requires KEY=VAL")
			}
			i++
			k, v, ok := strings.Cut(args[i], "=")
			k = strings.TrimSpace(k)
			if !ok || k == "" {
				return opts, fmt.Errorf("--var expects KEY=VAL, got %q", args[i])
			}
			opts.vars[k] = v
		case "--replay-logs":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--replay-logs requires a glob or file path")
			}
			i++
			for _, part := range strings.Split(args[i], ",") {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					opts.replayLogs = append(opts.replayLogs, trimmed)
				}
			}
		case "--replay-limit":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--replay-limit requires a non-negative integer")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return opts, fmt.Errorf("--replay-limit must be >= 0, got %q", args[i])
			}
			opts.replayLimit = n
		case "--strict-immutable":
			if i+1 < len(args) {
				if b, err := strconv.ParseBool(args[i+1]); err == nil && !strings.HasPrefix(args[i+1], "-") {
					opts.strictImmutable = b
					i++
					continue
				}
			}
			opts.strictImmutable = true
		case "--no-strict-immutable":
			opts.strictImmutable = false
		case "--verbose-private":
			if i+1 < len(args) {
				if b, err := strconv.ParseBool(args[i+1]); err == nil && !strings.HasPrefix(args[i+1], "-") {
					opts.verbosePrivate = b
					i++
					continue
				}
			}
			opts.verbosePrivate = true
		case "--save":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--save requires a file path")
			}
			i++
			opts.savePath = strings.TrimSpace(args[i])
		case "--markdown":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--markdown requires a file path")
			}
			i++
			opts.markdownPath = strings.TrimSpace(args[i])
		case "--compare":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--compare requires a baseline JSON file path")
			}
			i++
			opts.comparePath = strings.TrimSpace(args[i])
		case "--json":
			opts.jsonOutput = true
		default:
			return opts, fmt.Errorf("unknown flag: %s", args[i])
		}
	}

	if len(opts.replayLogs) == 0 {
		opts.replayLogs = []string{defaultReplayGlob}
	}
	return opts, nil
}

func executeBenchEval(ctx context.Context, rawArgs []string, configPath string, stdout, stderr io.Writer) int {
	opts, err := parseBenchEvalArgs(rawArgs)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		printBenchEvalHelp(stderr)
		return 1
	}
	if opts.showHelp {
		printBenchEvalHelp(stdout)
		return 0
	}

	resolvedCfgPath, _ := resolveCLIConfigPath(configPath)

	targetURL := opts.serverURL
	if targetURL == "" {
		targetURL = resolveEvalBaseURL(resolvedCfgPath)
	}
	targetURL = strings.TrimRight(targetURL, "/")

	profiles, err := loadEvalProfiles(opts.profilesPath, opts.profilesSet)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	capSuites, runReplay, err := expandEvalSuites(opts.suites, profiles)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if (opts.workspace != "" || opts.datasetPath != "") && len(capSuites) > 1 {
		fmt.Fprintln(stderr, "Error: --workspace and --dataset overrides apply to a single capability suite; specify --suite <name> or use --profiles")
		return 1
	}

	roHTTP := evalbench.NewReadOnlyHTTPClient(60*time.Second, nil)
	evalClient := evalbench.NewEvalClient(targetURL, roHTTP)

	beforeSnap, err := evalbench.CaptureWorkspaceSnapshot(ctx, evalClient.HTTPClient(), targetURL)
	if err != nil {
		fmt.Fprintf(stderr, "Error capturing pre-run workspace snapshot from %s: %v\n", targetURL, err)
		return 1
	}

	var suiteResults []evalbench.SuiteResult
	var targetHashes []string

	for idx, suiteName := range capSuites {
		prof := profilesForSuite(profiles, suiteName)
		dsPath := evalbench.DefaultSuiteDatasets[suiteName]
		if prof.Dataset != "" {
			dsPath = prof.Dataset
		}
		if opts.datasetPath != "" {
			dsPath = opts.datasetPath
		}
		wsSpec := prof.Workspace
		if opts.workspace != "" {
			wsSpec = opts.workspace
		}
		mergedVars := mergeSuiteVars(prof.Vars, opts.vars)

		sr, resolvedHash, runErr := evalbench.RunSuite(ctx, evalClient, evalbench.SuiteRunConfig{
			SuiteName:     suiteName,
			DatasetPath:   dsPath,
			Transport:     opts.transport,
			WorkspaceSpec: wsSpec,
			WorkspaceSlot: fmt.Sprintf("suite-ws-%d", idx+1),
			Vars:          mergedVars,
		}, beforeSnap)
		if runErr != nil {
			fmt.Fprintf(stderr, "Error running suite %s: %v\n", suiteName, runErr)
			return 1
		}
		suiteResults = append(suiteResults, sr)
		if resolvedHash != "" {
			targetHashes = append(targetHashes, resolvedHash)
		}
	}

	var replayPtr *evalbench.ReplaySummary
	if runReplay {
		repSum, replayHashes, repErr := evalbench.RunReplay(ctx, evalClient, evalbench.ReplayOptions{
			LogPaths:       opts.replayLogs,
			Limit:          opts.replayLimit,
			VerbosePrivate: opts.verbosePrivate,
		}, beforeSnap)
		if repErr != nil {
			fmt.Fprintf(stderr, "Error running observe replay: %v\n", repErr)
			return 1
		}
		replayPtr = &repSum
		targetHashes = append(targetHashes, replayHashes...)
	}

	afterSnap, err := evalbench.CaptureWorkspaceSnapshot(ctx, evalClient.HTTPClient(), targetURL)
	if err != nil {
		fmt.Fprintf(stderr, "Error capturing post-run workspace snapshot from %s: %v\n", targetURL, err)
		return 1
	}

	diff := evalbench.DiffSnapshots(beforeSnap, afterSnap, targetHashes)
	report := evalbench.BuildEvalReport(targetURL, opts.transport, opts.strictImmutable, suiteResults, replayPtr, diff)

	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "Error marshaling report JSON: %v\n", err)
		return 1
	}
	mdText := evalbench.FormatMarkdown(report)

	if opts.savePath != "" {
		if err := writeReportFile(opts.savePath, append(jsonBytes, '\n')); err != nil {
			fmt.Fprintf(stderr, "Error writing JSON report to %s: %v\n", opts.savePath, err)
			return 1
		}
	}
	if opts.markdownPath != "" {
		if err := writeReportFile(opts.markdownPath, []byte(mdText)); err != nil {
			fmt.Fprintf(stderr, "Error writing Markdown report to %s: %v\n", opts.markdownPath, err)
			return 1
		}
	}

	if opts.jsonOutput {
		fmt.Fprintln(stdout, string(jsonBytes))
	} else {
		fmt.Fprint(stdout, mdText)
	}

	exitCode := 0
	if opts.strictImmutable && diff.Mutated {
		fmt.Fprintf(stderr, "IMMUTABILITY CHECK FAILED: %s\n", diff.Note)
		exitCode = 1
	}

	if opts.comparePath != "" {
		baseRaw, readErr := os.ReadFile(opts.comparePath)
		if readErr != nil {
			fmt.Fprintf(stderr, "Error reading baseline report %s: %v\n", opts.comparePath, readErr)
			return 1
		}
		var baseReport evalbench.EvalReport
		if unmarshalErr := json.Unmarshal(baseRaw, &baseReport); unmarshalErr != nil {
			fmt.Fprintf(stderr, "Error parsing baseline report %s: %v\n", opts.comparePath, unmarshalErr)
			return 1
		}
		cmp := evalbench.CompareEvalReports(baseReport, report, evalbench.DefaultCompareThresholds())
		if !cmp.Passed {
			for _, reg := range cmp.Regressions {
				fmt.Fprintf(stderr, "REGRESSION [%s]: %s\n", reg.Metric, reg.Message)
			}
			exitCode = 1
		}
	}

	return exitCode
}

func resolveEvalBaseURL(resolvedConfigPath string) string {
	envHost := os.Getenv("NANO_BRAIN_HOST")
	envPort := os.Getenv("NANO_BRAIN_PORT")
	cfg, _ := config.Load(resolvedConfigPath)

	host := envHost
	if host == "" {
		if isContainerFn() {
			host = "host.docker.internal"
		} else if cfg != nil && cfg.Server.Host != "" {
			host = dialHost(cfg.Server.Host)
		} else {
			host = "localhost"
		}
	}

	port := 3100
	if envPort != "" {
		if v, err := strconv.Atoi(envPort); err == nil && v > 0 && v <= 65535 {
			port = v
		}
	} else if cfg != nil && cfg.Server.Port > 0 {
		port = cfg.Server.Port
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}

func loadEvalProfiles(profilesPath string, explicit bool) (evalbench.ProfilesConfig, error) {
	p := profilesPath
	if !explicit {
		p = defaultProfilesPath
		if _, err := os.Stat(p); err != nil {
			return evalbench.ProfilesConfig{}, nil
		}
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return evalbench.ProfilesConfig{}, fmt.Errorf("read profiles file %s: %w", p, err)
	}
	var cfg evalbench.ProfilesConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return evalbench.ProfilesConfig{}, fmt.Errorf("parse profiles file %s: %w", p, err)
	}
	return cfg, nil
}

func profilesForSuite(cfg evalbench.ProfilesConfig, suiteName string) evalbench.SuiteProfile {
	if cfg.Suites == nil {
		return evalbench.SuiteProfile{}
	}
	if p, ok := cfg.Suites[suiteName]; ok {
		return p
	}
	if suiteName == "typescript" {
		if p, ok := cfg.Suites["ts"]; ok {
			return p
		}
	}
	return evalbench.SuiteProfile{}
}

func expandEvalSuites(requested []string, profiles evalbench.ProfilesConfig) ([]string, bool, error) {
	seen := make(map[string]bool)
	var suites []string
	runReplay := false

	addSuite := func(s string) {
		if !seen[s] {
			seen[s] = true
			suites = append(suites, s)
		}
	}

	for _, raw := range requested {
		norm := evalbench.NormalizeSuiteName(raw)
		switch norm {
		case "all":
			for _, builtin := range []string{"go", "typescript", "vue", "rails"} {
				addSuite(builtin)
			}
			runReplay = true
		case "replay":
			runReplay = true
		case "go", "typescript", "vue", "rails":
			addSuite(norm)
		default:
			if prof := profilesForSuite(profiles, norm); prof.Dataset != "" {
				addSuite(norm)
				continue
			}
			return nil, false, fmt.Errorf("unknown suite %q (expected all,go,ts,vue,rails,replay or a custom suite defined in --profiles)", raw)
		}
	}
	return suites, runReplay, nil
}

func mergeSuiteVars(base, overrides map[string]string) map[string]string {
	if len(base) == 0 && len(overrides) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(overrides))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overrides {
		out[k] = v
	}
	return out
}

func writeReportFile(targetPath string, content []byte) error {
	dir := filepath.Dir(targetPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(targetPath, content, 0644)
}
