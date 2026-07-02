package main

import (
	"bufio"
	"fmt"

	"github.com/nano-brain/nano-brain/internal/health/doctor"
)

// serveOutcome describes how the wizard's serve step concluded.
type serveOutcome int

const (
	// serveStarted means the daemon was launched and reported healthy.
	serveStarted serveOutcome = iota
	// serveSkipped means the user declined, stdin is not a TTY, or the
	// daemon failed to become healthy after being launched.
	serveSkipped
	// serveAlreadyRunning means a healthy server was already reachable
	// before any launch attempt was made.
	serveAlreadyRunning
	// serveAborted means a prerequisite check (PostgreSQL) failed, so the
	// daemon was never launched.
	serveAborted
	// serveSkippedWindows means the platform is Windows, where background
	// daemon mode is not yet supported; the user is told to run
	// `nano-brain serve` manually instead.
	serveSkippedWindows
)

// launchServeDaemonFn is the platform-specific daemon launcher hook. Tests
// override it. The real implementation lives in init_serve_unix.go
// (!windows) and init_serve_windows.go (windows) so this tag-free file
// stays buildable under both GOOS without a direct call to the daemon
// package's own serve-launch symbols (RESEARCH Pattern 4 / Pitfall 4).
var launchServeDaemonFn = platformLaunchServeDaemon

// serverHealthyFn is the health-probe hook used both for the
// already-running precheck and the post-launch readiness wait. Tests
// override it. The real implementation delegates to waitForServerHealthy
// (client_helpers.go).
var serverHealthyFn = func() bool {
	return waitForServerHealthy(serverHealthTimeout) == nil
}

// stepServe implements the D-14 wizard server-start step: it aborts when
// doctor's PostgreSQL check has failed (the server must not start against a
// broken DB), skips with an "already running" note when a healthy server is
// already reachable, prompts the user to start the server (skipping on
// decline or non-TTY), and otherwise launches the daemon via
// launchServeDaemonFn and waits for it to report healthy.
func stepServe(scanner *bufio.Scanner, checks []doctor.Check, configPath string) serveOutcome {
	fmt.Print("\n── Server ──\n")

	for _, c := range checks {
		if c.Name == "PostgreSQL" && c.Status == "fail" {
			fmt.Println("  PostgreSQL check failed — cannot start the server against a broken database.")
			if c.Hint != "" {
				fmt.Printf("  %s\n", c.Hint)
			}
			return serveAborted
		}
	}

	if serverHealthyFn() {
		fmt.Println("  nano-brain server is already running.")
		return serveAlreadyRunning
	}

	if !isTTYFn() {
		fmt.Println("  Skipping server start (non-interactive session).")
		fmt.Printf("  Start it manually with: %s\n", suggestStartCommand())
		return serveSkipped
	}

	if !promptStartServer(promptReader, promptWriter) {
		fmt.Printf("  Skipped. Start it later with: %s\n", suggestStartCommand())
		return serveSkipped
	}

	launchServeDaemonFn(configPath)

	if !serverHealthyFn() {
		fmt.Println("  Server started but did not become healthy in time. Check logs: ~/.nano-brain/logs/nano-brain.log")
		return serveSkipped
	}

	fmt.Println("  nano-brain server started.")
	return serveStarted
}
