package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

func TestInitCodexHarvesters_Disabled(t *testing.T) {
	chs := initCodexHarvesters(context.Background(), config.CodexHarvesterConfig{Enabled: false}, nil, zerolog.Nop())
	if len(chs) != 0 {
		t.Error("expected no harvesters when disabled")
	}
}

func TestInitCodexHarvesters_NoSessionDirs(t *testing.T) {
	home := t.TempDir()
	chs := initCodexHarvesters(context.Background(),
		config.CodexHarvesterConfig{Enabled: true, CodexHome: home}, nil, zerolog.Nop())
	if len(chs) != 0 {
		t.Error("expected no harvesters when neither sessions/ nor archived_sessions/ exists")
	}
}

func TestInitCodexHarvesters_SessionsDir(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "sessions", "2026", "11", "20"), 0o755); err != nil {
		t.Fatal(err)
	}
	chs := initCodexHarvesters(context.Background(),
		config.CodexHarvesterConfig{Enabled: true, CodexHome: home}, nil, zerolog.Nop())
	if len(chs) != 1 {
		t.Fatalf("expected 1 harvester, got %d", len(chs))
	}
}

func TestInitCodexHarvesters_ArchivedOnly(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "archived_sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	chs := initCodexHarvesters(context.Background(),
		config.CodexHarvesterConfig{Enabled: true, CodexHome: home}, nil, zerolog.Nop())
	if len(chs) != 1 {
		t.Fatalf("expected 1 harvester from archived_sessions only, got %d", len(chs))
	}
}

func TestResolveCodexHome(t *testing.T) {
	t.Run("config wins", func(t *testing.T) {
		t.Setenv("CODEX_HOME", "/env/codex")
		t.Setenv("HOME", "/home/u")
		if got := resolveCodexHome("/cfg/codex"); got != "/cfg/codex" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("env second", func(t *testing.T) {
		t.Setenv("CODEX_HOME", "/env/codex")
		t.Setenv("HOME", "/home/u")
		if got := resolveCodexHome(""); got != "/env/codex" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("home fallback", func(t *testing.T) {
		t.Setenv("CODEX_HOME", "")
		t.Setenv("HOME", "/home/u")
		want := filepath.Join("/home/u", ".codex")
		if got := resolveCodexHome(""); got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})
}
