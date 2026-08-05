package graph

import (
	"sort"
	"testing"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func TestSourceScopedCallResolutionParserFeasibility(t *testing.T) {
	cases := []struct {
		name     string
		language *gotreesitter.Language
		source   string
		required []string
	}{
		{
			name:     "javascript-bindings-scopes-and-members",
			language: grammars.JavascriptLanguage(),
			source: `
import defaultService, { run as execute } from "./api";
import * as api from "./api";
class Controller {
  method(run) { var legacy; { let execute = () => {}; execute(); } try {} catch (api) { api.run(); } return this.api.run(); }
}
function hoisted() { return local(); }
function local() {}
`,
			required: []string{"import_statement", "import_specifier", "namespace_import", "class_declaration", "method_definition", "formal_parameters", "variable_declaration", "lexical_declaration", "catch_clause", "member_expression", "call_expression"},
		},
		{
			name:     "typescript-properties-and-parameter-properties",
			language: grammars.TypescriptLanguage(),
			source: `
import Api, { run as execute } from "./api";
class Controller {
  private api: Api;
  constructor(protected client: Api, value: Api) {}
  caller(run: () => void) { this.api.run(); this.client.run(); execute(); }
}
`,
			required: []string{"import_statement", "import_specifier", "class_declaration", "public_field_definition", "method_definition", "required_parameter", "accessibility_modifier", "type_annotation", "type_identifier", "member_expression", "call_expression"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parser := gotreesitter.NewParser(tc.language)
			tree, err := parser.Parse([]byte(tc.source))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			bound := gotreesitter.Bind(tree)
			defer bound.Release()

			kinds := nodeKinds(tree.RootNode(), tc.language)
			for _, required := range tc.required {
				if !kinds[required] {
					t.Errorf("parser did not expose node kind %q; got %v", required, sortedKinds(kinds))
				}
			}
			for _, field := range []struct {
				nodeType string
				field    string
			}{
				{"import_statement", "source"},
				{"class_declaration", "name"},
				{"method_definition", "name"},
				{"method_definition", "body"},
				{"call_expression", "function"},
			} {
				node := firstNodeWithType(tree.RootNode(), tc.language, field.nodeType)
				if node == nil || node.ChildByFieldName(field.field, tc.language) == nil {
					t.Errorf("%s.%s is not available", field.nodeType, field.field)
				}
			}
			t.Logf("parser node kinds: %v", sortedKinds(kinds))
		})
	}
}

func firstNodeWithType(node *gotreesitter.Node, language *gotreesitter.Language, want string) *gotreesitter.Node {
	if node == nil {
		return nil
	}
	if node.Type(language) == want {
		return node
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		if found := firstNodeWithType(node.Child(i), language, want); found != nil {
			return found
		}
	}
	return nil
}

func nodeKinds(node *gotreesitter.Node, language *gotreesitter.Language) map[string]bool {
	kinds := make(map[string]bool)
	var visit func(*gotreesitter.Node)
	visit = func(current *gotreesitter.Node) {
		if current == nil {
			return
		}
		kinds[current.Type(language)] = true
		for i := 0; i < int(current.ChildCount()); i++ {
			visit(current.Child(i))
		}
	}
	visit(node)
	return kinds
}

func sortedKinds(kinds map[string]bool) []string {
	values := make([]string, 0, len(kinds))
	for kind := range kinds {
		values = append(values, kind)
	}
	sort.Strings(values)
	return values
}
