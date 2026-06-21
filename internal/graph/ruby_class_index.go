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
	if entries, ok := idx.byShort[className]; ok && len(entries) > 0 {
		return entries
	}

	if dotIdx := strings.LastIndex(className, "::"); dotIdx >= 0 {
		shortName := className[dotIdx+2:]
		if entries, ok := idx.byShort[shortName]; ok && len(entries) > 0 {
			return entries
		}
	}

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
	if entries, ok := idx.byShort[className]; ok && len(entries) > 0 {
		return entries
	}

	if dotIdx := strings.LastIndex(className, "::"); dotIdx >= 0 {
		shortName := className[dotIdx+2:]
		if entries, ok := idx.byShort[shortName]; ok && len(entries) > 0 {
			return entries
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
	snake := CamelToSnake(className)
	return "app/models/" + snake + ".rb"
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
