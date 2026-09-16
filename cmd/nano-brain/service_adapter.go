package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// commandRunner executes external platform manager commands (launchctl,
// systemctl). Tests inject fakes so manager failure paths can be exercised
// without touching a live supervisor.
type commandRunner interface {
	run(ctx context.Context, argv []string) (stdout, stderr string, err error)
}

// execRunner runs manager commands through os/exec.
type execRunner struct{}

func (execRunner) run(ctx context.Context, argv []string) (string, string, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// servicePlatform owns the platform-specific pieces of managed-service
// lifecycle: paths, definition rendering, manager commands, and supervisor
// state inspection. The common layer in service.go owns state transitions,
// atomic writes, rollback, and exit-code semantics.
type servicePlatform interface {
	// name returns "darwin", "linux", or "unsupported".
	name() string
	// serviceID returns the launchd label / systemd unit name.
	serviceID() string
	// usable reports whether managed services are supported here, with an
	// actionable error for unsupported GOOS, root, containers, or an
	// unavailable user manager.
	usable() error
	// definitionPath returns the fixed per-user definition path.
	definitionPath() string
	// renderDefinition serializes the native definition for a spec.
	renderDefinition(spec serviceSpec) ([]byte, error)
	// register loads/enables/starts the written definition (idempotent);
	// also used by client recovery to (re)start a registered service.
	register(ctx context.Context) error
	// unregister stops/disables/boots out the registered service.
	unregister(ctx context.Context) error
	// restartService restarts the registered service through the manager.
	restartService(ctx context.Context) error
	// supervisorState returns "active", "inactive", "failed",
	// "unregistered", or "unknown" per the supervisor's view.
	supervisorState(ctx context.Context) (string, error)
}

// registered reports whether the owned definition file exists.
func registered(platform servicePlatform) bool {
	_, err := os.Stat(platform.definitionPath())
	return err == nil
}

// unsupportedPlatform implements servicePlatform for GOOS values without a
// native manager (Windows, BSD, ...). It always fails usable() with exit
// code 3 and never writes files.
type unsupportedPlatform struct{}

func (unsupportedPlatform) name() string           { return "unsupported" }
func (unsupportedPlatform) serviceID() string      { return "nano-brain" }
func (unsupportedPlatform) definitionPath() string { return "" }

func (unsupportedPlatform) usable() error {
	return fmt.Errorf("managed service registration is not supported on %s (supported: macOS launchd, Linux systemd user services)", runtime.GOOS)
}

func (unsupportedPlatform) renderDefinition(spec serviceSpec) ([]byte, error) {
	return nil, fmt.Errorf("managed services are not supported on %s", runtime.GOOS)
}

func (unsupportedPlatform) register(ctx context.Context) error {
	return fmt.Errorf("managed services are not supported on %s", runtime.GOOS)
}

func (unsupportedPlatform) unregister(ctx context.Context) error {
	return fmt.Errorf("managed services are not supported on %s", runtime.GOOS)
}

func (unsupportedPlatform) restartService(ctx context.Context) error {
	return fmt.Errorf("managed services are not supported on %s", runtime.GOOS)
}

func (unsupportedPlatform) supervisorState(ctx context.Context) (string, error) {
	return "unsupported", nil
}

// newServicePlatform returns the platform adapter for the current GOOS.
// The launchd/systemd adapters are plain runtime code (they only shell out
// to launchctl/systemctl), so this compiles on every GOOS while still
// rejecting unsupported platforms at runtime with exit code 3.
func newServicePlatform() servicePlatform {
	switch runtime.GOOS {
	case "darwin":
		return newLaunchdPlatform(execRunner{})
	case "linux":
		return newSystemdPlatform(execRunner{})
	default:
		return unsupportedPlatform{}
	}
}
