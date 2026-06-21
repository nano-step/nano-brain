package graph

import (
	"regexp"
	"strings"

	zerolog "github.com/rs/zerolog"
)

type RubyCrossFileResolver struct {
	classIndex    *RubyClassIndex
	logger        zerolog.Logger
	useFallback   bool
}

func NewRubyCrossFileResolver(classIndex *RubyClassIndex, logger zerolog.Logger) *RubyCrossFileResolver {
	return &RubyCrossFileResolver{
		classIndex:  classIndex,
		logger:      logger,
		useFallback: true,
	}
}

func NewRubyCrossFileResolverNoFallback(classIndex *RubyClassIndex, logger zerolog.Logger) *RubyCrossFileResolver {
	return &RubyCrossFileResolver{
		classIndex:  classIndex,
		logger:      logger,
		useFallback: false,
	}
}

var rubyQualifiedCallRe = regexp.MustCompile(`([A-Z]\w*)\.(new)\.(\w+)`)
var rubyClassMethodRe = regexp.MustCompile(`([A-Z]\w*)\.(\w+)`)

func (r *RubyCrossFileResolver) ResolveEdges(edges []Edge, fileContents map[string][]byte) []Edge {
	var result []Edge
	seen := map[string]bool{}

	for _, e := range edges {
		result = append(result, e)
	}

	for filePath, content := range fileContents {
		newEdges := r.resolveFileCalls(filePath, content)
		for _, e := range newEdges {
			key := e.SourceNode + "->" + e.TargetNode
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, e)
		}
	}

	return result
}

func (r *RubyCrossFileResolver) resolveFileCalls(filePath string, content []byte) []Edge {
	var edges []Edge
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		if matches := rubyQualifiedCallRe.FindAllStringSubmatch(line, -1); len(matches) > 0 {
			for _, m := range matches {
				className := m[1]
				methodName := m[3]
				edges = append(edges, r.resolveCall(filePath, className, methodName, lineNum+1)...)
			}
			continue
		}

		if matches := rubyClassMethodRe.FindAllStringSubmatch(line, -1); len(matches) > 0 {
			for _, m := range matches {
				className := m[1]
				methodName := m[2]
				if methodName == "new" || methodName == "class" || methodName == "super" {
					continue
				}
				edges = append(edges, r.resolveCall(filePath, className, methodName, lineNum+1)...)
			}
		}
	}

	return edges
}

func (r *RubyCrossFileResolver) resolveCall(sourceFile, className, methodName string, line int) []Edge {
	var entries []classEntry
	if r.useFallback {
		entries = r.classIndex.Lookup(className)
	} else {
		entries = r.classIndex.LookupStrict(className)
	}

	if len(entries) == 0 {
		return []Edge{{
			SourceNode: sourceFile + "::" + methodName,
			TargetNode: methodName,
			Kind:       EdgeCalls,
			SourceFile: sourceFile,
			Line:       line,
			Language:   "ruby",
			Metadata:   map[string]any{"unresolved": true},
		}}
	}

	var edges []Edge
	ambiguous := len(entries) > 1
	for _, entry := range entries {
		e := Edge{
			SourceNode: sourceFile + "::" + methodName,
			TargetNode: entry.FilePath + "::" + methodName,
			Kind:       EdgeCalls,
			SourceFile: sourceFile,
			Line:       line,
			Language:   "ruby",
		}
		if ambiguous {
			e.Metadata = map[string]any{"ambiguous": true}
		}
		edges = append(edges, e)
	}
	return edges
}

func (r *RubyCrossFileResolver) BuildReconcileEdges(edges []Edge) []Edge {
	var result []Edge

	for _, e := range edges {
		if e.Kind != EdgeHTTP {
			continue
		}
		handler := e.TargetNode
		parts := strings.SplitN(handler, "#", 2)
		if len(parts) != 2 {
			continue
		}
		ctrlShort := parts[0]
		action := parts[1]

		// Try full namespaced name first (e.g., "Api::V1::TokensController"),
		// then fall back to short name (e.g., "TokensController").
		entries := r.classIndex.Lookup(ctrlShort)
		if len(entries) == 0 {
			ctrlName := ctrlShort
			if idx := strings.LastIndex(ctrlShort, "::"); idx >= 0 {
				ctrlName = ctrlShort[idx+2:]
			}
			entries = r.classIndex.Lookup(ctrlName)
		}
		if len(entries) == 0 {
			continue
		}

		for _, entry := range entries {
			result = append(result, Edge{
				SourceNode: handler,
				TargetNode: entry.FilePath + "::" + action,
				Kind:       EdgeReconcile,
				SourceFile: e.SourceFile,
				Line:       e.Line,
				Language:   "ruby",
			})
		}
	}

	return result
}
