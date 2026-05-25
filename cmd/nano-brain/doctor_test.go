package main

import (
	"encoding/json"
	"strings"
	"testing"
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
	results := []checkResult{
		{Name: "Config", Status: "ok", Detail: "/home/user/.nano-brain/config.yml"},
		{Name: "PostgreSQL", Status: "fail", Detail: "localhost:5432", Hint: "Is PostgreSQL running?"},
	}
	out := struct {
		Checks    []checkResult `json:"checks"`
		AllPassed bool          `json:"all_passed"`
	}{results, false}

	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded struct {
		Checks    []checkResult `json:"checks"`
		AllPassed bool          `json:"all_passed"`
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
	r := checkConfig("/nonexistent/path/config.yml", &json.SyntaxError{})
	if r.Status != "fail" {
		t.Errorf("expected fail for missing config, got %q", r.Status)
	}
	if r.Name != "Config" {
		t.Errorf("expected name Config, got %q", r.Name)
	}
}

func TestCheckConfigSuccess(t *testing.T) {
	r := checkConfig("/home/user/.nano-brain/config.yml", nil)
	if r.Status != "ok" {
		t.Errorf("expected ok for valid config, got %q", r.Status)
	}
}
