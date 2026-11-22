package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/health/doctor"
)

func TestPadRight(t *testing.T) {
	if got := padRight("abc", 6); got != "abc..." {
		t.Errorf("padRight got %q, want %q", got, "abc...")
	}
	if got := padRight("toolong", 4); got != "toolong" {
		t.Errorf("padRight should not truncate, got %q", got)
	}
	if got := padRight("exact", 5); got != "exact" {
		t.Errorf("padRight exact len got %q, want %q", got, "exact")
	}
}

func TestCheckResultJSONFormat(t *testing.T) {
	results := []doctor.Check{
		{Name: "Config", Status: "ok", Detail: "/home/user/.nano-brain/config.yml"},
		{Name: "PostgreSQL", Status: "fail", Detail: "localhost:5432", Hint: "Is PostgreSQL running?"},
	}
	out := struct {
		Checks    []doctor.Check `json:"checks"`
		AllPassed bool           `json:"all_passed"`
	}{results, false}

	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded struct {
		Checks    []doctor.Check `json:"checks"`
		AllPassed bool           `json:"all_passed"`
	}
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.AllPassed {
		t.Error("all_passed should be false")
	}
	if len(decoded.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(decoded.Checks))
	}
	if decoded.Checks[1].Status != "fail" {
		t.Errorf("second check status should be fail, got %q", decoded.Checks[1].Status)
	}
	if decoded.Checks[1].Hint != "Is PostgreSQL running?" {
		t.Errorf("hint mismatch: %q", decoded.Checks[1].Hint)
	}
}

func TestCheckEmbeddingModelOllamaMatch(t *testing.T) {
	body := []byte(`{"models":[{"name":"nomic-embed-text:latest"},{"name":"llama2"}]}`)

	cfg := struct {
		Provider string
		Model    string
	}{Provider: "ollama", Model: "nomic-embed-text"}

	var resp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	found := false
	for _, m := range resp.Models {
		name := strings.TrimSuffix(m.Name, ":latest")
		if name == cfg.Model || m.Name == cfg.Model {
			found = true
		}
	}
	if !found {
		t.Errorf("expected model %q to be found in ollama response", cfg.Model)
	}
}

func TestCheckConfigMissingPath(t *testing.T) {
	r := doctor.CheckConfig("/nonexistent/path/config.yml", &json.SyntaxError{})
	if r.Status != "fail" {
		t.Errorf("expected fail for missing config, got %q", r.Status)
	}
	if r.Name != "Config" {
		t.Errorf("expected name Config, got %q", r.Name)
	}
}

func TestCheckConfigSuccess(t *testing.T) {
	r := doctor.CheckConfig("/home/user/.nano-brain/config.yml", nil)
	if r.Status != "ok" {
		t.Errorf("expected ok for valid config, got %q", r.Status)
	}
}

func TestDoctor_MissingExplicitConfigFailsAndWarns(t *testing.T) {
	findConfigCheck := func(t *testing.T, stdout string) doctor.Check {
		t.Helper()
		var parsed struct {
			Checks    []doctor.Check `json:"checks"`
			AllPassed bool           `json:"all_passed"`
		}
		if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
		}
		for _, c := range parsed.Checks {
			if c.Name == "Config" {
				return c
			}
		}
		t.Fatalf("Config check not found in %+v", parsed.Checks)
		return doctor.Check{}
	}

	t.Run("NANO_BRAIN_CONFIG missing reports Config FAIL and emits WARNING on stderr", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "does-not-exist.yml")

		stdout, stderr, code := invokeCLI(t, map[string]string{
			"HOME":                          dir,
			"NANO_BRAIN_CONFIG":             missing,
			"DATABASE_URL":                  "",
			"NANO_BRAIN_EMBEDDING_PROVIDER": "",
		}, "doctor", "--json")

		if code == 0 {
			t.Errorf("exit code = 0, want non-zero when explicit config is missing")
		}
		if !strings.Contains(stderr, "WARNING:") || !strings.Contains(stderr, "NANO_BRAIN_CONFIG") || !strings.Contains(stderr, missing) {
			t.Errorf("stderr = %q, want WARNING mentioning NANO_BRAIN_CONFIG and %q", stderr, missing)
		}
		cfgCheck := findConfigCheck(t, stdout)
		if cfgCheck.Status != "fail" {
			t.Errorf("Config check status = %q, want \"fail\" (got %+v)", cfgCheck.Status, cfgCheck)
		}
		if !strings.Contains(cfgCheck.Hint, "file does not exist") && !strings.Contains(cfgCheck.Detail, "file does not exist") {
			t.Errorf("Config check missing 'file does not exist' message: %+v", cfgCheck)
		}
	})

	t.Run("explicit --config missing reports Config FAIL and emits WARNING on stderr", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "flag-does-not-exist.yml")

		stdout, stderr, code := invokeCLI(t, map[string]string{
			"HOME":                          dir,
			"NANO_BRAIN_CONFIG":             "",
			"DATABASE_URL":                  "",
			"NANO_BRAIN_EMBEDDING_PROVIDER": "",
		}, "--config", missing, "doctor")

		if code == 0 {
			t.Errorf("exit code = 0, want non-zero when --config file is missing")
		}
		if !strings.Contains(stderr, "WARNING:") || !strings.Contains(stderr, "--config flag") || !strings.Contains(stderr, missing) {
			t.Errorf("stderr = %q, want WARNING mentioning --config flag and %q", stderr, missing)
		}
		if !strings.Contains(stdout, "FAIL") || !strings.Contains(stdout, "file does not exist") {
			t.Errorf("stdout = %q, want Config FAIL with 'file does not exist'", stdout)
		}
	})

	t.Run("unset env and missing default path still reports Config OK with empty stderr", func(t *testing.T) {
		dir := t.TempDir()

		stdout, stderr, _ := invokeCLI(t, map[string]string{
			"HOME":                          dir,
			"NANO_BRAIN_CONFIG":             "",
			"DATABASE_URL":                  "",
			"NANO_BRAIN_EMBEDDING_PROVIDER": "",
		}, "doctor", "--json")

		if stderr != "" {
			t.Errorf("stderr = %q, want empty when neither --config nor NANO_BRAIN_CONFIG is set", stderr)
		}
		cfgCheck := findConfigCheck(t, stdout)
		if cfgCheck.Status != "ok" {
			t.Errorf("Config check status = %q, want \"ok\" for zero-config default (got %+v)", cfgCheck.Status, cfgCheck)
		}
	})
}

func TestDoctor_TrimsConfigFlagWhitespace(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "valid-config.yml")
	if err := os.WriteFile(cfgPath, []byte("server:\n  port: 3199\nembedding:\n  provider: \"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, _ := invokeCLI(t, map[string]string{
		"HOME":              dir,
		"NANO_BRAIN_CONFIG": "",
		"DATABASE_URL":      "",
	}, "--config", "  "+cfgPath+"  ", "doctor", "--json")

	if stderr != "" {
		t.Errorf("stderr = %q, want empty for valid padded --config path", stderr)
	}
	var parsed struct {
		Checks []doctor.Check `json:"checks"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	for _, c := range parsed.Checks {
		if c.Name == "Config" {
			if c.Status != "ok" {
				t.Errorf("Config status = %q, want \"ok\" (check=%+v)", c.Status, c)
			}
			if c.Detail != cfgPath {
				t.Errorf("Config detail = %q, want trimmed %q", c.Detail, cfgPath)
			}
			return
		}
	}
	t.Fatal("Config check not found")
}
