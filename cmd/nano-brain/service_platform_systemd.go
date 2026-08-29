package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// systemdPlatform implements servicePlatform for Linux systemd user
// services: ~/.config/systemd/user/nano-brain.service driven through
// `systemctl --user`.
type systemdPlatform struct {
	runner commandRunner
	home   string
}

// newSystemdPlatform builds the Linux adapter. The runner is injectable so
// unit tests exercise manager failures without touching systemctl.
func newSystemdPlatform(runner commandRunner) *systemdPlatform {
	home, _ := os.UserHomeDir()
	return &systemdPlatform{runner: runner, home: home}
}

func (p *systemdPlatform) name() string      { return "linux" }
func (p *systemdPlatform) serviceID() string { return "nano-brain" }

func (p *systemdPlatform) definitionPath() string {
	return filepath.Join(p.home, ".config", "systemd", "user", "nano-brain.service")
}

func (p *systemdPlatform) usable() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("managed service registration requires Linux (found %s)", runtime.GOOS)
	}
	if os.Geteuid() == 0 {
		return fmt.Errorf("service installation as root is not supported; install as your regular user")
	}
	if isContainer() {
		return fmt.Errorf("service installation inside a container is not supported")
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl is not available: %w", err)
	}
	// A usable user manager needs XDG_RUNTIME_DIR; without it systemctl --user
	// cannot talk to the user bus.
	if os.Getenv("XDG_RUNTIME_DIR") == "" {
		return fmt.Errorf("no systemd user manager detected (XDG_RUNTIME_DIR is unset); log in with a graphical/linger session and run 'loginctl enable-linger \"$USER\"' to keep the service at boot")
	}
	return nil
}

func (p *systemdPlatform) renderDefinition(spec serviceSpec) ([]byte, error) {
	return renderSystemdUnit(spec.label, launcherServeArgv(spec)), nil
}

func (p *systemdPlatform) register(ctx context.Context) error {
	if _, stderr, err := p.runner.run(ctx, []string{"systemctl", "--user", "daemon-reload"}); err != nil {
		return fmt.Errorf("systemctl daemon-reload failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	if _, stderr, err := p.runner.run(ctx, []string{"systemctl", "--user", "enable", p.serviceID() + ".service"}); err != nil {
		return fmt.Errorf("systemctl enable failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	// restart (not start): on install it starts an inactive unit, and on
	// update it reloads a running service onto the rewritten unit — a plain
	// `start` is a no-op on an already-active unit, which would leave the
	// old binary running after an upgrade.
	if _, stderr, err := p.runner.run(ctx, []string{"systemctl", "--user", "restart", p.serviceID() + ".service"}); err != nil {
		return fmt.Errorf("systemctl restart failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	return nil
}

func (p *systemdPlatform) unregister(ctx context.Context) error {
	unit := p.serviceID() + ".service"
	if _, stderr, err := p.runner.run(ctx, []string{"systemctl", "--user", "stop", unit}); err != nil {
		if strings.Contains(stderr, "could not be found") || strings.Contains(stderr, "not loaded") {
			// Unit file already absent or never loaded — nothing registered.
			return nil
		}
		return fmt.Errorf("systemctl stop failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	if _, stderr, err := p.runner.run(ctx, []string{"systemctl", "--user", "disable", unit}); err != nil {
		return fmt.Errorf("systemctl disable failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	return nil
}

func (p *systemdPlatform) restartService(ctx context.Context) error {
	if _, stderr, err := p.runner.run(ctx, []string{"systemctl", "--user", "restart", p.serviceID() + ".service"}); err != nil {
		return fmt.Errorf("systemctl restart failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	return nil
}

// supervisorState maps `systemctl --user is-active` output: active,
// inactive, or failed. A missing unit prints "could not be found" and is
// reported as unregistered.
func (p *systemdPlatform) supervisorState(ctx context.Context) (string, error) {
	stdout, stderr, err := p.runner.run(ctx, []string{"systemctl", "--user", "is-active", p.serviceID() + ".service"})
	out := strings.TrimSpace(stdout)
	switch {
	case err == nil && out == "active":
		return "active", nil
	case err == nil && (out == "inactive" || out == "failed"):
		return out, nil
	case strings.Contains(stderr, "could not be found") || strings.Contains(out, "could not be found"):
		return "unregistered", nil
	}
	if err != nil {
		return "unknown", fmt.Errorf("systemctl is-active failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	return "unknown", nil
}
