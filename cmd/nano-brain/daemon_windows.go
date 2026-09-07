//go:build windows

package main

// Windows stubs for the daemon lifecycle. Background daemon mode (setsid,
// detached child, PID-file lifecycle, SIGTERM) is not implemented on
// Windows because the underlying primitives differ; users run
// `nano-brain serve` in the foreground instead, or wrap it with NSSM /
// the Windows Service Control Manager for true background behavior.
//
// These symbols MUST exist (same names as daemon.go) so main.go and
// client.go compile under GOOS=windows. They do NOT need to do real
// work — the foreground `nano-brain serve` path (runServeCmd →
// startServer) is the supported Windows entry point.

import (
	"fmt"
	"os"
	"path/filepath"
)

// pidFilePath returns %USERPROFILE%/.nano-brain/nano-brain.pid — the same
// layout Unix uses via os.UserHomeDir(). The function exists because
// main.go's daemon-child defer references it unconditionally; on Windows
// we never write the file (runServeDaemon never returns), but the symbol
// still has to resolve so the package compiles.
func pidFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "nano-brain.pid"
	}
	return filepath.Join(home, ".nano-brain", "nano-brain.pid")
}

// readPID is a Windows stub. Background daemon mode is not implemented
// here, so no PID file is ever written; we return ErrNotExist so callers
// treat any stale file as "process is gone" and proceed to clean up.
func readPID() (int, error) { return 0, os.ErrNotExist }

// isRunning is a Windows stub (see readPID above).
func isRunning(pid int) bool { return false }

// runServeCmd on Windows ignores -d/--detach and runs the server in the
// foreground. The Windows process model (no setsid, different console
// attachment) does not support POSIX-style daemonization. Users who want
// background behavior should run `start /B nano-brain serve` from cmd.exe
// or use a service wrapper like NSSM.
func runServeCmd(args []string, configPath string) {
	cliLog.Debug().Str("cmd", "serve").Msg("cli command started")
	for _, a := range args {
		switch a {
		case "-d", "--detach":
			fmt.Fprintln(os.Stderr, "Note: -d/--detach is not supported on Windows; starting in foreground. Use `start /B nano-brain serve` from cmd.exe to background.")
		case "--unsafe-no-auth":
			unsafeNoAuth = true
		case "--serve-only":
			serveOnlyFlag = true
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", a)
			fmt.Fprintln(os.Stderr, "Usage: nano-brain serve [-d] [--unsafe-no-auth] [--serve-only]")
			os.Exit(1)
		}
	}
	startServer(configPath)
}

// runServeDaemon is the auto-start path used when a client tool (e.g.
// `nano-brain query`) detects the server is not running and the user
// accepts the auto-start prompt. On Windows we cannot daemonize, so we
// inform the user how to start the server manually and exit — the calling
// client will then report its own connection-refused error.
func runServeDaemon(configPath string) {
	fmt.Fprintln(os.Stderr, "Background daemon mode is not supported on Windows.")
	fmt.Fprintln(os.Stderr, "Start the server manually in another terminal: nano-brain serve")
	os.Exit(1)
}

func runStopCmd() {
	cliLog.Debug().Str("cmd", "stop").Msg("cli command started")
	fmt.Fprintln(os.Stderr, "`nano-brain stop` is not supported on Windows (background daemon mode is not implemented).")
	fmt.Fprintln(os.Stderr, "If the server is running in another terminal, stop it with Ctrl+C.")
	os.Exit(1)
}

func runRestartCmd(args []string, configPath string) {
	cliLog.Debug().Str("cmd", "restart").Msg("cli command started")
	fmt.Fprintln(os.Stderr, "`nano-brain restart` is not supported on Windows (background daemon mode is not implemented).")
	os.Exit(1)
}
