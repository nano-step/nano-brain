//go:build !windows

package main

// platformLaunchServeDaemon starts the nano-brain server as a background
// daemon on Unix-like platforms by delegating to the existing
// runServeDaemonFn test-hook seam (client.go). This is the ONLY file among
// the init_serve*.go set permitted to reference runServeDaemonFn, keeping
// the tag-free init_serve.go compilable under GOOS=windows.
func platformLaunchServeDaemon(configPath string) {
	runServeDaemonFn(configPath)
}
