package graph

import (
	"strings"
)

type classEntry struct {
	FilePath  string
	ShortName string
}

type RubyClassIndex struct {
	byShort map[string][]classEntry
}

func BuildClassIndex(edges []Edge) *RubyClassIndex {
	idx := &RubyClassIndex{
		byShort: make(map[string][]classEntry),
	}

	for _, e := range edges {
		if e.Kind != EdgeContains {
			continue
		}
		shortName := containsSuffix(e.TargetNode)
		if shortName == "" || !isRubyClassName(shortName) {
			continue
		}
		filePath := e.SourceNode
		idx.byShort[shortName] = append(idx.byShort[shortName], classEntry{
			FilePath:  filePath,
			ShortName: shortName,
		})
	}

	return idx
}

func (idx *RubyClassIndex) Lookup(className string) []classEntry {
	// 1. Exact short-name match (e.g., "TokensController")
	if entries, ok := idx.byShort[className]; ok && len(entries) > 0 {
		return preferController(entries)
	}

	// 2. Namespaced lookup (e.g., "Api::V1::TokensController")
	if dotIdx := strings.LastIndex(className, "::"); dotIdx >= 0 {
		shortName := className[dotIdx+2:]
		namespace := className[:dotIdx]
		if entries, ok := idx.byShort[shortName]; ok && len(entries) > 0 {
			return preferByNamespace(entries, namespace)
		}
	}

	// 3. Rails convention fallback
	if isRubyClassName(className) {
		fallbackPath := railsConventionPath(className)
		if fallbackPath != "" {
			return []classEntry{{
				FilePath:  fallbackPath,
				ShortName: className,
			}}
		}
	}

	return nil
}

func (idx *RubyClassIndex) LookupStrict(className string) []classEntry {
	// 1. Exact short-name match
	if entries, ok := idx.byShort[className]; ok && len(entries) > 0 {
		return preferController(entries)
	}

	// 2. Namespaced lookup
	if dotIdx := strings.LastIndex(className, "::"); dotIdx >= 0 {
		shortName := className[dotIdx+2:]
		namespace := className[:dotIdx]
		if entries, ok := idx.byShort[shortName]; ok && len(entries) > 0 {
			return preferByNamespace(entries, namespace)
		}
	}

	return nil
}

func containsSuffix(targetNode string) string {
	idx := strings.LastIndex(targetNode, "::")
	if idx < 0 {
		return ""
	}
	return targetNode[idx+2:]
}

func isRubyClassName(name string) bool {
	if len(name) == 0 {
		return false
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}

func railsConventionPath(className string) string {
	// Strip namespace prefix (e.g., "Admin::UsersController" → "UsersController")
	shortName := className
	namespace := ""
	if dotIdx := strings.LastIndex(className, "::"); dotIdx >= 0 {
		shortName = className[dotIdx+2:]
		namespace = className[:dotIdx]
	}
	snake := CamelToSnake(shortName)
	if strings.HasSuffix(shortName, "Controller") {
		if namespace != "" {
			return "app/controllers/" + namespaceToPath(namespace) + "/" + snake + ".rb"
		}
		return "app/controllers/" + snake + ".rb"
	}
	if namespace != "" {
		return "app/models/" + namespaceToPath(namespace) + "/" + snake + ".rb"
	}
	return "app/models/" + snake + ".rb"
}

// namespaceToPath converts a Ruby namespace to a filesystem path segment.
// e.g., "Api::V1" → "api/v1"
func namespaceToPath(namespace string) string {
	parts := strings.Split(namespace, "::")
	for i, p := range parts {
		parts[i] = CamelToSnake(p)
	}
	return strings.Join(parts, "/")
}

// preferController reorders entries so that app/controllers/ paths come first.
// When multiple classes share a short name (e.g., model + controller), the controller wins.
func preferController(entries []classEntry) []classEntry {
	if len(entries) <= 1 {
		return entries
	}
	var controllers, others []classEntry
	for _, e := range entries {
		if strings.Contains(e.FilePath, "app/controllers/") {
			controllers = append(controllers, e)
		} else {
			others = append(others, e)
		}
	}
	if len(controllers) > 0 {
		return append(controllers, others...)
	}
	return entries
}

// preferByNamespace filters entries to those whose file path matches the given
// namespace. If no namespace match exists, falls back to preferController ordering.
func preferByNamespace(entries []classEntry, namespace string) []classEntry {
	if len(entries) <= 1 {
		return entries
	}
	nsPath := namespaceToPath(namespace)
	var namespaceMatch []classEntry
	for _, e := range entries {
		if strings.Contains(e.FilePath, nsPath) {
			namespaceMatch = append(namespaceMatch, e)
		}
	}
	if len(namespaceMatch) > 0 {
		return namespaceMatch
	}
	return preferController(entries)
}

func CamelToSnake(s string) string {
	var result []rune
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				prev := rune(s[i-1])
				if prev >= 'a' && prev <= 'z' {
					result = append(result, '_')
				} else if prev >= 'A' && prev <= 'Z' && i+1 < len(s) {
					next := rune(s[i+1])
					if next >= 'a' && next <= 'z' {
						result = append(result, '_')
					}
				}
			}
			result = append(result, r+32)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}
