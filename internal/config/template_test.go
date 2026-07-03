package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestRenderConfig_RoundTrip is the D-04 correctness anchor: it renders
// fullConfigTemplate with the same values getDefaults() uses for database
// URL and embedding, writes the result to a temp file, loads it via
// config.Load, and asserts the result equals Load("") (defaults loaded
// through the same koanf path, with no file on disk) for every field. This
// is the comparison that would catch koanf/yaml key drift between the
// hand-authored template and the Config struct's koanf tags — exactly the
// bug that ruled out yaml.Marshal(getDefaults()) (D-04).
//
// The baseline is Load(""), not getDefaults(), because koanf's
// structs.Provider normalizes nil slices/maps (e.g. Server.Auth.Users,
// Watcher.Workspaces) to non-nil empty values as part of loading defaults
// — a pre-existing Load() characteristic independent of any file being
// read, reproducible even with no config file on disk at all. Comparing
// against getDefaults() directly would flag that normalization as a false
// template-drift failure; comparing against Load("") isolates exactly what
// the rendered template changes relative to koanf's own default-loading
// path.
func TestRenderConfig_RoundTrip(t *testing.T) {
	defaults := getDefaults()

	rendered := RenderConfig(RenderOpts{
		DatabaseURL: defaults.Database.URL,
		EmbeddingBlock: "embedding:\n" +
			"  provider: " + defaults.Embedding.Provider + "\n" +
			"  url: " + defaults.Embedding.URL + "\n" +
			"  model: " + defaults.Embedding.Model + "\n" +
			"  concurrency: 3\n",
	})

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(rendered), 0600); err != nil {
		t.Fatalf("write rendered template: %v", err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load(rendered template) error = %v; rendered:\n%s", err, rendered)
	}

	want, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") (defaults baseline) error = %v", err)
	}

	if !reflect.DeepEqual(loaded, want) {
		t.Errorf("Load(rendered template) != Load(\"\") (defaults)\n got:  %+v\nwant: %+v", loaded, want)
	}
}

// TestRenderConfig_Substitutions verifies both substitution points land in
// the right place: DatabaseURL under database.url, and EmbeddingBlock
// nested correctly under the embedding: section (not top-level), with
// voyage_api_key remaining a sibling key inside that same section.
func TestRenderConfig_Substitutions(t *testing.T) {
	rendered := RenderConfig(RenderOpts{
		DatabaseURL:    "postgres://custom:pass@dbhost:5432/mydb",
		EmbeddingBlock: "embedding:\n  provider: \"\"\n",
	})

	if !strings.Contains(rendered, `url: "postgres://custom:pass@dbhost:5432/mydb"`) {
		t.Errorf("rendered config missing substituted (quoted) database URL:\n%s", rendered)
	}
	if !strings.Contains(rendered, "provider: \"\"") {
		t.Errorf("rendered config missing substituted embedding block:\n%s", rendered)
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(rendered), 0600); err != nil {
		t.Fatalf("write rendered template: %v", err)
	}
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load(BM25-only rendered template) error = %v; rendered:\n%s", err, rendered)
	}
	if loaded.Database.URL != "postgres://custom:pass@dbhost:5432/mydb" {
		t.Errorf("loaded.Database.URL = %q, want the substituted URL", loaded.Database.URL)
	}
	if loaded.Embedding.Provider != "" {
		t.Errorf("loaded.Embedding.Provider = %q, want empty (BM25-only)", loaded.Embedding.Provider)
	}
	if loaded.Embedding.VoyageAPIKey != "" {
		t.Errorf("loaded.Embedding.VoyageAPIKey = %q, want empty — no key should ever be written", loaded.Embedding.VoyageAPIKey)
	}
}

// TestRenderConfig_SpecialCharsInURL verifies a DSN whose password contains
// YAML-significant characters (#, @, :) round-trips intact — the database.url
// placeholder is double-quoted and RenderConfig escapes backslash/quote so the
// value can't break out of the quoted scalar or be truncated as a comment.
func TestRenderConfig_SpecialCharsInURL(t *testing.T) {
	dsn := `postgres://user:p#a@ss:w"rd\x@dbhost:5432/mydb?sslmode=disable`
	rendered := RenderConfig(RenderOpts{
		DatabaseURL:    dsn,
		EmbeddingBlock: "embedding:\n  provider: \"\"\n",
	})

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(rendered), 0600); err != nil {
		t.Fatalf("write rendered template: %v", err)
	}
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load(rendered with special-char URL) error = %v; rendered:\n%s", err, rendered)
	}
	if loaded.Database.URL != dsn {
		t.Errorf("loaded.Database.URL = %q, want %q (special chars must survive YAML round-trip)", loaded.Database.URL, dsn)
	}
}

// TestRenderConfig_NoSecretsWritten is the D-05 anchor: it scans the
// rendered template for known secret-bearing key names and asserts every
// occurrence is an empty string or a comment, never a real value.
func TestRenderConfig_NoSecretsWritten(t *testing.T) {
	rendered := RenderConfig(RenderOpts{
		DatabaseURL:    "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev",
		EmbeddingBlock: "embedding:\n  provider: ollama\n  url: http://localhost:11434\n  model: nomic-embed-text\n  concurrency: 3\n",
	})

	for _, line := range strings.Split(rendered, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "api_key:") || strings.Contains(trimmed, "voyage_api_key:") || strings.Contains(trimmed, "password_hash:") {
			if !strings.HasSuffix(trimmed, `""`) {
				t.Errorf("template line looks like it writes a non-empty secret: %q", trimmed)
			}
		}
	}
}
