package graph

import (
	"regexp"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

type directExport struct {
	target  string
	methods map[string]string
}

type directExportCatalog struct {
	imports ImportContext
	modules map[string]map[string]directExport
}

func newDirectExportCatalog(imports ImportContext) *directExportCatalog {
	return &directExportCatalog{imports: imports, modules: make(map[string]map[string]directExport)}
}

var (
	directNamedExport   = regexp.MustCompile(`^\s*export\s+(?:async\s+)?(?:function|class)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`)
	directValueExport   = regexp.MustCompile(`^\s*export\s+(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`)
	directDefaultExport = regexp.MustCompile(`^\s*export\s+default\s+(?:(?:async\s+)?(?:function|class)\b|\(?|[A-Za-z_$])`)
	identifierPattern   = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)
)

// module returns direct exports for one resolved project-local module. The
// cache belongs to this contextual extraction only, so exporter changes are
// always visible to the next extraction run.
func (c *directExportCatalog) module(rawSpecifier, sourceFile string) map[string]directExport {
	if !isProjectImportSpecifier(rawSpecifier, c.imports.AliasMap) || c.imports.Exists == nil || c.imports.ReadFile == nil {
		return nil
	}
	module := ResolveImportTarget(rawSpecifier, sourceFile, c.imports.AliasMap, c.imports.Exists)
	if module == rawSpecifier || !isJavaScriptOrTypeScriptPath(module) {
		return nil
	}
	if exports, ok := c.modules[module]; ok {
		return exports
	}

	content, err := c.imports.ReadFile(module)
	if err != nil {
		c.modules[module] = nil
		return nil
	}
	exports := parseDirectExports(module, content)
	c.modules[module] = exports
	return exports
}

func isProjectImportSpecifier(raw string, aliases map[string]string) bool {
	if strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") {
		return true
	}
	_, _, ok := longestAliasMatch(raw, aliases)
	return ok
}

func isJavaScriptOrTypeScriptPath(filePath string) bool {
	switch strings.ToLower(fileExtension(filePath)) {
	case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts":
		return true
	default:
		return false
	}
}

func fileExtension(filePath string) string {
	lastSlash := strings.LastIndexByte(filePath, '/')
	lastDot := strings.LastIndexByte(filePath, '.')
	if lastDot <= lastSlash {
		return ""
	}
	return filePath[lastDot:]
}

func parseDirectExports(filePath string, content []byte) map[string]directExport {
	lang := languageForSourcePath(filePath)
	if lang == nil {
		return nil
	}
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil
	}
	bound := gotreesitter.Bind(tree)
	defer bound.Release()

	exports := make(map[string]directExport)
	for i := 0; i < int(tree.RootNode().ChildCount()); i++ {
		node := tree.RootNode().Child(i)
		if node == nil || node.Type(lang) != "export_statement" {
			continue
		}
		for _, name := range directExportNames(bound.NodeText(node), node, lang, bound) {
			recordDirectExport(exports, name, filePath, node, lang, bound)
		}
	}
	return exports
}

func languageForSourcePath(filePath string) *gotreesitter.Language {
	switch strings.ToLower(fileExtension(filePath)) {
	case ".js", ".jsx", ".mjs", ".cjs":
		return grammars.JavascriptLanguage()
	case ".ts", ".mts", ".cts":
		return grammars.TypescriptLanguage()
	case ".tsx":
		return grammars.TsxLanguage()
	default:
		return nil
	}
}

func directExportNames(source string, node *gotreesitter.Node, lang *gotreesitter.Language, bound *gotreesitter.BoundTree) []string {
	if directDefaultExport.MatchString(source) {
		return []string{"default"}
	}
	if match := directNamedExport.FindStringSubmatch(source); len(match) == 2 {
		return []string{match[1]}
	}
	if !directValueExport.MatchString(source) {
		return nil
	}

	var declaration *gotreesitter.Node
	for _, want := range []string{"lexical_declaration", "variable_declaration"} {
		if declaration = firstDescendantOfType(node, lang, want); declaration != nil {
			break
		}
	}
	if declaration == nil {
		return nil
	}

	var names []string
	for i := 0; i < int(declaration.ChildCount()); i++ {
		declarator := declaration.Child(i)
		if declarator == nil || declarator.Type(lang) != "variable_declarator" {
			continue
		}
		name := declarator.ChildByFieldName("name", lang)
		if name != nil && isIdentifier(bound.NodeText(name)) {
			names = append(names, bound.NodeText(name))
		}
	}
	return names
}

func recordDirectExport(exports map[string]directExport, name, filePath string, node *gotreesitter.Node, lang *gotreesitter.Language, bound *gotreesitter.BoundTree) {
	if name == "" {
		return
	}
	if duplicate, exists := exports[name]; exists {
		duplicate.target = ""
		duplicate.methods = nil
		exports[name] = duplicate
		return
	}

	target := filePath + "::" + name
	exports[name] = directExport{target: target, methods: directClassMethods(node, target, lang, bound)}
}

func directClassMethods(node *gotreesitter.Node, target string, lang *gotreesitter.Language, bound *gotreesitter.BoundTree) map[string]string {
	class := firstDescendantOfType(node, lang, "class_declaration")
	if class == nil {
		return nil
	}
	className := class.ChildByFieldName("name", lang)
	if className == nil || !isIdentifier(bound.NodeText(className)) {
		return nil
	}
	body := firstDescendantOfType(class, lang, "class_body")
	if body == nil {
		return nil
	}
	methods := make(map[string]string)
	for i := 0; i < int(body.ChildCount()); i++ {
		method := body.Child(i)
		if method == nil || method.Type(lang) != "method_definition" {
			continue
		}
		name := method.ChildByFieldName("name", lang)
		if name == nil {
			continue
		}
		methodName := bound.NodeText(name)
		if isIdentifier(methodName) {
			methods[methodName] = target + "." + methodName
		}
	}
	return methods
}

func firstDescendantOfType(node *gotreesitter.Node, lang *gotreesitter.Language, want string) *gotreesitter.Node {
	if node == nil {
		return nil
	}
	if node.Type(lang) == want {
		return node
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		if found := firstDescendantOfType(node.Child(i), lang, want); found != nil {
			return found
		}
	}
	return nil
}

func isIdentifier(value string) bool {
	return identifierPattern.MatchString(value)
}
