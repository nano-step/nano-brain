package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// launchdPlatform implements servicePlatform for macOS LaunchAgent
// registration: ~/Library/LaunchAgents/com.nano-step.nano-brain.plist under
// the per-user gui/<uid> domain.
type launchdPlatform struct {
	runner commandRunner
	uid    int
	home   string
}

// newLaunchdPlatform builds the macOS adapter. The runner is injectable so
// unit tests exercise manager failures without touching launchctl.
func newLaunchdPlatform(runner commandRunner) *launchdPlatform {
	home, _ := os.UserHomeDir()
	return &launchdPlatform{runner: runner, uid: os.Getuid(), home: home}
}

func (p *launchdPlatform) name() string      { return "darwin" }
func (p *launchdPlatform) serviceID() string { return "com.nano-step.nano-brain" }

func (p *launchdPlatform) definitionPath() string {
	return filepath.Join(p.home, "Library", "LaunchAgents", "com.nano-step.nano-brain.plist")
}

func (p *launchdPlatform) logDir() string {
	return filepath.Join(p.home, ".nano-brain", "logs")
}

// target is the launchd domain target for the per-user service.
func (p *launchdPlatform) target() string {
	return fmt.Sprintf("gui/%d/%s", p.uid, p.serviceID())
}

func (p *launchdPlatform) usable() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("managed service registration requires macOS (found %s)", runtime.GOOS)
	}
	if os.Geteuid() == 0 {
		return fmt.Errorf("service installation as root is not supported; install as your regular user")
	}
	if isContainer() {
		return fmt.Errorf("service installation inside a container is not supported")
	}
	if _, err := exec.LookPath("launchctl"); err != nil {
		return fmt.Errorf("launchctl is not available: %w", err)
	}
	return nil
}

func (p *launchdPlatform) renderDefinition(spec serviceSpec) ([]byte, error) {
	return renderLaunchdPlist(spec.label, launcherServeArgv(spec), p.logDir()), nil
}

// register loads the written plist and starts the service. bootout is best
// effort (the service may not be loaded yet); bootstrap retries with a short
// backoff because launchd fails with "Input/output error" when it is still
// tearing down the previous job (state = SIGTERMed); kickstart force-restarts
// onto the new definition.
func (p *launchdPlatform) register(ctx context.Context) error {
	_, _, _ = p.runner.run(ctx, []string{"launchctl", "bootout", p.target()})
	domain := fmt.Sprintf("gui/%d", p.uid)
	def := p.definitionPath()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if _, stderr, err := p.runner.run(ctx, []string{"launchctl", "bootstrap", domain, def}); err != nil {
			lastErr = fmt.Errorf("launchctl bootstrap failed: %w: %s", err, strings.TrimSpace(stderr))
			select {
			case <-ctx.Done():
				return lastErr
			case <-time.After(700 * time.Millisecond):
			}
			continue
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		return lastErr
	}
	if _, stderr, err := p.runner.run(ctx, []string{"launchctl", "kickstart", "-k", p.target()}); err != nil {
		return fmt.Errorf("launchctl kickstart failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	return nil
}

func (p *launchdPlatform) unregister(ctx context.Context) error {
	if _, stderr, err := p.runner.run(ctx, []string{"launchctl", "bootout", p.target()}); err != nil {
		if strings.Contains(stderr, "Could not find service") {
			return nil // already unloaded
		}
		return fmt.Errorf("launchctl bootout failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	return nil
}

func (p *launchdPlatform) restartService(ctx context.Context) error {
	if _, stderr, err := p.runner.run(ctx, []string{"launchctl", "kickstart", "-k", p.target()}); err != nil {
		return fmt.Errorf("launchctl kickstart failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	return nil
}

// supervisorState inspects `launchctl print gui/<uid>/<label>`. A loaded and
// running job prints `state = running`; an absent job prints
// "Could not find service".
func (p *launchdPlatform) supervisorState(ctx context.Context) (string, error) {
	stdout, stderr, err := p.runner.run(ctx, []string{"launchctl", "print", p.target()})
	if err != nil {
		if strings.Contains(stderr, "Could not find service") {
			return "unregistered", nil
		}
		return "unknown", fmt.Errorf("launchctl print failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	if strings.Contains(stdout, "state = running") {
		return "active", nil
	}
	if strings.Contains(stdout, "state = not running") {
		return "inactive", nil
	}
	if strings.Contains(stdout, "state = ") {
		idx := strings.Index(stdout, "state = ")
		rest := stdout[idx+len("state = "):]
		state := strings.Fields(rest)
		if len(state) > 0 {
			return state[0], nil
		}
	}
	return "inactive", nil
}
