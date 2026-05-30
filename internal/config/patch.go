package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// PatchableFieldPaths lists every dot-path that the config PATCH endpoint
// accepts. Fields not in this list are rejected.
var PatchableFieldPaths = []string{
	"server.host",
	"server.port",
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
	return nil
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
