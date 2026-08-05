package graph

import gotreesitter "github.com/odvcencio/gotreesitter"

// callResolver holds source-scoped state for one contextual JS/TS extraction.
// Extractor adapters populate it in tasks 2.2 and 2.3.
type callResolver struct {
	filePath string
	imports  ImportContext
	catalog  *directExportCatalog
	module   *lexicalScope
}

func newCallResolver(filePath string, imports ImportContext) *callResolver {
	module := newLexicalScope(nil, true)
	return &callResolver{
		filePath: filePath,
		imports:  imports,
		catalog:  newDirectExportCatalog(imports),
		module:   module,
	}
}

// forEachImportBinding walks the JavaScript/TypeScript import_clause grammar
// instead of searching source text for the `from` keyword. Imported names can
// themselves be named `from`, so textual splitting would mis-bind valid
// direct imports.
func forEachImportBinding(node *gotreesitter.Node, lang *gotreesitter.Language, source []byte, fn func(local, exported string, namespace bool)) {
	if node == nil {
		return
	}
	switch node.Type(lang) {
	case "import_clause":
		for i := 0; i < int(node.ChildCount()); i++ {
			forEachImportBinding(node.Child(i), lang, source, fn)
		}
	case "namespace_import":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type(lang) == "identifier" {
				fn(child.Text(source), "", true)
				return
			}
		}
	case "import_specifier":
		name := node.ChildByFieldName("name", lang)
		alias := node.ChildByFieldName("alias", lang)
		if name == nil {
			return
		}
		exported := name.Text(source)
		local := exported
		if alias != nil {
			local = alias.Text(source)
		}
		if isIdentifier(local) && isIdentifier(exported) {
			fn(local, exported, false)
		}
	case "identifier":
		name := node.Text(source)
		if isIdentifier(name) {
			fn(name, "default", false)
		}
	default:
		for i := 0; i < int(node.ChildCount()); i++ {
			forEachImportBinding(node.Child(i), lang, source, fn)
		}
	}
}

func (r *callResolver) resolveImported(rawSpecifier, exportName, method string) string {
	exports := r.catalog.module(rawSpecifier, r.filePath)
	if exports == nil {
		return unresolvedCallTarget
	}
	export, ok := exports[exportName]
	if !ok || export.target == "" {
		return unresolvedCallTarget
	}
	if method == "" {
		return export.target
	}
	target, ok := export.methods[method]
	if !ok {
		return unresolvedCallTarget
	}
	return target
}

func (r *callResolver) resolveBinding(scope *lexicalScope, name string) string {
	value := scope.lookup(name)
	if value.unresolved || !IsCanonicalJSCallTarget(value.target) {
		return unresolvedCallTarget
	}
	return value.target
}

type directReceiver struct {
	fields map[string]map[string]string
}

func (r directReceiver) resolve(field, method string) string {
	methods := r.fields[field]
	target, ok := methods[method]
	if !ok || !IsCanonicalJSCallTarget(target) {
		return unresolvedCallTarget
	}
	return target
}
