package graph

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// ImportContext carries the per-file resolution context threaded into the
// JS/TS/Vue extractors so they can resolve raw import specifiers
// (`~/utils/enums`, `./foo`, `vue`) to workspace-relative paths. It is passed
// per call (never stored on an extractor's own struct fields), so a single
// shared extractor instance stays safe for concurrent use across multiple
// watched collections/workspaces that may have completely different alias
// maps (see AliasIndex — G3, nearest-ancestor tsconfig resolution).
type ImportContext struct {
	// AliasMap maps an alias prefix (e.g. "~/", "@/", "@utils/") to the
	// workspace-relative directory it substitutes to. Nil is a valid,
	// safe no-op (only alias-style specifiers go unresolved; relative and
	// bare-package resolution are unaffected).
	AliasMap map[string]string
	// Exists reports whether a candidate workspace-relative path
	// corresponds to an admitted file. Nil disables the existence/suffix
	// probe (a resolved candidate is accepted as-is, unverified).
	Exists func(workspaceRelPath string) bool
}

// ImportResolvingExtractor is implemented by extractors that can resolve
// import specifiers given an ImportContext. It intentionally does NOT
// replace or modify the shared Extractor interface (internal/graph/edge.go)
// — every other extractor is untouched. Registry callers that want
// resolution type-assert for this interface and fall back to the plain
// Extractor.ExtractEdges otherwise.
type ImportResolvingExtractor interface {
	Extractor
	ExtractEdgesWithImportContext(filePath string, content []byte, ic ImportContext) ([]Edge, error)
}

// importSuffixCandidates are tried, in order, against ImportContext.Exists
// when a resolved candidate doesn't exist verbatim — covers extension-omitted
// specifiers and directory-index imports (e.g. `~/components` -> `components/index.ts`).
var importSuffixCandidates = []string{"", ".ts", ".tsx", ".js", ".jsx", ".vue", "/index.ts", "/index.tsx", "/index.js", "/index.jsx", "/index.vue"}

// ResolveImportTarget resolves rawSpecifier (exactly as written in an
// import/require statement) to the workspace-relative path format used for
// graph_edges.target_node / source_node / document paths: forward-slash,
// relative to the workspace (collection) root, no leading "./". sourceRelPath
// is the importing file's own path in that same format.
//
// Resolution order (per design.md "Fix B resolution algorithm"):
//  1. Relative ("./", "../"): joined against the importing file's directory.
//     Needs no alias map — always attempted first.
//  2. Aliased: longest-matching prefix in aliasMap is substituted.
//  3. Bare package (anything else): left unchanged — this is correct
//     behavior, not a resolution gap.
//
// After steps 1/2 produce a candidate, it is verified against `exists`
// (falling back through importSuffixCandidates for extension/index
// omission). If nothing matches, the RAW specifier is returned unchanged
// (fail-safe, never fail-silent-wrong) — this also implements the
// path-escape clamp: a relative candidate that resolves above the workspace
// root is never returned.
func ResolveImportTarget(rawSpecifier, sourceRelPath string, aliasMap map[string]string, exists func(string) bool) string {
	if rawSpecifier == "" {
		return rawSpecifier
	}

	var candidate string
	if strings.HasPrefix(rawSpecifier, "./") || strings.HasPrefix(rawSpecifier, "../") {
		dir := path.Dir(filepath.ToSlash(sourceRelPath))
		if dir == "." {
			dir = ""
		}
		joined := path.Join(dir, rawSpecifier)
		if escapesRoot(joined) {
			return rawSpecifier
		}
		candidate = joined
	} else {
		prefix, targetRoot, ok := longestAliasMatch(rawSpecifier, aliasMap)
		if !ok {
			return rawSpecifier // bare package specifier — unchanged
		}
		remainder := strings.TrimPrefix(rawSpecifier, prefix)
		joined := path.Join(targetRoot, remainder)
		if escapesRoot(joined) {
			return rawSpecifier
		}
		candidate = joined
	}

	if exists == nil {
		return candidate
	}
	for _, suf := range importSuffixCandidates {
		if try := candidate + suf; exists(try) {
			return try
		}
	}
	return rawSpecifier
}

// escapesRoot reports whether a joined, not-yet-cleaned relative path
// resolves above the workspace root once cleaned (e.g. "../x" from a
// top-level file importing "../../x").
func escapesRoot(p string) bool {
	clean := path.Clean(p)
	return clean == ".." || strings.HasPrefix(clean, "../")
}

func longestAliasMatch(specifier string, aliasMap map[string]string) (prefix, targetRoot string, ok bool) {
	bestLen := -1
	for p, root := range aliasMap {
		if strings.HasPrefix(specifier, p) && len(p) > bestLen {
			bestLen = len(p)
			prefix, targetRoot, ok = p, root, true
		}
	}
	return prefix, targetRoot, ok
}

// resolveImport applies ic to a raw specifier found in sourceRelPath and
// returns the Edge.TargetNode value plus (only when the specifier actually
// changed) the {"raw_specifier": ...} metadata to attach — preserving the
// original alias/relative string per #501's own suggestion. A zero-value
// ImportContext still performs relative-import resolution (it needs no
// alias map) but leaves alias/bare specifiers untouched, since AliasMap and
// Exists are both nil.
func resolveImport(ic ImportContext, rawSpecifier, sourceRelPath string) (target string, metadata map[string]any) {
	resolved := ResolveImportTarget(rawSpecifier, sourceRelPath, ic.AliasMap, ic.Exists)
	if resolved == rawSpecifier {
		return rawSpecifier, nil
	}
	return resolved, map[string]any{"raw_specifier": rawSpecifier}
}

// --- Alias map loading (G3: nearest-ancestor, not a single global map) ---

// conventionAliases are framework-convention specifiers (Nuxt/Vite `~/` and
// `@/`) treated as built-in aliases pointing at the nearest config/package
// root, without evaluating any TS (nuxt.config.ts is never parsed).
var conventionAliases = []string{"~/", "@/"}

// AliasIndex maps config directories (workspace-relative, forward-slash,
// "" for the workspace root) to their tsconfig/jsconfig "paths" alias map.
// It is built once per workspace/collection root (not per file) and looked
// up per source file via nearest-ancestor resolution: a file nested under a
// sub-package's own tsconfig uses THAT config's map, never a sibling
// package's or a stale global one (a single global map is known-wrong for
// multi-repo workspaces — each repo's "~/" points at its own root).
type AliasIndex struct {
	// configDirs is sorted longest-path-first so the nearest (most nested)
	// ancestor is matched before a shallower one.
	configDirs []string
	aliasMaps  map[string]map[string]string
}

// skippedDirs are never descended into while searching for tsconfig/jsconfig
// files — generated/vendored/dependency trees never define authoritative
// aliases for the workspace's own source, and walking them wastes I/O on
// large repos.
var skippedDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".next":        true,
	".nuxt":        true,
	".output":      true,
}

// BuildAliasIndex walks workspaceRootAbs (an absolute filesystem path) for
// tsconfig.json/jsconfig.json at any depth, parses `compilerOptions.paths`
// (tolerating JSONC comments/trailing commas — see parseJSONC) into a
// prefix->targetRoot alias map per config directory, and registers the
// built-in "~/"/"@/" convention aliases for that same directory unless the
// config already defines its own mapping for that prefix. If no config is
// found anywhere, the workspace root still gets a synthetic entry so "~/"
// and "@/" resolve workspace-wide (matches today's Nuxt convention usage
// without needing any config file at all).
func BuildAliasIndex(workspaceRootAbs string) (*AliasIndex, error) {
	idx := &AliasIndex{aliasMaps: map[string]map[string]string{}}
	found := false

	walkErr := filepath.WalkDir(workspaceRootAbs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort; skip unreadable entries
		}
		if d.IsDir() {
			if p != workspaceRootAbs && skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if name != "tsconfig.json" && name != "jsconfig.json" {
			return nil
		}
		configDirAbs := filepath.Dir(p)
		rel, relErr := filepath.Rel(workspaceRootAbs, configDirAbs)
		if relErr != nil {
			return nil
		}
		relSlash := toWorkspaceRel(rel)
		raw, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil
		}
		merged := idx.aliasMaps[relSlash]
		if merged == nil {
			merged = map[string]string{}
		}
		for k, v := range parseTSConfigPaths(raw, relSlash) {
			merged[k] = v
		}
		for _, conv := range conventionAliases {
			if _, ok := merged[conv]; !ok {
				merged[conv] = relSlash
			}
		}
		idx.aliasMaps[relSlash] = merged
		found = true
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if !found {
		root := map[string]string{}
		for _, conv := range conventionAliases {
			root[conv] = ""
		}
		idx.aliasMaps[""] = root
	}

	for dir := range idx.aliasMaps {
		idx.configDirs = append(idx.configDirs, dir)
	}
	sort.Slice(idx.configDirs, func(i, j int) bool {
		return len(idx.configDirs[i]) > len(idx.configDirs[j])
	})
	return idx, nil
}

// AliasMapFor returns the nearest-ancestor alias map for a source file
// (workspace-relative path, forward-slash). Returns nil if the index has no
// entries at all.
func (idx *AliasIndex) AliasMapFor(sourceRelPath string) map[string]string {
	if idx == nil {
		return nil
	}
	sourceDir := toWorkspaceRel(path.Dir(filepath.ToSlash(sourceRelPath)))
	for _, cd := range idx.configDirs {
		if cd == "" {
			continue
		}
		if sourceDir == cd || strings.HasPrefix(sourceDir, cd+"/") {
			return idx.aliasMaps[cd]
		}
	}
	return idx.aliasMaps[""]
}

func toWorkspaceRel(rel string) string {
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return ""
	}
	return rel
}

// parseTSConfigPaths extracts compilerOptions.paths from a tsconfig/jsconfig
// JSON(C) payload, resolving each target against compilerOptions.baseUrl
// (default ".") and the config's own workspace-relative directory. Returns
// nil (not an error) on any parse/shape failure — a malformed config simply
// contributes no aliases, it must never abort indexing.
func parseTSConfigPaths(raw []byte, configDirRelSlash string) map[string]string {
	data, err := parseJSONC(raw)
	if err != nil {
		return nil
	}
	co, _ := data["compilerOptions"].(map[string]any)
	if co == nil {
		return nil
	}
	baseURL := "."
	if b, ok := co["baseUrl"].(string); ok && b != "" {
		baseURL = b
	}
	pathsRaw, _ := co["paths"].(map[string]any)
	if pathsRaw == nil {
		return nil
	}
	result := map[string]string{}
	for key, val := range pathsRaw {
		targets, ok := val.([]any)
		if !ok || len(targets) == 0 {
			continue
		}
		targetStr, ok := targets[0].(string)
		if !ok {
			continue
		}
		prefix := strings.TrimSuffix(key, "*")
		target := strings.TrimSuffix(targetStr, "*")
		joined := path.Join(configDirRelSlash, baseURL, target)
		result[prefix] = toWorkspaceRel(joined)
	}
	return result
}

// parseJSONC parses JSON that may contain // and /* */ comments and trailing
// commas (tsconfig.json is conventionally JSONC, not strict JSON). It tries
// strict json.Unmarshal first and only pays the tolerant-parse cost on
// failure.
func parseJSONC(raw []byte) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err == nil {
		return m, nil
	}
	stripped := stripJSONCComments(raw)
	if err := json.Unmarshal(stripped, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// stripJSONCComments removes // line comments and /* */ block comments that
// occur outside of JSON string literals, then strips trailing commas before
// a closing "}" or "]". It is a minimal tolerant scanner, not a full JSONC
// parser — sufficient for real-world tsconfig.json files.
func stripJSONCComments(raw []byte) []byte {
	var out []byte
	inString, inLineComment, inBlockComment, escaped := false, false, false, false

	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case inLineComment:
			if c == '\n' {
				inLineComment = false
				out = append(out, c)
			}
		case inBlockComment:
			if c == '*' && i+1 < len(raw) && raw[i+1] == '/' {
				inBlockComment = false
				i++
			}
		case inString:
			out = append(out, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(raw) && raw[i+1] == '/':
			inLineComment = true
			i++
		case c == '/' && i+1 < len(raw) && raw[i+1] == '*':
			inBlockComment = true
			i++
		default:
			out = append(out, c)
		}
	}
	return stripTrailingCommas(out)
}

func stripTrailingCommas(raw []byte) []byte {
	var out []byte
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == ',' {
			j := i + 1
			for j < len(raw) && (raw[j] == ' ' || raw[j] == '\t' || raw[j] == '\n' || raw[j] == '\r') {
				j++
			}
			if j < len(raw) && (raw[j] == '}' || raw[j] == ']') {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// DiskExistsChecker returns an ImportContext.Exists func backed by real
// filesystem lookups under workspaceRootAbs — the existence/suffix-probe
// step used by ResolveImportTarget when indexing an actual workspace.
func DiskExistsChecker(workspaceRootAbs string) func(string) bool {
	return func(relPath string) bool {
		if relPath == "" {
			return false
		}
		info, err := os.Stat(filepath.Join(workspaceRootAbs, relPath))
		return err == nil && !info.IsDir()
	}
}
