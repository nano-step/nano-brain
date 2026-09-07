//go:build !windows

package main

// runServeDaemonFn is the daemon launcher hook (see client.go). The
// default delegates to the real runServeDaemon in daemon.go. Tests
// override it (commands_test.go: withRecoveryHooks).
var runServeDaemonFn = runServeDaemon
