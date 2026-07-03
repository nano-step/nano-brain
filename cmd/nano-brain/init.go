package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/health/doctor"
)

func detectOllama(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// Orchestrator seams (D-01..D-04, D-16, D-17): each defaults to the real
// Wave-1/2 step function so production behavior is unchanged, but tests can
// override them to drive runInteractiveInit through canned outcomes without
// touching Docker, a real DB, the daemon, or the network.
var (
	stepDatabaseFn      = stepDatabase
	stepEmbeddingFn     = stepEmbedding
	runDoctorChecksFn   = defaultRunDoctorChecks
	stepServeFn         = stepServe
	registerWorkspaceFn = registerWorkspace
)

// defaultRunDoctorChecks loads config and runs doctor.RunAll, printing the
// same per-check table runDoctorCmd prints — but, unlike runDoctorCmd, it
// never os.Exit(1)s on a failing check. The wizard needs the checks (in
// particular whether PostgreSQL is healthy) without being killed before it
// can reach the serve gate (stepServe), which already knows how to react to
// a PostgreSQL failure (serveAborted).
func defaultRunDoctorChecks(configPath string) []doctor.Check {
	cfg, cfgErr := config.Load(configPath)
	results := doctor.RunAll(configPath, cfg, cfgErr, resolveBinaryPath())

	fmt.Print("\nnano-brain doctor\n\n")
	for _, r := range results {
		label := padRight(r.Name, 22)
		detail := padRight(r.Detail, 28)
		status := "OK"
		if r.Status == "fail" {
			status = "FAIL"
		} else if r.Status == "skip" {
			status = "SKIP"
		} else if r.Status == "warn" {
			status = "WARN"
		}
		fmt.Printf("  %s %s %s\n", label, detail, status)
		if (r.Status == "fail" || r.Status == "warn") && r.Hint != "" {
			for _, line := range strings.Split(r.Hint, "\n") {
				fmt.Printf("    → %s\n", line)
			}
		}
	}
	fmt.Println()

	return results
}

// defaultProbeDBURL/defaultProbeEmbURL/defaultProbeModel are the starting
// points for the D-02 probe sequence (database, embedding), shared by
// runInteractiveInit and runNonInteractiveInit so the two paths never drift
// on what "default" means before probing.
const (
	defaultProbeDBURL  = "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev"
	defaultProbeEmbURL = "http://localhost:11434"
	defaultProbeModel  = "nomic-embed-text"
)

// buildRenderedConfig runs the D-02 probe sequence (database, embedding)
// and returns the fully rendered commented config (D-03/D-04) ready to
// write to disk. It is shared by both runInteractiveInit and the --yes
// non-interactive path (D-08) so the two never diverge on what "default
// config" means. ok=false means the database step aborted (closed stdin
// mid-prompt, per CR-01) and the caller must not proceed.
func buildRenderedConfig(scanner *bufio.Scanner) (yaml string, notes string, ok bool) {
	dbURL, dbOK := stepDatabaseFn(scanner, defaultProbeDBURL)
	if !dbOK {
		return "", "", false
	}

	var notesBuf bytes.Buffer
	embBlock := stepEmbeddingFn(scanner, &notesBuf, defaultProbeEmbURL, defaultProbeModel)

	yaml = config.RenderConfig(config.RenderOpts{
		DatabaseURL:    dbURL,
		EmbeddingBlock: embBlock,
	})
	return yaml, notesBuf.String(), true
}

// writeConfigFile creates the config directory if needed and writes data to
// configPath, exiting the process on failure — shared by both init paths so
// the failure messages and permissions (0700 dir / 0600 file) never drift.
func writeConfigFile(configPath, data string) {
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create config directory: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(configPath, []byte(data), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write config: %v\n", err)
		os.Exit(1)
	}
}

// runInteractiveInit is the zero-question-by-default wizard orchestrator
// (D-01..D-10): TTY gate → config-exists [k]eep/[o]verwrite gate (D-10) →
// probe (database, embedding) → render the full commented config template
// (D-03/D-04) → write → doctor → serve step → register step → MCP config →
// summary. On a machine with reachable Postgres + Ollama this asks ZERO
// config questions (D-01) — stepAdvanced and all config-detail prompts from
// the Phase 13 wizard are gone; the commented template replaces the need to
// prompt for them (D-09).
func runInteractiveInit(configPath string) {
	if configPath == "" {
		configPath = config.ResolveConfigPath("")
	}

	// D-04 (Phase 13): interactive init requires a TTY. Non-interactive
	// callers (CI, scripts, piped stdin) get pointed at the flag-driven path
	// instead of silently hanging on a prompt read or writing a config with
	// no consent.
	if !isTTYFn() {
		fmt.Println("nano-brain init needs an interactive terminal (TTY).")
		fmt.Println("For non-interactive setup, use: nano-brain init --yes")
		return
	}

	fmt.Print("\nnano-brain setup\n────────────────\n\n")

	scanner := bufio.NewScanner(os.Stdin)

	// D-10 (Phase 13 D-03): config-exists gate defaults to KEEP (not
	// overwrite). Keep skips straight to the service steps (doctor → serve →
	// register → MCP) with zero config questions.
	keep := false
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("  Config exists at %s\n", configPath)
		answer, ok := promptConsequential(scanner, "[k]eep/[o]verwrite", "keep")
		if !ok || (!strings.EqualFold(answer, "o") && !strings.EqualFold(answer, "overwrite")) {
			keep = true
			fmt.Println("  Keeping existing config.")
		}
		fmt.Println()
	}

	if !keep {
		yaml, notes, ok := buildRenderedConfig(scanner)
		if !ok {
			fmt.Println("Aborted.")
			return
		}
		if notes != "" {
			fmt.Print(notes)
		}

		writeConfigFile(configPath, yaml)

		fmt.Printf("\nConfig written to %s\n\n", configPath)
	}

	// ── Doctor ──────────────────────────────────────────────────────────
	checks := runDoctorChecksFn(configPath)

	// ── Serve (D-14) ──────────────────────────────────────────────────────
	outcome := stepServeFn(scanner, checks, configPath)
	if outcome == serveAborted {
		os.Exit(1)
	}

	// ── Register (D-15) ───────────────────────────────────────────────────
	var res initResult
	registered := false
	if outcome == serveStarted || outcome == serveAlreadyRunning {
		cwd, _ := os.Getwd()
		wsDir, ok := promptConsequential(scanner, "Register this directory as a workspace? (path, or n to skip)", cwd)
		if ok && wsDir != "" && !strings.EqualFold(wsDir, "n") && !strings.EqualFold(wsDir, "no") && !strings.EqualFold(wsDir, "skip") {
			var err error
			res, err = registerWorkspaceFn(wsDir, "", false)
			if err != nil {
				fmt.Printf("  Registration failed: %v\n", err)
			} else {
				registered = true
			}
		} else {
			fmt.Println("  Skipped registration — MCP client config needs a registered workspace.")
		}
	}

	// ── Summary (D-17) ────────────────────────────────────────────────────
	// Never echo the DB password or any API key here — only the host/port
	// via getBaseURL() and the workspace name/hash are printed.
	fmt.Println("\n── Summary ──────────────────────")
	fmt.Printf("  Server: %s\n", getBaseURL())
	if registered {
		fmt.Printf("  Workspace: %s (%s)\n", res.Name, res.WorkspaceHash)
		fmt.Println("  MCP clients you selected above are now configured for this workspace.")
	} else {
		fmt.Println("  No workspace registered — MCP client config was skipped.")
	}
	fmt.Println("  Next: restart your AI client to pick up the new MCP configuration.")
	fmt.Println("──────────────────────────────────")
}

// runNonInteractiveInit implements D-08: the --yes / non-interactive path.
// It writes the rendered default config (DB from env/default via
// stepDatabaseFn's probe, embeddings auto-probed or off) with ZERO prompts,
// mirroring `ollama run` / `docker run`. Unlike runInteractiveInit it never
// reads stdin — the scanner passed to stepDatabaseFn/stepEmbeddingFn is
// backed by an already-closed reader, so any step that would otherwise
// prompt instead falls through its EOF-means-decline (CR-01) branch and
// keeps its non-interactive default. One consequence: if Postgres is
// unreachable at the default URL, stepDatabaseFn never auto-provisions via
// Docker here even when Docker is available — that offer is itself a
// prompt, and EOF declines it — so this path aborts with ok=false rather
// than silently starting a container without consent; the caller is told to
// set NANO_BRAIN_DATABASE_URL/DATABASE_URL or run interactive `nano-brain
// init` instead. It does NOT keep an existing config — callers who want
// that should omit --yes and use the interactive keep gate, or check for
// the file themselves before calling this. It always proceeds through
// doctor; it does not chain into serve/register/MCP, since those steps are
// interactive gates by design (D-08 only collapses config building).
func runNonInteractiveInit(configPath string) {
	if configPath == "" {
		configPath = config.ResolveConfigPath("")
	}

	// A scanner over an empty, already-closed reader: any prompt read inside
	// stepDatabaseFn/stepEmbeddingFn immediately hits EOF, so those steps
	// take their non-interactive default/decline path rather than blocking.
	scanner := bufio.NewScanner(strings.NewReader(""))

	yaml, notes, ok := buildRenderedConfig(scanner)
	if !ok {
		fmt.Fprintln(os.Stderr, "Failed to determine a usable database URL non-interactively.")
		fmt.Fprintln(os.Stderr, "Set NANO_BRAIN_DATABASE_URL or DATABASE_URL and retry, or run: nano-brain init")
		os.Exit(1)
	}
	if notes != "" {
		fmt.Print(notes)
	}

	preExisting := false
	if _, statErr := os.Stat(configPath); statErr == nil {
		preExisting = true
	}

	writeConfigFile(configPath, yaml)

	fmt.Printf("Config written to %s\n", configPath)

	checks := runDoctorChecksFn(configPath)
	for _, c := range checks {
		if c.Name == "PostgreSQL" && c.Status == "fail" {
			fmt.Println("  PostgreSQL check failed — the written config points at an unreachable database.")
			if c.Hint != "" {
				fmt.Printf("  %s\n", c.Hint)
			}
			// Don't leave a broken config behind that a later bare `init`
			// would silently keep — remove only the file we just created.
			if !preExisting {
				if rmErr := os.Remove(configPath); rmErr == nil {
					fmt.Printf("  Removed %s — fix the database, then re-run: nano-brain init --yes\n", configPath)
				} else {
					fmt.Fprintf(os.Stderr, "  Warning: failed to remove broken config %s: %v\n", configPath, rmErr)
				}
			}
			os.Exit(1)
		}
	}

	fmt.Printf("Start it with: %s\n", suggestStartCommand())
}
