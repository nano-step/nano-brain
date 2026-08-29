package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeRunner records invoked argv lists and delegates to fn, letting tests
// exercise manager failures without touching launchctl/systemctl.
type fakeRunner struct {
	mu    sync.Mutex
	calls [][]string
	fn    func(argv []string) (string, string, error)
}

func (f *fakeRunner) run(_ context.Context, argv []string) (string, string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, append([]string{}, argv...))
	f.mu.Unlock()
	if f.fn == nil {
		return "", "", nil
	}
	return f.fn(argv)
}

func (f *fakeRunner) commandList() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		out = append(out, strings.Join(c, " "))
	}
	return out
}

func withHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
}

func TestParseServiceArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantOp  serviceOperation
		wantErr bool
	}{
		{"install", []string{"install"}, serviceOpInstall, false},
		{"install json", []string{"install", "--json"}, serviceOpInstall, false},
		{"status", []string{"status"}, serviceOpStatus, false},
		{"status json", []string{"status", "--json"}, serviceOpStatus, false},
		{"uninstall", []string{"uninstall"}, serviceOpUninstall, false},
		{"restart", []string{"restart"}, serviceOpRestart, false},
		{"update", []string{"update"}, serviceOpUpdate, false},
		{"missing subcommand", nil, "", true},
		{"unknown subcommand", []string{"explode"}, "", true},
		{"unknown flag", []string{"status", "--verbose"}, "", true},
		{"unexpected argument", []string{"install", "extra"}, "", true},
		{"flag before subcommand", []string{"--json", "status"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, opts, err := parseServiceArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseServiceArgs(%v) = %v, %+v; want error", tt.args, op, opts)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseServiceArgs(%v) unexpected error: %v", tt.args, err)
			}
			if op != tt.wantOp {
				t.Errorf("parseServiceArgs(%v) op = %q, want %q", tt.args, op, tt.wantOp)
			}
		})
	}
}

func TestStatusExitCode(t *testing.T) {
	ready := serviceStatus{Registered: true, SupervisorState: "active", HealthReachable: true, Ready: true}
	tests := []struct {
		name string
		st   serviceStatus
		want int
	}{
		{"ready", ready, serviceExitOK},
		{"not registered", serviceStatus{Registered: false}, serviceExitNotRegistered},
		{"registered inactive", serviceStatus{Registered: true, SupervisorState: "inactive"}, serviceExitDegraded},
		{"active unreachable", serviceStatus{Registered: true, SupervisorState: "active"}, serviceExitDegraded},
		{"active not ready", serviceStatus{Registered: true, SupervisorState: "active", HealthReachable: true}, serviceExitDegraded},
		{"active unauthorized", serviceStatus{Registered: true, SupervisorState: "active", HealthReachable: true, Ready: false}, serviceExitDegraded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusExitCode(tt.st); got != tt.want {
				t.Errorf("statusExitCode(%+v) = %d, want %d", tt.st, got, tt.want)
			}
		})
	}
}

func TestUnsupportedPlatform(t *testing.T) {
	p := unsupportedPlatform{}
	if p.name() != "unsupported" {
		t.Errorf("name = %q, want unsupported", p.name())
	}
	if err := p.usable(); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("usable() = %v, want unsupported error", err)
	}
	if _, err := p.renderDefinition(serviceSpec{}); err == nil {
		t.Error("renderDefinition should fail on unsupported platform")
	}
	if err := p.register(context.Background()); err == nil {
		t.Error("register should fail on unsupported platform")
	}
	if err := p.unregister(context.Background()); err == nil {
		t.Error("unregister should fail on unsupported platform")
	}
	if err := p.restartService(context.Background()); err == nil {
		t.Error("restartService should fail on unsupported platform")
	}
	if state, _ := p.supervisorState(context.Background()); state != "unsupported" {
		t.Errorf("supervisorState = %q, want unsupported", state)
	}
}

func TestInstallDefinitionWritesAndRollsBack(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)
	runner := &fakeRunner{fn: func(argv []string) (string, string, error) {
		if argv[1] == "bootstrap" {
			return "", "service already loaded", os.ErrNotExist
		}
		return "", "", nil
	}}
	platform := newLaunchdPlatform(runner)
	spec := serviceSpec{
		label:      platform.serviceID(),
		launcher:   []string{"/opt/homebrew/bin/node", "/opt/homebrew/lib/node_modules/@nano-step/nano-brain/npm/run.js"},
		configPath: "/home/user/.nano-brain/config.yml",
	}

	err := installDefinition(context.Background(), platform, spec)
	if err == nil || !strings.Contains(err.Error(), "bootstrap") {
		t.Fatalf("installDefinition err = %v, want bootstrap failure", err)
	}
	// Rollback: no previous definition existed → new file removed.
	if _, statErr := os.Stat(platform.definitionPath()); !os.IsNotExist(statErr) {
		t.Errorf("definition file should be rolled back after manager failure, stat err = %v", statErr)
	}
	// The definition directory itself may remain; no file should.
}

func TestInstallDefinitionRollbackRestoresPrevious(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)
	runner := &fakeRunner{fn: func(argv []string) (string, string, error) {
		return "", "bootstrap: service already loaded", os.ErrNotExist
	}}
	platform := newLaunchdPlatform(runner)
	spec := serviceSpec{
		label:      platform.serviceID(),
		launcher:   []string{"/bin/echo"},
		configPath: "/home/user/.nano-brain/config.yml",
	}

	prev := []byte("previous definition\n")
	defPath := platform.definitionPath()
	if err := os.MkdirAll(filepath.Dir(defPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defPath, prev, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installDefinition(context.Background(), platform, spec); err == nil {
		t.Fatal("installDefinition should fail when bootstrap fails")
	}
	got, err := os.ReadFile(defPath)
	if err != nil {
		t.Fatalf("previous definition should be restored: %v", err)
	}
	if string(got) != string(prev) {
		t.Errorf("restored content = %q, want %q", string(got), string(prev))
	}
}

func TestUninstallRemovesDefinitionAfterUnregister(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)
	runner := &fakeRunner{}
	platform := newLaunchdPlatform(runner)
	defPath := platform.definitionPath()
	if err := os.MkdirAll(filepath.Dir(defPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defPath, []byte("plist"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !registered(platform) {
		t.Fatal("platform should report registered when definition exists")
	}
	if err := platform.unregister(context.Background()); err != nil {
		t.Fatalf("unregister: %v", err)
	}
	cmds := runner.commandList()
	if len(cmds) != 1 || !strings.HasPrefix(cmds[0], "launchctl bootout gui/") {
		t.Errorf("unregister calls = %v, want launchctl bootout gui/<uid>/<label>", cmds)
	}
}
