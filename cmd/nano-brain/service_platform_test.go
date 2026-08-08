package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaunchdDefinitionPath(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)
	p := newLaunchdPlatform(&fakeRunner{})
	want := filepath.Join(home, "Library", "LaunchAgents", "com.nano-step.nano-brain.plist")
	if got := p.definitionPath(); got != want {
		t.Errorf("definitionPath = %q, want %q", got, want)
	}
}

func TestLaunchdRegisterSequence(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)
	runner := &fakeRunner{}
	p := newLaunchdPlatform(runner)

	if err := p.register(context.Background()); err != nil {
		t.Fatalf("register: %v", err)
	}
	cmds := runner.commandList()
	if len(cmds) != 3 {
		t.Fatalf("register calls = %v, want bootout+bootstrap+kickstart", cmds)
	}
	if !strings.Contains(cmds[0], "launchctl bootout gui/") {
		t.Errorf("first call = %q, want launchctl bootout", cmds[0])
	}
	if !strings.Contains(cmds[1], "launchctl bootstrap gui/") {
		t.Errorf("second call = %q, want launchctl bootstrap", cmds[1])
	}
	if !strings.HasSuffix(cmds[1], p.definitionPath()) {
		t.Errorf("bootstrap should target the written plist: %q", cmds[1])
	}
	if !strings.Contains(cmds[2], "launchctl kickstart -k gui/") {
		t.Errorf("third call = %q, want launchctl kickstart -k", cmds[2])
	}
}

func TestLaunchdRegisterBootstrapFailure(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)
	runner := &fakeRunner{fn: func(argv []string) (string, string, error) {
		if argv[1] == "bootstrap" {
			return "", "bootstrap: service already loaded", errors.New("exit status 5")
		}
		return "", "", nil
	}}
	p := newLaunchdPlatform(runner)
	err := p.register(context.Background())
	if err == nil || !strings.Contains(err.Error(), "bootstrap") {
		t.Fatalf("register err = %v, want bootstrap failure with stderr", err)
	}
}

func TestLaunchdUnregisterNotFoundIsIdempotent(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)
	runner := &fakeRunner{fn: func(argv []string) (string, string, error) {
		if argv[1] == "bootout" {
			return "", "Could not find service \"com.nano-step.nano-brain\" in domain for user", errors.New("exit status 3")
		}
		return "", "", nil
	}}
	p := newLaunchdPlatform(runner)
	if err := p.unregister(context.Background()); err != nil {
		t.Fatalf("unregister on absent service should be idempotent: %v", err)
	}
}

func TestLaunchdSupervisorStateMatrix(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		stderr string
		err    error
		want   string
	}{
		{"active", "state = running\n", "", nil, "active"},
		{"inactive", "state = not running\n", "", nil, "inactive"},
		{"unregistered", "", "Could not find service \"com.nano-step.nano-brain\" in domain", errors.New("exit status 3"), "unregistered"},
		{"unknown on other error", "", "launchctl: internal error", errors.New("exit status 1"), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			withHome(t, home)
			runner := &fakeRunner{fn: func(argv []string) (string, string, error) {
				return tt.stdout, tt.stderr, tt.err
			}}
			p := newLaunchdPlatform(runner)
			got, err := p.supervisorState(context.Background())
			if err != nil && tt.want != "unknown" {
				t.Fatalf("supervisorState err = %v", err)
			}
			if got != tt.want {
				t.Errorf("supervisorState = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSystemdRegisterSequence(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)
	runner := &fakeRunner{}
	p := newSystemdPlatform(runner)
	if err := p.register(context.Background()); err != nil {
		t.Fatalf("register: %v", err)
	}
	cmds := runner.commandList()
	want := []string{
		"systemctl --user daemon-reload",
		"systemctl --user enable nano-brain.service",
		"systemctl --user restart nano-brain.service",
	}
	if len(cmds) != len(want) {
		t.Fatalf("register calls = %v, want %v", cmds, want)
	}
	for i := range want {
		if cmds[i] != want[i] {
			t.Errorf("call %d = %q, want %q", i, cmds[i], want[i])
		}
	}
}

func TestSystemdUnregisterAndState(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)

	t.Run("stop then disable", func(t *testing.T) {
		runner := &fakeRunner{}
		p := newSystemdPlatform(runner)
		if err := p.unregister(context.Background()); err != nil {
			t.Fatalf("unregister: %v", err)
		}
		cmds := runner.commandList()
		if len(cmds) != 2 || cmds[0] != "systemctl --user stop nano-brain.service" || cmds[1] != "systemctl --user disable nano-brain.service" {
			t.Errorf("unregister calls = %v", cmds)
		}
	})

	t.Run("absent unit idempotent", func(t *testing.T) {
		runner := &fakeRunner{fn: func(argv []string) (string, string, error) {
			if argv[2] == "stop" {
				return "", "Unit nano-brain.service could not be found.", errors.New("exit status 5")
			}
			return "", "", nil
		}}
		p := newSystemdPlatform(runner)
		if err := p.unregister(context.Background()); err != nil {
			t.Fatalf("unregister on absent unit should be idempotent: %v", err)
		}
	})

	stateTests := []struct {
		name   string
		stdout string
		stderr string
		err    error
		want   string
	}{
		{"active", "active\n", "", nil, "active"},
		{"inactive", "inactive\n", "", nil, "inactive"},
		{"failed", "failed\n", "", nil, "failed"},
		{"unregistered", "", "Unit nano-brain.service could not be found.", errors.New("exit status 4"), "unregistered"},
	}
	for _, tt := range stateTests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{fn: func(argv []string) (string, string, error) {
				return tt.stdout, tt.stderr, tt.err
			}}
			p := newSystemdPlatform(runner)
			got, err := p.supervisorState(context.Background())
			if err != nil && tt.want != "unregistered" {
				t.Fatalf("supervisorState err = %v", err)
			}
			if got != tt.want {
				t.Errorf("supervisorState = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSystemdRenderDefinition(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)
	p := newSystemdPlatform(&fakeRunner{})
	data, err := p.renderDefinition(serviceSpec{
		label:      "nano-brain",
		launcher:   []string{"/usr/bin/node", "/usr/local/lib/node_modules/@nano-step/nano-brain/npm/run.js"},
		configPath: "/home/user/.nano-brain/config.yml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "ExecStart=/usr/bin/node /usr/local/lib/node_modules/@nano-step/nano-brain/npm/run.js --config /home/user/.nano-brain/config.yml serve") {
		t.Errorf("unexpected unit:\n%s", string(data))
	}
}

func TestPlatformUsableRootAndContainer(t *testing.T) {
	// Root check: only meaningful when euid==0; skip otherwise.
	if os.Geteuid() == 0 {
		t.Skip("running as root")
	}
	home := t.TempDir()
	withHome(t, home)
	p := newLaunchdPlatform(&fakeRunner{})
	if err := p.usable(); err != nil {
		t.Fatalf("usable on a normal mac session should pass or fail on launchctl presence only: %v", err)
	}
}

func TestLaunchdUnregisterNoSuchProcessIsIdempotent(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)
	runner := &fakeRunner{fn: func(argv []string) (string, string, error) {
		if argv[1] == "bootout" {
			return "", "Boot-out failed: 3: No such process", errors.New("exit status 3")
		}
		return "", "", nil
	}}
	p := newLaunchdPlatform(runner)
	if err := p.unregister(context.Background()); err != nil {
		t.Fatalf("unregister on an absent job should be idempotent: %v", err)
	}
}

func TestSystemdUnregisterNotLoadedIsIdempotent(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)
	runner := &fakeRunner{fn: func(argv []string) (string, string, error) {
		if argv[2] == "stop" {
			return "", "Unit nano-brain.service not loaded.", errors.New("exit status 5")
		}
		return "", "", nil
	}}
	p := newSystemdPlatform(runner)
	if err := p.unregister(context.Background()); err != nil {
		t.Fatalf("unregister on a not-loaded unit should be idempotent: %v", err)
	}
}
