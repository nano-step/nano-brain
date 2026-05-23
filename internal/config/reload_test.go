package config

import (
	"os"
	"path/filepath"
	"testing"
)

func validDefaults() *Config {
	return getDefaults()
}

func writeYAML(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	return p
}

func TestReloadNoChanges(t *testing.T) {
	dir := t.TempDir()
	current := validDefaults()
	path := writeYAML(t, dir, "")

	_, result, err := Reload(path, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Reloaded) != 0 {
		t.Errorf("expected 0 reloaded, got %v", result.Reloaded)
	}
	if len(result.RequiresRestart) != 0 {
		t.Errorf("expected 0 requires_restart, got %v", result.RequiresRestart)
	}
	if len(result.Unchanged) == 0 {
		t.Error("expected non-empty unchanged list")
	}
}

func TestReloadReloadableChange(t *testing.T) {
	dir := t.TempDir()
	current := validDefaults()
	path := writeYAML(t, dir, `search:
  rrf_k: 80
  recency_weight: 0.3
  recency_half_life_days: 180
  limit: 20
`)

	newCfg, result, err := Reload(path, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newCfg.Search.RrfK != 80 {
		t.Errorf("expected new RrfK=80, got %v", newCfg.Search.RrfK)
	}
	found := false
	for _, s := range result.Reloaded {
		if s == "search.rrf_k" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected search.rrf_k in reloaded, got %v", result.Reloaded)
	}
	if len(result.RequiresRestart) != 0 {
		t.Errorf("expected 0 requires_restart, got %v", result.RequiresRestart)
	}
}

func TestReloadNonReloadableChange(t *testing.T) {
	dir := t.TempDir()
	current := validDefaults()
	path := writeYAML(t, dir, `server:
  host: "0.0.0.0"
  port: 3100
`)

	_, result, err := Reload(path, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, s := range result.RequiresRestart {
		if s == "server.host" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected server.host in requires_restart, got %v", result.RequiresRestart)
	}
}

func TestReloadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	current := validDefaults()
	path := writeYAML(t, dir, `server:
  port: -1
`)

	_, _, err := Reload(path, current)
	if err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
}

func TestReloadMixedChanges(t *testing.T) {
	dir := t.TempDir()
	current := validDefaults()
	path := writeYAML(t, dir, `server:
  host: "0.0.0.0"
  port: 3100
search:
  rrf_k: 100
  recency_weight: 0.5
  recency_half_life_days: 180
  limit: 20
logging:
  level: "debug"
`)

	_, result, err := Reload(path, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	restartSet := make(map[string]bool)
	for _, s := range result.RequiresRestart {
		restartSet[s] = true
	}
	reloadedSet := make(map[string]bool)
	for _, s := range result.Reloaded {
		reloadedSet[s] = true
	}

	if !restartSet["server.host"] {
		t.Errorf("server.host should require restart")
	}
	if !reloadedSet["search.rrf_k"] {
		t.Errorf("search.rrf_k should be reloaded")
	}
	if !reloadedSet["search.recency_weight"] {
		t.Errorf("search.recency_weight should be reloaded")
	}
	if !reloadedSet["logging.level"] {
		t.Errorf("logging.level should be reloaded")
	}
}

func TestReloadFileNotFound(t *testing.T) {
	current := validDefaults()
	path := "/nonexistent/path/config.yml"

	_, result, err := Reload(path, current)
	if err != nil {
		t.Fatalf("file-not-found should not error (defaults used): %v", err)
	}
	if len(result.Reloaded) != 0 {
		t.Errorf("expected no reloaded changes for missing file, got %v", result.Reloaded)
	}
}
