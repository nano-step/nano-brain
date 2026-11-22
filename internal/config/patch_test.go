package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const patchTestYAML = `# kept comment
server:
  host: localhost
  port: 3199
  allowed_hosts: []
embedding:
  url: http://localhost:11434
search:
  rrf_k: 60
  limit: 20
summarization:
  enabled: false
`

func writePatchTestConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(patchTestYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestApplyPatch_TypeCheck(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		value   string // JSON
		wantErr string // empty: the patch is written
	}{
		{"string field, string", "server.host", `"127.0.0.1"`, ""},
		{"string field, number", "server.host", `1`, `field "server.host" expects a string, got a number`},
		{"string field, null", "embedding.url", `null`, `field "embedding.url" expects a string, got null`},
		{"int field, integer", "server.port", `3198`, ""},
		{"int field, huge", "server.port", `1e300`, `field "server.port" expects an integer, got a number`},
		{"int field, past int64", "server.port", `9223372036854775807`, `field "server.port" expects an integer, got a number`},
		{"int field, string", "server.port", `"3198"`, `field "server.port" expects an integer, got a string`},
		{"int field, fraction", "search.limit", `2.5`, `field "search.limit" expects an integer, got a number`},
		{"int field, bool", "search.limit", `true`, `field "search.limit" expects an integer, got a boolean`},
		{"float field, fraction", "search.rrf_k", `61.5`, ""},
		{"float field, string", "search.rrf_k", `"61"`, `field "search.rrf_k" expects a number, got a string`},
		{"bool field, bool", "summarization.enabled", `false`, ""},
		{"bool field, string", "summarization.enabled", `"false"`, `field "summarization.enabled" expects a boolean, got a string`},
		{"list field, strings", "server.allowed_hosts", `["brain.example","localhost"]`, ""},
		{"list field, empty", "server.allowed_hosts", `[]`, ""},
		{"list field, object", "server.allowed_hosts", `{"a":"b"}`, `field "server.allowed_hosts" expects an array of strings, got an object`},
		{"list field, string", "server.allowed_hosts", `"brain.example"`, `field "server.allowed_hosts" expects an array of strings, got a string`},
		{"list field, mixed array", "server.allowed_hosts", `["brain.example",1]`, `field "server.allowed_hosts" expects an array of strings, got an array`},
		{"list field, nested object", "server.allowed_hosts", `[{"a":"b"}]`, `field "server.allowed_hosts" expects an array of strings, got an array`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writePatchTestConfig(t)
			var value interface{}
			if err := json.Unmarshal([]byte(tt.value), &value); err != nil {
				t.Fatal(err)
			}

			err := ApplyPatch(path, PatchRequest{Path: tt.path, Value: value})
			after, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ApplyPatch() failed: %v", err)
				}
				if string(after) == patchTestYAML {
					t.Error("ApplyPatch() succeeded but the file is unchanged")
				}
				if _, err := Load(path); err != nil {
					t.Errorf("patched file no longer loads: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ApplyPatch() error = %v, want %q", err, tt.wantErr)
			}
			if string(after) != patchTestYAML {
				t.Errorf("rejected patch changed the file:\n%s", after)
			}
		})
	}
}

// Every patchable path must resolve to a Config field of a kind the type
// check understands; otherwise ApplyPatch rejects every value for it.
func TestPatchableFieldPaths_AllHaveACheckableType(t *testing.T) {
	samples := map[string]interface{}{
		"a string": "x", "a boolean": true, "a number": float64(1), "an array": []interface{}{"x"},
	}
	for _, path := range PatchableFieldPaths {
		t.Run(path, func(t *testing.T) {
			accepted := 0
			for _, value := range samples {
				if checkPatchValueType(path, value) == nil {
					accepted++
				}
			}
			if accepted != 1 {
				t.Errorf("%d of the sample JSON types are accepted for %s, want exactly 1", accepted, path)
			}
		})
	}
}

func TestApplyPatch_RollsBackWhenTheConfigNoLongerLoads(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		value   interface{}
		wantErr string
	}{
		{"port zero", "server.port", float64(0), "server.port must be between 1 and 65535"},
		{"port too high", "server.port", float64(70000), "server.port must be between 1 and 65535"},
		{"unknown log level", "logging.level", "loud", `logging.level "loud" is not valid`},
		{"search limit zero", "search.limit", float64(0), "search.limit must be >= 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writePatchTestConfig(t)
			if tt.path == "logging.level" {
				withLevel := patchTestYAML + "logging:\n  level: info\n"
				if err := os.WriteFile(path, []byte(withLevel), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			err = ApplyPatch(path, PatchRequest{Path: tt.path, Value: tt.value})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ApplyPatch() error = %v, want it to contain %q", err, tt.wantErr)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Errorf("invalid patch was not rolled back:\n%s", after)
			}
		})
	}
}

func TestApplyPatch_ConcurrentRollbackDoesNotOverwriteValidPatch(t *testing.T) {
	path := writePatchTestConfig(t)
	for i := 0; i < 20; i++ {
		var wg sync.WaitGroup
		for j := 0; j < 8; j++ {
			wg.Add(1)
			go func(valid bool) {
				defer wg.Done()
				if valid {
					if err := ApplyPatch(path, PatchRequest{Path: "search.limit", Value: float64(25)}); err != nil {
						t.Errorf("valid patch: %v", err)
					}
				} else if err := ApplyPatch(path, PatchRequest{Path: "server.port", Value: float64(0)}); err == nil {
					t.Error("invalid patch succeeded")
				}
			}(j%2 == 0)
		}
		wg.Wait()
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Server.Port != 3199 || cfg.Search.Limit != 25 {
			t.Fatalf("iteration %d: port=%d limit=%d", i, cfg.Server.Port, cfg.Search.Limit)
		}
	}
}
