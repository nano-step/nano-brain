package main

import (
	"fmt"
	"os"
)

// isTTY reports whether BOTH os.Stdin and os.Stderr are connected to a
// character device (terminal). Stdlib-only: uses os.ModeCharDevice from
// each file's Stat() mode. Returns false on any Stat error.
func isTTY() bool {
	return isCharDevice(os.Stdin) && isCharDevice(os.Stderr)
}

func isCharDevice(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// isNpxLaunched reports whether the CLI was launched via npx/npm. npx and
// npm both leave breadcrumb environment variables in the child process.
func isNpxLaunched() bool {
	if os.Getenv("npm_execpath") != "" {
		return true
	}
	if os.Getenv("npm_package_name") != "" {
		return true
	}
	return false
}

// suggestStartCommand returns the user-facing command to start the
// nano-brain server, tailored to how this binary was launched.
func suggestStartCommand() string {
	if isNpxLaunched() {
		return "npx @nano-step/nano-brain@beta serve -d"
	}
	return "nano-brain serve -d"
}

// formatConnectError builds the 3-line user-facing error shown when the
// CLI cannot reach the server (header, hint, action).
func formatConnectError(host string, port int) string {
	return fmt.Sprintf(
		"Error: cannot connect to nano-brain server at %s:%d\n"+
			"The server does not appear to be running.\n"+
			"Run this to start it: %s",
		host, port, suggestStartCommand(),
	)
}
