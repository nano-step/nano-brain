//go:build windows

package main

// legacyDaemonRunning reports whether a live legacy PID-file daemon exists.
// The PID-file daemon is a Unix-only path; on Windows there is never a
// legacy daemon to collide with, so managed-service install is free to
// proceed (and is then rejected by the platform guard with exit code 3).
func legacyDaemonRunning() (running bool, pid int) {
	return false, 0
}
