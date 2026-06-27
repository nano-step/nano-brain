package graph

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

// AliasMap maps an import alias prefix (e.g. "~", "@") to a workspace-relative
// base directory (e.g. "" for the workspace root, "src" for a src layout).
// Resolution rewrites "<alias>/rest" to "<base>/rest" before extension probing.
type AliasMap map[string]string

// importProbeExtensions are tried, in order, against a bare specifier path.
// Order matters: a .ts file wins over a same-named .js file (TS-first repos).
var importProbeExtensions = []string{".ts", ".tsx", ".js", ".jsx", ".vue", ".mjs", ".cjs"}

// importIndexExtensions are tried for "<spec>/index.<ext>" directory imports.
var importIndexExtensions = []string{".ts", ".tsx", ".js", ".jsx", ".vue"}

// IsRelativeImport reports whether spec is a relative module specifier
// ("./x", "../x"). These resolve against the importing file's directory.
func IsRelativeImport(spec string) bool {
	return strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") ||
		spec == "." || spec == ".."
}

// IsAliasImport reports whether spec uses a path alias ("~/x", "~x", "@/x").
// A scoped npm package ("@org/pkg") is NOT an alias: the "@/" alias rule only
// matches when the character after "@" is "/", not a package-scope word.
func IsAliasImport(spec string) bool {
	if strings.HasPrefix(spec, "~") {
		return true
	}
	// "@/..." is the Vite/Nuxt root alias; "@org/..." is a scoped npm package.
	return strings.HasPrefix(spec, "@/")
}

// IsBarePackage reports whether spec is a bare/scoped npm package specifier
// ("lodash", "lodash/fp", "@org/pkg", "@org/pkg/sub") — i.e. not relative and
// not an alias. Bare packages are passed through without filesystem resolution.
func IsBarePackage(spec string) bool {
	return !IsRelativeImport(spec) && !IsAliasImport(spec)
}

// ResolveImportPath resolves an import specifier to a workspace-relative target
// file (forward-slashed), so import edges byte-match the stored source_node form
// of the imported file.
//
//   - relative ("./x", "../x") → joined against dir(sourceFile), then probed
//   - alias ("~/x", "@/x")      → prefix mapped via alias, then probed
//   - bare/scoped package       → returned unchanged (no filesystem resolution)
//
// sourceFile is workspace-relative (forward-slashed). workspaceRoot is the
// absolute workspace directory (used only by the prod exists closure). exists
// receives a workspace-relative, forward-slashed path and reports whether that
// file is on disk.
//
// On any miss or ambiguity the RAW spec is returned (never an extensionless
// half-path, which would itself mismatch the canonicalizer) and a warning is
// logged. No edge is ever dropped.
func ResolveImportPath(spec, sourceFile, workspaceRoot string, alias AliasMap, exists func(relPath string) bool, logger zerolog.Logger) string {
	if exists == nil {
		exists = func(string) bool { return false }
	}

	var basePath string // workspace-relative, no extension, slash-separated
	switch {
	case IsRelativeImport(spec):
		basePath = path.Join(path.Dir(filepath.ToSlash(sourceFile)), spec)
	case IsAliasImport(spec):
		mapped, ok := applyAlias(spec, alias)
		if !ok {
			logger.Warn().Str("spec", spec).Str("source", sourceFile).
				Msg("import alias not in alias map, keeping raw specifier")
			return spec
		}
		basePath = mapped
	default:
		// bare/scoped npm package: passthrough, no resolution attempted.
		return spec
	}

	// path.Join collapses "./" and "../"; a basePath that climbs above the
	// workspace root (including the bare ".." case) is unresolvable here and
	// must not probe outside the workspace — keep the raw spec.
	if basePath == "" || basePath == "." || basePath == ".." || strings.HasPrefix(basePath, "../") {
		logger.Warn().Str("spec", spec).Str("source", sourceFile).
			Msg("import resolves outside workspace, keeping raw specifier")
		return spec
	}

	if resolved, ok := probeImport(basePath, exists); ok {
		return resolved
	}

	logger.Warn().Str("spec", spec).Str("source", sourceFile).Str("base", basePath).
		Msg("import target not found by extension/index probing, keeping raw specifier")
	return spec
}

// applyAlias rewrites "<alias>/rest" to "<base>/rest" using the longest-matching
// alias prefix. Returns ok=false when no alias matches.
func applyAlias(spec string, alias AliasMap) (string, bool) {
	// Try the prefix forms longest-first so "@/" wins over a hypothetical "@".
	bestKey := ""
	for key := range alias {
		if matchesAliasPrefix(spec, key) && len(key) > len(bestKey) {
			bestKey = key
		}
	}
	if bestKey == "" {
		return "", false
	}
	rest := strings.TrimPrefix(spec, bestKey)
	rest = strings.TrimPrefix(rest, "/")
	base := alias[bestKey]
	return path.Join(base, rest), true
}

// matchesAliasPrefix reports whether spec begins with alias key as a path
// segment boundary ("~" matches "~/x" and "~x"; "@/" matches "@/x").
func matchesAliasPrefix(spec, key string) bool {
	if !strings.HasPrefix(spec, key) {
		return false
	}
	rest := spec[len(key):]
	// Exact alias (spec == key) is not an importable path.
	if rest == "" {
		return false
	}
	// "~foo" and "~/foo" both map; "@/foo" already carries its separator.
	// Degenerate shapes like "~~weird" match here but resolve to a non-existent
	// path and raw-fall-back in ResolveImportPath, so no special-casing is needed.
	return true
}

// probeImport tries extension and index-file variants of a base path, returning
// the first that exists (workspace-relative, slash-separated).
func probeImport(basePath string, exists func(relPath string) bool) (string, bool) {
	// If the spec already names a file with a known extension, accept it as-is.
	if ext := path.Ext(basePath); ext != "" && exists(basePath) {
		return basePath, true
	}
	for _, ext := range importProbeExtensions {
		cand := basePath + ext
		if exists(cand) {
			return cand, true
		}
	}
	for _, ext := range importIndexExtensions {
		cand := path.Join(basePath, "index"+ext)
		if exists(cand) {
			return cand, true
		}
	}
	return "", false
}

// --- Alias map loading -----------------------------------------------------

// tsconfigShape is the minimal subset of tsconfig/jsconfig we read.
type tsconfigShape struct {
	CompilerOptions struct {
		BaseURL string              `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

// AliasCache memoizes per-workspace alias maps so the (filesystem-reading)
// loader runs once per workspace, not once per edge.
type AliasCache struct {
	mu sync.Mutex
	m  map[string]AliasMap // workspaceRoot -> alias map
}

// NewAliasCache returns an empty, ready-to-use alias map cache.
func NewAliasCache() *AliasCache {
	return &AliasCache{m: make(map[string]AliasMap)}
}

// Get returns the cached alias map for workspaceRoot, loading it on first use.
func (c *AliasCache) Get(workspaceRoot string, logger zerolog.Logger) AliasMap {
	c.mu.Lock()
	defer c.mu.Unlock()
	if am, ok := c.m[workspaceRoot]; ok {
		return am
	}
	am := LoadAliasMap(workspaceRoot, logger)
	c.m[workspaceRoot] = am
	return am
}

// LoadAliasMap derives the import alias map for a workspace, in precedence order:
//  1. committed tsconfig.json / jsconfig.json compilerOptions.paths
//  2. generated .nuxt/tsconfig.json (often absent at index time, git-ignored)
//  3. Nuxt srcDir convention fallback (~/@ → derived srcDir base)
//
// Returned aliases map a prefix ("~", "@") to a workspace-relative base dir.
// An empty/missing config yields an empty map (all alias imports fall back to
// raw), which is the safe behavior.
func LoadAliasMap(workspaceRoot string, logger zerolog.Logger) AliasMap {
	if workspaceRoot == "" {
		return AliasMap{}
	}

	for _, name := range []string{"tsconfig.json", "jsconfig.json"} {
		if am, ok := aliasFromTSConfig(filepath.Join(workspaceRoot, name), logger); ok {
			return am
		}
	}
	if am, ok := aliasFromTSConfig(filepath.Join(workspaceRoot, ".nuxt", "tsconfig.json"), logger); ok {
		return am
	}
	return nuxtConventionAlias(workspaceRoot)
}

// aliasFromTSConfig reads compilerOptions.paths from a tsconfig/jsconfig file
// and converts entries like "~/*": ["./*"] into an alias prefix → base map.
// Returns ok=false when the file is absent or yields no usable aliases.
func aliasFromTSConfig(configPath string, logger zerolog.Logger) (AliasMap, bool) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, false
	}
	var cfg tsconfigShape
	if err := json.Unmarshal(data, &cfg); err != nil {
		logger.Warn().Err(err).Str("config", configPath).
			Msg("tsconfig parse failed, skipping its alias paths")
		return nil, false
	}

	am := AliasMap{}
	for pattern, targets := range cfg.CompilerOptions.Paths {
		if len(targets) == 0 {
			continue
		}
		prefix, ok := aliasPrefix(pattern)
		if !ok {
			continue
		}
		base := aliasBase(targets[0], cfg.CompilerOptions.BaseURL)
		// Keep the first (highest-priority) target for each prefix.
		if _, exists := am[prefix]; !exists {
			am[prefix] = base
		}
	}
	if len(am) == 0 {
		return nil, false
	}
	return am, true
}

// aliasPrefix extracts the alias prefix from a tsconfig paths pattern.
// "~/*" → "~", "@/*" → "@/", "~" → "~". Non-alias patterns (bare package
// remaps like "components/*") are ignored.
func aliasPrefix(pattern string) (string, bool) {
	clean := strings.TrimSuffix(pattern, "*")
	clean = strings.TrimSuffix(clean, "/")
	switch clean {
	case "~":
		return "~", true
	case "@":
		return "@/", true
	default:
		return "", false
	}
}

// aliasBase converts a tsconfig path target ("./*", "./src/*", "app/*") plus an
// optional baseUrl into a workspace-relative base directory ("" for root, "src",
// "app", ...).
func aliasBase(target, baseURL string) string {
	clean := strings.TrimSuffix(target, "*")
	clean = strings.TrimSuffix(clean, "/")
	clean = strings.TrimPrefix(clean, "./")
	if baseURL != "" {
		b := strings.TrimPrefix(strings.TrimSuffix(baseURL, "/"), "./")
		clean = path.Join(b, clean)
	}
	if clean == "." {
		return ""
	}
	return clean
}

// nuxtConventionAlias derives the Nuxt srcDir alias fallback when no tsconfig is
// present. Nuxt 4 nests source under app/; Nuxt 3 uses the project root. We can
// only detect the directory layout, so: if app/ exists, ~/@ → app; else root.
func nuxtConventionAlias(workspaceRoot string) AliasMap {
	if fi, err := os.Stat(filepath.Join(workspaceRoot, "app")); err == nil && fi.IsDir() {
		return AliasMap{"~": "app", "@/": "app"}
	}
	return AliasMap{"~": "", "@/": ""}
}
