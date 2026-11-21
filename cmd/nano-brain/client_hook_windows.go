//go:build windows

package main

import (
	"fmt"
	"os"
)

// runServeDaemonFn on Windows cannot daemonize — daemon.go's runServeDaemon
// is excluded from the Windows build (//go:build !windows) because the
// POSIX primitives it relies on (setsid, /dev/null, SIGTERM via signal 0,
// forked detached child) are not available. The default prints the same
// foreground-only message that daemon_windows.go's runServeDaemon prints,
// so the auto-start path used by client.go's recoverFromConnectionRefused
// gives a clear, actionable message instead of failing with a
// missing-symbol link error.
//
// Tests may override this hook as on Unix (commands_test.go: withRecoveryHooks).
var runServeDaemonFn = func(string) {
	fmt.Fprintln(os.Stderr, "Background daemon mode is not supported on Windows.")
	fmt.Fprintln(os.Stderr, "Start the server manually in another terminal: nano-brain serve")
	os.Exit(1)
}
