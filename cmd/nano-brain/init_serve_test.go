package main

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/nano-brain/nano-brain/internal/health/doctor"
)

// withServeHooks saves/overrides/restores the stepServe test seams
// (launchServeDaemonFn, serverHealthyFn, isTTYFn, promptReader, promptWriter)
// so tests never spawn a real daemon process or contact a real health
// endpoint. Mirrors withRecoveryHooks in commands_test.go. accept controls
// the promptReader content the internal promptStartServer call will read.
func withServeHooks(t *testing.T, isTTYReturn bool, healthy bool, accept bool, launchCount *int) {
	t.Helper()
	origLaunch := launchServeDaemonFn
	origHealthy := serverHealthyFn
	origIsTTY := isTTYFn
	origReader := promptReader
	origWriter := promptWriter

	launchServeDaemonFn = func(string) {
		if launchCount != nil {
			*launchCount++
		}
	}
	serverHealthyFn = func() bool { return healthy }
	isTTYFn = func() bool { return isTTYReturn }
	if accept {
		promptReader = bytes.NewBufferString("Y\n")
	} else {
		promptReader = bytes.NewBufferString("n\n")
	}
	promptWriter = &bytes.Buffer{}

	t.Cleanup(func() {
		launchServeDaemonFn = origLaunch
		serverHealthyFn = origHealthy
		isTTYFn = origIsTTY
		promptReader = origReader
		promptWriter = origWriter
	})
}

func TestStepServe_AbortsOnPostgreSQLFail(t *testing.T) {
	launchCount := 0
	withServeHooks(t, true, false, true, &launchCount)

	checks := []doctor.Check{
		{Name: "PostgreSQL", Status: "fail", Detail: "no URL configured"},
	}
	scanner := bufio.NewScanner(bytes.NewBufferString(""))

	got := stepServe(scanner, checks, "")

	if got != serveAborted {
		t.Errorf("stepServe() = %v, want serveAborted", got)
	}
	if launchCount != 0 {
		t.Errorf("launchServeDaemonFn called %d times, want 0", launchCount)
	}
}

func TestStepServe_AlreadyRunningSkipsLaunch(t *testing.T) {
	launchCount := 0
	withServeHooks(t, true, true, true, &launchCount)

	checks := []doctor.Check{
		{Name: "PostgreSQL", Status: "ok", Detail: "localhost:5432"},
	}
	scanner := bufio.NewScanner(bytes.NewBufferString(""))

	got := stepServe(scanner, checks, "")

	if got != serveAlreadyRunning {
		t.Errorf("stepServe() = %v, want serveAlreadyRunning", got)
	}
	if launchCount != 0 {
		t.Errorf("launchServeDaemonFn called %d times, want 0", launchCount)
	}
}

func TestStepServe_AcceptAndStart(t *testing.T) {
	launchCount := 0
	withServeHooks(t, true, false, true, &launchCount)
	// Override serverHealthyFn again so the already-running precheck (called
	// before launch) reports unhealthy, but the post-launch wait reports
	// healthy once the daemon has been launched.
	serverHealthyFn = func() bool {
		return launchCount > 0
	}

	checks := []doctor.Check{
		{Name: "PostgreSQL", Status: "ok", Detail: "localhost:5432"},
	}
	scanner := bufio.NewScanner(bytes.NewBufferString(""))

	got := stepServe(scanner, checks, "/tmp/config.yaml")

	if got != serveStarted {
		t.Errorf("stepServe() = %v, want serveStarted", got)
	}
	if launchCount != 1 {
		t.Errorf("launchServeDaemonFn called %d times, want 1", launchCount)
	}
}

func TestStepServe_DeclineSkipsLaunch(t *testing.T) {
	launchCount := 0
	withServeHooks(t, true, false, false, &launchCount)

	checks := []doctor.Check{
		{Name: "PostgreSQL", Status: "ok", Detail: "localhost:5432"},
	}
	scanner := bufio.NewScanner(bytes.NewBufferString(""))

	got := stepServe(scanner, checks, "")

	if got != serveSkipped {
		t.Errorf("stepServe() = %v, want serveSkipped", got)
	}
	if launchCount != 0 {
		t.Errorf("launchServeDaemonFn called %d times, want 0", launchCount)
	}
}

func TestStepServe_NonTTYSkipsLaunch(t *testing.T) {
	launchCount := 0
	withServeHooks(t, false, false, true, &launchCount)

	checks := []doctor.Check{
		{Name: "PostgreSQL", Status: "ok", Detail: "localhost:5432"},
	}
	scanner := bufio.NewScanner(bytes.NewBufferString(""))

	got := stepServe(scanner, checks, "")

	if got != serveSkipped {
		t.Errorf("stepServe() = %v, want serveSkipped", got)
	}
	if launchCount != 0 {
		t.Errorf("launchServeDaemonFn called %d times, want 0", launchCount)
	}
}
