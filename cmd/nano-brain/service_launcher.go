package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nano-brain/nano-brain/internal/config"
)

// serviceSpec is the fully-resolved definition the native manager owns.
// Only absolute, argv-safe, stable paths are serialized — never shell
// environment, DATABASE_URL, auth tokens, or API keys.
type serviceSpec struct {
	label      string   // launchd label / systemd unit name
	launcher   []string // absolute stable launcher argv, without the serve args
	configPath string   // absolute config file path
}

// validateExecutable checks that path exists, is a regular file, and is
// executable.
func validateExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%q does not exist or cannot be read: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", path)
	}
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("%q is not executable", path)
	}
	return nil
}

// resolveLauncher resolves the stable launcher argv prefix for a persistent
// service definition. Precedence:
//
//  1. A validated NANO_BRAIN_BIN is installed as a direct executable.
//  2. Global npm wrapper metadata set by npm/run.js (absolute run.js + node)
//     is used only when the package resolves from a global npm root.
//  3. The canonicalized os.Executable() path, validated as a regular
//     executable, for direct binary installations.
//
// Local/npx/ephemeral npm launchers are rejected with a migration message so
// a persistent service is never pinned to an ephemeral cache.
func resolveLauncher() ([]string, string, error) {
	if bin := strings.TrimSpace(os.Getenv("NANO_BRAIN_BIN")); bin != "" {
		if err := validateExecutable(bin); err != nil {
			return nil, "", fmt.Errorf("NANO_BRAIN_BIN: %w", err)
		}
		return []string{bin}, "NANO_BRAIN_BIN", nil
	}

	runjs := os.Getenv("NANO_BRAIN_NPM_RUNJS")
	if runjs != "" && os.Getenv("NANO_BRAIN_NPM_GLOBAL") == "true" {
		absRunJS, err := filepath.Abs(runjs)
		if err != nil {
			return nil, "", fmt.Errorf("npm wrapper run.js: %w", err)
		}
		node := os.Getenv("NANO_BRAIN_NPM_NODE")
		if node == "" {
			node = "node"
		}
		if err := validateExecutable(absRunJS); err != nil {
			return nil, "", fmt.Errorf("npm wrapper: %w", err)
		}
		return []string{node, absRunJS}, "global npm wrapper", nil
	}

	if os.Getenv("NANO_BRAIN_NPM_LAUNCHED") == "true" {
		return nil, "", fmt.Errorf("npx/local npm launchers cannot be pinned to a persistent service; install globally with 'npm install -g @nano-step/nano-brain' (or use a direct binary via NANO_BRAIN_BIN) and run 'nano-brain service install' again")
	}

	exe, err := os.Executable()
	if err != nil {
		return nil, "", fmt.Errorf("cannot resolve binary path: %w", err)
	}
	if err := validateExecutable(exe); err != nil {
		return nil, "", fmt.Errorf("binary path %q: %w", exe, err)
	}
	return []string{exe}, "direct binary", nil
}

// resolveInstalledConfig resolves the absolute config path recorded into a
// service definition: the --config argument, else NANO_BRAIN_CONFIG, else the
// default ~/.nano-brain/config.yml. The path is made absolute so service
// startup is independent of the manager's working directory.
func resolveInstalledConfig(configPath string) (string, error) {
	path := configPath
	if path == "" {
		path = os.Getenv("NANO_BRAIN_CONFIG")
	}
	if path == "" {
		path = config.ResolveConfigPath("")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve config path %q: %w", path, err)
	}
	return abs, nil
}

// launcherServeArgv returns the full service argv: stable launcher plus
// `--config <abs> serve` so the foreground daemon owns the configured file.
func launcherServeArgv(spec serviceSpec) []string {
	argv := append([]string{}, spec.launcher...)
	argv = append(argv, "--config", spec.configPath, "serve")
	return argv
}

// buildServiceSpec resolves the launcher and config for a fresh install or
// update. It validates the selected config file loads, and refuses when a
// live legacy PID-file daemon is already running so the managed service
// never competes with it.
func buildServiceSpec(platform servicePlatform, configPath string) (serviceSpec, error) {
	launcher, _, err := resolveLauncher()
	if err != nil {
		return serviceSpec{}, err
	}
	absConfig, err := resolveInstalledConfig(configPath)
	if err != nil {
		return serviceSpec{}, err
	}
	if _, err := config.Load(absConfig); err != nil {
		return serviceSpec{}, fmt.Errorf("selected config %s does not load: %w", absConfig, err)
	}
	if pid, err := readPID(); err == nil && isRunning(pid) {
		return serviceSpec{}, fmt.Errorf("a legacy daemon is already running (PID %d). Stop it with 'nano-brain stop' before registering the managed service", pid)
	}
	return serviceSpec{
		label:      platform.serviceID(),
		launcher:   launcher,
		configPath: absConfig,
	}, nil
}
