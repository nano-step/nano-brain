package config

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var patchMu sync.Mutex

// PatchableFieldPaths lists every dot-path that the config PATCH endpoint
// accepts. Fields not in this list are rejected.
var PatchableFieldPaths = []string{
	"server.host",
	"server.port",
	"server.allowed_hosts",
	"embedding.provider",
	"embedding.url",
	"embedding.model",
	"embedding.dimension",
	"embedding.concurrency",
	"search.rrf_k",
	"search.recency_weight",
	"search.recency_half_life_days",
	"search.limit",
	"watcher.debounce_ms",
	"watcher.reindex_interval",
	"storage.max_file_size",
	"storage.max_size",
	"telemetry.retention_days",
	"logging.level",
	"logging.file",
	"summarization.enabled",
	"summarization.provider_url",
	"summarization.model",
	"summarization.max_tokens",
	"summarization.concurrency",
}

func IsPatchableFieldPath(path string) bool {
	for _, p := range PatchableFieldPaths {
		if p == path {
			return true
		}
	}
	return false
}

// PatchRequest is the JSON body for a single config patch.
type PatchRequest struct {
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

// ApplyPatch reads the YAML config at cfgPath, applies the patch, and writes
// back preserving comments and key order via yaml.v3 Node manipulation.
func ApplyPatch(cfgPath string, patch PatchRequest) error {
	if IsSecretFieldPath(patch.Path) {
		return fmt.Errorf("field %q is a secret and cannot be patched via API", patch.Path)
	}
	if !IsPatchableFieldPath(patch.Path) {
		return fmt.Errorf("field %q is not patchable", patch.Path)
	}

	if err := checkPatchValueType(patch.Path, patch.Value); err != nil {
		return err
	}

	// Keep read/validate/write/rollback one transaction for concurrent API patches.
	patchMu.Lock()
	defer patchMu.Unlock()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse config YAML: %w", err)
	}

	parts := strings.Split(patch.Path, ".")
	if err := setNodeValue(&doc, parts, patch.Value); err != nil {
		return fmt.Errorf("set %q: %w", patch.Path, err)
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(cfgPath, out, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	// A value of the right type can still be one the server refuses to start
	// with (server.port: 0), so the patched file is loaded before it is kept.
	if _, loadErr := Load(cfgPath); loadErr != nil {
		if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
			return fmt.Errorf("patched config is invalid (%v) and restoring the previous file failed: %w", loadErr, err)
		}
		return fmt.Errorf("patch rejected, config file left unchanged: %w", loadErr)
	}
	return nil
}

// configFieldType returns the Go type of the Config field at the koanf
// dot-path, found by walking the struct tags.
func configFieldType(path string) (reflect.Type, bool) {
	t := reflect.TypeOf(Config{})
	for _, part := range strings.Split(path, ".") {
		if t.Kind() != reflect.Struct {
			return nil, false
		}
		found := false
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).Tag.Get("koanf") == part {
				t, found = t.Field(i).Type, true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	return t, true
}

// checkPatchValueType rejects a decoded JSON value whose type does not match
// the Config field at path.
func checkPatchValueType(path string, value interface{}) error {
	t, ok := configFieldType(path)
	if !ok {
		return fmt.Errorf("field %q does not exist in the config", path)
	}
	var want string
	switch t.Kind() {
	case reflect.String:
		want = "a string"
		if _, ok := value.(string); ok {
			return nil
		}
	case reflect.Bool:
		want = "a boolean"
		if _, ok := value.(bool); ok {
			return nil
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		want = "an integer"
		if n, ok := value.(float64); ok && n == math.Trunc(n) {
			return nil
		}
	case reflect.Float32, reflect.Float64:
		want = "a number"
		if _, ok := value.(float64); ok {
			return nil
		}
	case reflect.Slice:
		if t.Elem().Kind() != reflect.String {
			return fmt.Errorf("field %q has a type that cannot be patched", path)
		}
		want = "an array of strings"
		if items, ok := value.([]interface{}); ok {
			allStrings := true
			for _, item := range items {
				if _, ok := item.(string); !ok {
					allStrings = false
				}
			}
			if allStrings {
				return nil
			}
		}
	default:
		return fmt.Errorf("field %q has a type that cannot be patched", path)
	}
	return fmt.Errorf("field %q expects %s, got %s", path, want, jsonTypeName(value))
}

// jsonTypeName names the JSON type of a value decoded by encoding/json.
func jsonTypeName(value interface{}) string {
	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "a string"
	case bool:
		return "a boolean"
	case float64:
		return "a number"
	case []interface{}:
		return "an array"
	case map[string]interface{}:
		return "an object"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func setNodeValue(doc *yaml.Node, parts []string, value interface{}) error {
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return fmt.Errorf("empty YAML document")
		}
		return setNodeValue(doc.Content[0], parts, value)
	}
	if doc.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node, got %d", doc.Kind)
	}

	key := parts[0]
	for i := 0; i < len(doc.Content)-1; i += 2 {
		if doc.Content[i].Value == key {
			if len(parts) == 1 {
				valBytes, err := json.Marshal(value)
				if err != nil {
					return err
				}
				var yamlVal yaml.Node
				if err := yaml.Unmarshal(valBytes, &yamlVal); err != nil {
					return err
				}
				if yamlVal.Kind == yaml.DocumentNode && len(yamlVal.Content) > 0 {
					*doc.Content[i+1] = *yamlVal.Content[0]
				} else {
					*doc.Content[i+1] = yamlVal
				}
				return nil
			}
			return setNodeValue(doc.Content[i+1], parts[1:], value)
		}
	}
	return fmt.Errorf("key %q not found in YAML", key)
}
