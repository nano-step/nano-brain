//go:build !windows

package main

// legacyDaemonRunning reports whether a live legacy PID-file daemon exists.
// The managed service refuses to register while one is running so the
// supervised process never competes with it (issue #615).
func legacyDaemonRunning() (running bool, pid int) {
	pid, err := readPID()
	if err != nil {
		return false, 0
	}
	return isRunning(pid), pid
}
