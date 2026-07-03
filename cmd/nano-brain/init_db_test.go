package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

// withPostgresPinger overrides the package-level pingPostgres seam with a
// scripted sequence of results (nil = ready, non-nil = transient failure).
// Each call to pingPostgres consumes the next scripted result; once the
// script is exhausted, the last result repeats. Restored via t.Cleanup,
// mirroring withRecoveryHooks's save/override/restore idiom
// (commands_test.go:500-525).
func withPostgresPinger(t *testing.T, script []error) {
	t.Helper()
	orig := pingPostgres
	calls := 0
	pingPostgres = func(ctx context.Context, dbURL string) error {
		idx := calls
		if idx >= len(script) {
			idx = len(script) - 1
		}
		calls++
		return script[idx]
	}
	t.Cleanup(func() {
		pingPostgres = orig
	})
}

var errConnRefused = errors.New("connection refused")

func TestWaitForPostgresReady_ImmediateSuccess(t *testing.T) {
	withPostgresPinger(t, []error{nil})

	start := time.Now()
	err := waitForPostgresReady(context.Background(), "postgres://x", 2*time.Second, 50*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("returned too late for an immediate success: %s", elapsed)
	}
}

func TestWaitForPostgresReady_CancelledContext(t *testing.T) {
	withPostgresPinger(t, []error{errConnRefused})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForPostgresReady(ctx, "postgres://x", 2*time.Second, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestWaitForPostgresReady_Timeout(t *testing.T) {
	withPostgresPinger(t, []error{errConnRefused})

	start := time.Now()
	err := waitForPostgresReady(context.Background(), "postgres://x", 500*time.Millisecond, 100*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed < 500*time.Millisecond {
		t.Errorf("returned too early: %s", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Errorf("returned too late: %s", elapsed)
	}
}

func TestWaitForPostgresReady_RefuseThenReady(t *testing.T) {
	// Pitfall 2: Postgres self-restarts once during first-time init. The
	// poll loop must survive a refuse-then-ready sequence, not just a
	// single retry.
	withPostgresPinger(t, []error{errConnRefused, nil})

	start := time.Now()
	err := waitForPostgresReady(context.Background(), "postgres://x", 2*time.Second, 50*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected nil error after refuse-then-ready, got %v", err)
	}
	if elapsed > 1*time.Second {
		t.Errorf("returned too late: %s", elapsed)
	}
}

// withDockerStatusScript overrides runDocker so dockerStatus(ctx) returns
// the given status, reusing Plan 01's runDocker seam per RESEARCH's
// "prefer reusing runDocker to avoid a second seam" guidance.
func withDockerStatusScript(t *testing.T, status dockerStatusType) {
	t.Helper()
	orig := runDocker
	runDocker = func(ctx context.Context, args ...string) (string, string, int, error) {
		if len(args) > 0 && args[0] == "info" {
			switch status {
			case dockerStatusNotInstalled:
				return "", "", -1, errors.New("exec: \"docker\": executable file not found in $PATH")
			case dockerStatusDaemonNotRunning:
				return "", "Cannot connect to the Docker daemon at unix:///var/run/docker.sock", 1, nil
			case dockerStatusAvailable:
				return "ok", "", 0, nil
			default:
				return "", "some other error", 1, nil
			}
		}
		return "", "", 0, nil
	}
	t.Cleanup(func() {
		runDocker = orig
	})
}

func TestStepDatabase_ReachableDefault_NoPrompts(t *testing.T) {
	withPostgresPinger(t, []error{nil})

	// A non-empty buffer proves the scanner is never read from: if
	// stepDatabase issued any prompt, Scan() would consume this line and
	// the leftover-bytes check below would fail.
	input := "SHOULD-NOT-BE-CONSUMED\n"
	buf := bytes.NewBufferString(input)
	scanner := bufio.NewScanner(buf)

	dbURL, ok := stepDatabase(scanner, "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev")
	if !ok {
		t.Fatal("expected ok=true for reachable default")
	}
	if dbURL != "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev" {
		t.Errorf("dbURL = %q, want default URL unchanged", dbURL)
	}

	// Verify zero prompts were issued: if stepDatabase called Scan(), buf
	// would no longer hold the full original input.
	if remaining := buf.String(); remaining != input {
		t.Errorf("scanner was read from (remaining = %q), expected untouched input %q", remaining, input)
	}
}

func TestStepDatabase_UnreachableNoDocker_PromptsRemoteURL(t *testing.T) {
	// Ping script: default URL fails (attempt during detection), then the
	// remote URL the user enters succeeds (attempt during live-validation).
	withPostgresPinger(t, []error{errConnRefused, nil})
	withDockerStatusScript(t, dockerStatusNotInstalled)

	remoteURL := "postgres://user:pass@remote-host:5432/mydb"
	scanner := bufio.NewScanner(bytes.NewBufferString(remoteURL + "\n"))

	dbURL, ok := stepDatabase(scanner, "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev")
	if !ok {
		t.Fatal("expected ok=true when a valid remote URL is entered")
	}
	if dbURL != remoteURL {
		t.Errorf("dbURL = %q, want %q", dbURL, remoteURL)
	}
}

func TestStepDatabase_UnreachableDaemonNotRunning_PromptsRemoteURL(t *testing.T) {
	withPostgresPinger(t, []error{errConnRefused, nil})
	withDockerStatusScript(t, dockerStatusDaemonNotRunning)

	remoteURL := "postgres://user:pass@remote-host:5432/mydb"
	scanner := bufio.NewScanner(bytes.NewBufferString(remoteURL + "\n"))

	dbURL, ok := stepDatabase(scanner, "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev")
	if !ok {
		t.Fatal("expected ok=true when a valid remote URL is entered")
	}
	if dbURL != remoteURL {
		t.Errorf("dbURL = %q, want %q", dbURL, remoteURL)
	}
}

func TestStepDatabase_InvalidThenEscape_ReturnsEnteredURLWithWarning(t *testing.T) {
	// Ping script: default fails (detection), first entered URL fails
	// (live-validate), then the escape answer accepts without a further
	// successful ping.
	withPostgresPinger(t, []error{errConnRefused, errConnRefused})
	withDockerStatusScript(t, dockerStatusNotInstalled)

	invalidURL := "postgres://user:pass@unreachable-host:5432/mydb"
	// First line: an invalid URL that fails live-validation and re-prompts.
	// Second line: blank input is the "save anyway" escape hatch.
	scanner := bufio.NewScanner(bytes.NewBufferString(invalidURL + "\n\n"))

	dbURL, ok := stepDatabase(scanner, "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev")
	if !ok {
		t.Fatal("expected ok=true via the save-anyway escape hatch")
	}
	if dbURL != invalidURL {
		t.Errorf("dbURL = %q, want the entered (unvalidated) URL %q", dbURL, invalidURL)
	}
}

func TestStepDatabase_EOFAfterFailedEntry_DeclinesNotSaves(t *testing.T) {
	// CR-01: stdin closing after a failed URL entry is a decline, NOT an
	// implicit "save anyway" — ok must be false and no URL returned.
	withPostgresPinger(t, []error{errConnRefused, errConnRefused})
	withDockerStatusScript(t, dockerStatusNotInstalled)

	invalidURL := "postgres://user:pass@unreachable-host:5432/mydb"
	// One entered URL that fails live-validation, then the stream ends
	// (no trailing newline → scanner hits EOF at the re-prompt).
	scanner := bufio.NewScanner(bytes.NewBufferString(invalidURL))

	dbURL, ok := stepDatabase(scanner, "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev")
	if ok {
		t.Fatalf("expected ok=false on EOF after a failed entry, got ok=true with dbURL=%q", dbURL)
	}
	if dbURL != "" {
		t.Errorf("dbURL = %q, want empty on decline", dbURL)
	}
}

func TestStepDatabase_NonPostgresScheme_RejectedThenValidAccepted(t *testing.T) {
	// T-13-05 V5: non-postgres schemes never reach pgx.Connect — the ping
	// script has exactly one post-detection success, consumed by the valid
	// postgres:// entry (a ping on the mysql:// line would desync the script).
	withPostgresPinger(t, []error{errConnRefused, nil})
	withDockerStatusScript(t, dockerStatusNotInstalled)

	remoteURL := "postgres://user:pass@remote-host:5432/mydb"
	scanner := bufio.NewScanner(bytes.NewBufferString("mysql://user:pass@host:3306/db\n" + remoteURL + "\n"))

	dbURL, ok := stepDatabase(scanner, "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev")
	if !ok {
		t.Fatal("expected ok=true after rejecting the mysql:// URL and accepting the postgres:// one")
	}
	if dbURL != remoteURL {
		t.Errorf("dbURL = %q, want %q", dbURL, remoteURL)
	}
}

func TestStepDatabase_DockerAvailable_ProvisionsAndPolls(t *testing.T) {
	// Detection ping fails, docker is available, user accepts the
	// Docker-provision prompt, then the post-provision poll succeeds.
	withPostgresPinger(t, []error{errConnRefused, nil})

	origRunDocker := runDocker
	origProvision := provisionPostgresFn
	t.Cleanup(func() {
		runDocker = origRunDocker
		provisionPostgresFn = origProvision
	})

	runDocker = func(ctx context.Context, args ...string) (string, string, int, error) {
		if len(args) > 0 && args[0] == "info" {
			return "ok", "", 0, nil
		}
		return "", "", 0, nil
	}
	provisionedURL := "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev?sslmode=disable"
	provisionCalled := false
	provisionPostgresFn = func(ctx context.Context) (string, error) {
		provisionCalled = true
		return provisionedURL, nil
	}

	// User accepts the Docker-provision prompt with an affirmative answer.
	scanner := bufio.NewScanner(bytes.NewBufferString("Y\n"))

	dbURL, ok := stepDatabase(scanner, "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev")
	if !ok {
		t.Fatal("expected ok=true after successful Docker provisioning")
	}
	if !provisionCalled {
		t.Error("expected provisionPostgres to be called")
	}
	if dbURL != provisionedURL {
		t.Errorf("dbURL = %q, want %q", dbURL, provisionedURL)
	}
}

func TestStepDatabase_DockerAvailable_UserDeclines_PromptsRemoteURL(t *testing.T) {
	withPostgresPinger(t, []error{errConnRefused, nil})
	withDockerStatusScript(t, dockerStatusAvailable)

	remoteURL := "postgres://user:pass@remote-host:5432/mydb"
	// First line: decline the Docker-provision prompt. Second line: enter
	// a remote URL that live-validates successfully.
	scanner := bufio.NewScanner(bytes.NewBufferString("n\n" + remoteURL + "\n"))

	dbURL, ok := stepDatabase(scanner, "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev")
	if !ok {
		t.Fatal("expected ok=true when user declines Docker then enters a valid remote URL")
	}
	if dbURL != remoteURL {
		t.Errorf("dbURL = %q, want %q", dbURL, remoteURL)
	}
}
