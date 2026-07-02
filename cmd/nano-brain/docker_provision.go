package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// dockerRunner shells out to the docker CLI and reports structured results.
// stdout/stderr are captured text; exitCode is the process exit code (-1 if
// the docker binary itself could not be started); err is non-nil only when
// the binary could not be executed at all (e.g. not on PATH).
type dockerRunner func(ctx context.Context, args ...string) (stdout, stderr string, exitCode int, err error)

// runDocker is the package-level hook. Tests override it to avoid touching a
// real Docker daemon.
var runDocker dockerRunner = defaultRunDocker

// defaultRunDocker executes the real docker CLI via a fixed argv — the
// command name is always the literal "docker" and args are passed through
// exec.CommandContext without any shell interpretation, so no user input can
// ever be interpreted as shell metacharacters.
func defaultRunDocker(ctx context.Context, args ...string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), exitErr.ExitCode(), nil
		}
		// docker binary itself not found (or otherwise unexecutable) — wraps
		// exec.ErrNotFound in an *exec.Error.
		return outBuf.String(), errBuf.String(), -1, err
	}

	return outBuf.String(), errBuf.String(), 0, nil
}

// dockerStatus classifies the local docker CLI/daemon state.
type dockerStatusType string

const (
	dockerStatusNotInstalled     dockerStatusType = "not-installed"
	dockerStatusDaemonNotRunning dockerStatusType = "daemon-not-running"
	dockerStatusAvailable        dockerStatusType = "available"
	dockerStatusUnknownError     dockerStatusType = "unknown-error"
)

// dockerStatus runs `docker info` and classifies the result into one of the
// four dockerStatus* buckets. VERIFIED empirically against a real Docker
// daemon (see 13-RESEARCH.md Architecture Pattern 2).
func dockerStatus(ctx context.Context) dockerStatusType {
	_, stderr, exitCode, err := runDocker(ctx, "info")
	if err != nil {
		// *exec.Error wrapping ErrNotFound — docker binary is not on PATH.
		return dockerStatusNotInstalled
	}
	if exitCode != 0 {
		if strings.Contains(stderr, "Cannot connect to the Docker daemon") {
			return dockerStatusDaemonNotRunning
		}
		return dockerStatusUnknownError
	}
	return dockerStatusAvailable
}

const (
	dockerPGContainerName = "nanobrain-pg"
	dockerPGImage         = "pgvector/pgvector:pg17"
)

// provisionPostgres runs (or recovers/starts) the fixed nanobrain-pg
// container per D-06/D-07, returning the postgres:// URL the wizard should
// write to config. The container name and image are fixed literals — no
// user-entered value ever reaches the docker argv.
func provisionPostgres(ctx context.Context) (string, error) {
	const url5432 = "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev?sslmode=disable"
	const url5433 = "postgres://nanobrain:nanobrain@localhost:5433/nanobrain_dev?sslmode=disable"

	runArgs5432 := []string{
		"run", "-d",
		"--name", dockerPGContainerName,
		"--restart", "unless-stopped",
		"-p", "5432:5432",
		"-e", "POSTGRES_USER=nanobrain",
		"-e", "POSTGRES_PASSWORD=nanobrain",
		"-e", "POSTGRES_DB=nanobrain_dev",
		dockerPGImage,
	}

	_, stderr, exitCode, err := runDocker(ctx, runArgs5432...)
	if err != nil {
		return "", fmt.Errorf("docker run failed to execute: %w", err)
	}
	if exitCode == 0 {
		return url5432, nil
	}

	if exitCode != 125 {
		return "", fmt.Errorf("docker run failed (exit %d): %s", exitCode, stderr)
	}

	// exit 125 with a name conflict — the container already exists (likely
	// stopped). Start it instead of creating a new one.
	if strings.Contains(stderr, "is already in use by container") {
		_, startStderr, startExit, startErr := runDocker(ctx, "start", dockerPGContainerName)
		if startErr != nil {
			return "", fmt.Errorf("docker start failed to execute: %w", startErr)
		}
		if startExit != 0 {
			return "", fmt.Errorf("docker start failed (exit %d): %s", startExit, startStderr)
		}
		return url5432, nil
	}

	// exit 125 with a port conflict — docker leaves a stray Created-state
	// container behind (Pitfall 1). Remove it, then retry on port 5433.
	if strings.Contains(stderr, "port is already allocated") {
		// Ignore any error from rm (e.g. "no such container") — best-effort
		// cleanup of the stray container.
		_, _, _, _ = runDocker(ctx, "rm", dockerPGContainerName)

		runArgs5433 := []string{
			"run", "-d",
			"--name", dockerPGContainerName,
			"--restart", "unless-stopped",
			"-p", "5433:5433",
			"-e", "POSTGRES_USER=nanobrain",
			"-e", "POSTGRES_PASSWORD=nanobrain",
			"-e", "POSTGRES_DB=nanobrain_dev",
			dockerPGImage,
		}
		_, retryStderr, retryExit, retryErr := runDocker(ctx, runArgs5433...)
		if retryErr != nil {
			return "", fmt.Errorf("docker run (port 5433 retry) failed to execute: %w", retryErr)
		}
		if retryExit != 0 {
			return "", fmt.Errorf("docker run (port 5433 retry) failed (exit %d): %s", retryExit, retryStderr)
		}
		return url5433, nil
	}

	return "", fmt.Errorf("docker run failed (exit %d): %s", exitCode, stderr)
}
