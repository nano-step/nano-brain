//go:build windows

package main

import "fmt"

// platformLaunchServeDaemon on Windows does not start a background daemon
// (background daemon mode is not yet supported on this platform — the
// underlying runServeDaemon/pidFilePath symbols in daemon.go are excluded
// from Windows builds entirely, per RESEARCH Pitfall 4). It prints the
// manual instruction instead and returns without referencing any daemon
// symbols.
func platformLaunchServeDaemon(configPath string) {
	fmt.Println("  Background daemon mode is not yet supported on Windows.")
	fmt.Println("  Start the server manually in another terminal: nano-brain serve")
}
