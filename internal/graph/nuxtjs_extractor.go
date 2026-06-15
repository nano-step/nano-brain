package graph

import (
	"path/filepath"
	"strings"

	zerolog "github.com/rs/zerolog"
)

var _ Extractor = (*NuxtExtractor)(nil)

type NuxtExtractor struct {
	logger zerolog.Logger
}

func NewNuxtExtractor(logger zerolog.Logger) (*NuxtExtractor, error) {
	return &NuxtExtractor{
		logger: logger.With().Str("component", "nuxt-extractor").Logger(),
	}, nil
}

func (e *NuxtExtractor) Supports(ext string) bool {
	return ext == ".vue" || ext == ".ts" || ext == ".js"
}

func (e *NuxtExtractor) RequiresFrameworks() []string {
	return []string{"nuxt"}
}

func (e *NuxtExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	if !e.Supports(filepath.Ext(filePath)) {
		return nil, nil
	}
	path := filepath.ToSlash(filePath)

	var routeRelPath string
	var prefix string

	if strings.HasPrefix(path, "pages/") || strings.Contains(path, "/pages/") {
		prefix = "pages/"
		if idx := strings.LastIndex(path, "/pages/"); idx >= 0 {
			routeRelPath = path[idx+len("/pages/"):]
		} else if strings.HasPrefix(path, "pages/") {
			routeRelPath = path[len("pages/"):]
		}
	} else if strings.HasPrefix(path, "server/api/") || strings.Contains(path, "/server/api/") {
		prefix = "server/api/"
		if idx := strings.LastIndex(path, "/server/api/"); idx >= 0 {
			routeRelPath = path[idx+len("/server/api/"):]
		} else if strings.HasPrefix(path, "server/api/") {
			routeRelPath = path[len("server/api/"):]
		}
	}

	if routeRelPath == "" {
		return nil, nil
	}

	if prefix == "pages/" {
		edges := e.extractPageRoute(routeRelPath, path)
		if len(edges) == 0 {
			return nil, nil
		}
		return edges, nil
	}

	if prefix == "server/api/" {
		edges := e.extractAPIRoute(routeRelPath, path)
		if len(edges) == 0 {
			return nil, nil
		}
		return edges, nil
	}

	return nil, nil
}

func (e *NuxtExtractor) extractPageRoute(relPath string, fullPath string) []Edge {
	routePath := nuxtFilesystemPathToRoute(relPath)
	if routePath == "" {
		return nil
	}

	handlerName := "pages/" + normalizePath(relPath)

	return []Edge{
		{
			SourceNode: "GET " + routePath,
			TargetNode: handlerName,
			Kind:       EdgeHTTP,
			SourceFile: fullPath,
			Line:       0,
			Language:   parseLanguage(fullPath),
			Metadata: map[string]any{
				"method": "GET",
				"path":   routePath,
			},
		},
	}
}

func (e *NuxtExtractor) extractAPIRoute(relPath string, fullPath string) []Edge {
	method, routePart := parseNuxtAPIRoute(relPath)
	if method == "" || routePart == "" {
		return nil
	}

	routeSegments := nuxtPathToSegments(routePart)
	routePath := "/api" + buildRouteFromSegments(routeSegments)
	handlerName := "server/api/" + normalizePath(relPath)

	return []Edge{
		{
			SourceNode: method + " " + routePath,
			TargetNode: handlerName,
			Kind:       EdgeHTTP,
			SourceFile: fullPath,
			Line:       0,
			Language:   parseLanguage(fullPath),
			Metadata: map[string]any{
				"method": method,
				"path":   routePath,
			},
		},
	}
}

func nuxtFilesystemPathToRoute(relPath string) string {
	noExt := strings.TrimSuffix(relPath, filepath.Ext(relPath))
	segments := strings.Split(noExt, "/")

	var routeParts []string
	for _, seg := range segments {
		if seg == "index" {
			continue
		}
		routeParts = append(routeParts, nuxtSegmentToRouteParam(seg))
	}

	if len(routeParts) == 0 {
		return "/"
	}
	return "/" + strings.Join(routeParts, "/")
}

func nuxtSegmentToRouteParam(seg string) string {
	if strings.HasPrefix(seg, "[") && strings.HasSuffix(seg, "]") {
		inner := seg[1 : len(seg)-1]
		if strings.HasPrefix(inner, "...") {
			return "*" + strings.TrimPrefix(inner, "...")
		}
		return ":" + inner
	}
	return seg
}

func parseNuxtAPIRoute(relPath string) (method string, routePart string) {
	noExt := strings.TrimSuffix(relPath, filepath.Ext(relPath))

	lastDot := strings.LastIndex(noExt, ".")
	if lastDot < 0 {
		return "", ""
	}

	method = strings.ToUpper(noExt[lastDot+1:])
	if !httpVerbs[method] {
		return "", ""
	}

	routePart = noExt[:lastDot]
	return method, routePart
}

func nuxtPathToSegments(relPath string) []string {
	segments := strings.Split(relPath, "/")
	var result []string
	for _, seg := range segments {
		converted := nuxtSegmentToRouteParam(seg)
		result = append(result, converted)
	}
	return result
}

func buildRouteFromSegments(segments []string) string {
	if len(segments) == 0 {
		return ""
	}
	return "/" + strings.Join(segments, "/")
}

func normalizePath(p string) string {
	return filepath.ToSlash(p)
}

func parseLanguage(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".vue":
		return "vue"
	case ".ts":
		return "typescript"
	case ".js":
		return "javascript"
	default:
		return ""
	}
}
