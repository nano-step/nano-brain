package graph

import (
	"testing"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func TestGotreesitterJSAST(t *testing.T) {
	// Test JS/TS grammar with gotreesitter
	jsLang := grammars.JavascriptLanguage()
	tsLang := grammars.TypescriptLanguage()

	// Minimal JS/TS test code
	testCode := `
function handleRequest(req, res) {
  const param = req.params.id;
  if (param === '') {
    return res.status(400).json({ error: 'invalid' });
  }
  switch (param) {
    case 'admin':
      return res.status(200).json({ role: 'admin' });
    case 'user':
      return res.status(200).json({ role: 'user' });
    default:
      return res.status(404).json({ error: 'not found' });
  }
}

const processItem = (item) => {
  if (!item) {
    throw new Error('item required');
  }
  return item.value;
};
`

	// Parse JS
	parser := gotreesitter.NewParser(jsLang)
	tree, err := parser.Parse([]byte(testCode))
	if err != nil {
		t.Fatalf("Failed to parse JS: %v", err)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	root := tree.RootNode()
	t.Logf("Root node type: %s", root.Type(jsLang))
	t.Logf("Root child count: %d", root.ChildCount())

	// Test ChildByFieldName for function_declaration
	walkNodes(root, jsLang, "function_declaration", func(n *gotreesitter.Node) {
		nameNode := n.ChildByFieldName("name", jsLang)
		bodyNode := n.ChildByFieldName("body", jsLang)
		if nameNode != nil && bodyNode != nil {
			t.Logf("Function declaration: %s", bt.NodeText(nameNode))
			t.Logf("  Body type: %s", bodyNode.Type(jsLang))
		}
	})

	// Test ChildByFieldName for if_statement
	walkNodes(root, jsLang, "if_statement", func(n *gotreesitter.Node) {
		conditionNode := n.ChildByFieldName("condition", jsLang)
		consequenceNode := n.ChildByFieldName("consequence", jsLang)
		alternativeNode := n.ChildByFieldName("alternative", jsLang)
		if conditionNode != nil {
			t.Logf("If statement condition: %s", bt.NodeText(conditionNode))
			t.Logf("  Condition type: %s", conditionNode.Type(jsLang))
		}
		if consequenceNode != nil {
			t.Logf("  Consequence type: %s", consequenceNode.Type(jsLang))
		}
		if alternativeNode != nil {
			t.Logf("  Alternative type: %s", alternativeNode.Type(jsLang))
		}
	})

	// Test ChildByFieldName for switch_statement
	walkNodes(root, jsLang, "switch_statement", func(n *gotreesitter.Node) {
		discriminantNode := n.ChildByFieldName("discriminant", jsLang)
		bodyNode := n.ChildByFieldName("body", jsLang)
		if discriminantNode != nil {
			t.Logf("Switch statement discriminant: %s", bt.NodeText(discriminantNode))
		}
		if bodyNode != nil {
			t.Logf("  Switch body type: %s", bodyNode.Type(jsLang))
			t.Logf("  Switch body child count: %d", bodyNode.ChildCount())
		}
	})

	// Test ChildByFieldName for return_statement
	walkNodes(root, jsLang, "return_statement", func(n *gotreesitter.Node) {
		valueNode := n.ChildByFieldName("value", jsLang)
		if valueNode != nil {
			t.Logf("Return statement value type: %s", valueNode.Type(jsLang))
		}
	})

	// Test ChildByFieldName for arrow_function
	walkNodes(root, jsLang, "arrow_function", func(n *gotreesitter.Node) {
		parametersNode := n.ChildByFieldName("parameters", jsLang)
		bodyNode := n.ChildByFieldName("body", jsLang)
		if parametersNode != nil {
			t.Logf("Arrow function parameters type: %s", parametersNode.Type(jsLang))
		}
		if bodyNode != nil {
			t.Logf("  Arrow function body type: %s", bodyNode.Type(jsLang))
		}
	})

	// Parse TypeScript
	parserTS := gotreesitter.NewParser(tsLang)
	treeTS, err := parserTS.Parse([]byte(testCode))
	if err != nil {
		t.Fatalf("Failed to parse TS: %v", err)
	}
	btTS := gotreesitter.Bind(treeTS)
	defer btTS.Release()

	rootTS := treeTS.RootNode()
	t.Logf("TS Root node type: %s", rootTS.Type(tsLang))

	// Test TypeScript-specific features
	tsCode := `
interface User {
  id: string;
  name: string;
}

function getUser(id: string): User | null {
  if (id === '') {
    return null;
  }
  return { id, name: 'test' };
}
`

	treeTS2, err := parserTS.Parse([]byte(tsCode))
	if err != nil {
		t.Fatalf("Failed to parse TS interface: %v", err)
	}
	btTS2 := gotreesitter.Bind(treeTS2)
	defer btTS2.Release()

	rootTS2 := treeTS2.RootNode()
	t.Logf("TS2 Root node type: %s", rootTS2.Type(tsLang))

	// Test interface_declaration
	walkNodes(rootTS2, tsLang, "interface_declaration", func(n *gotreesitter.Node) {
		nameNode := n.ChildByFieldName("name", tsLang)
		bodyNode := n.ChildByFieldName("body", tsLang)
		if nameNode != nil {
			t.Logf("Interface declaration: %s", btTS2.NodeText(nameNode))
		}
		if bodyNode != nil {
			t.Logf("  Interface body type: %s", bodyNode.Type(tsLang))
		}
	})

	// Test type_annotation
	walkNodes(rootTS2, tsLang, "type_annotation", func(n *gotreesitter.Node) {
		t.Logf("Type annotation type: %s", n.Type(tsLang))
		t.Logf("  Type annotation text: %s", btTS2.NodeText(n))
	})
}

func TestGotreesitterChildByFieldName(t *testing.T) {
	jsLang := grammars.JavascriptLanguage()

	testCode := `
if (condition) {
  return value;
}
switch (expr) {
  case 1: break;
}
function foo() {}
const bar = () => {};
`

	parser := gotreesitter.NewParser(jsLang)
	tree, err := parser.Parse([]byte(testCode))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	root := tree.RootNode()

	// if_statement: ChildByFieldName works for condition, consequence, alternative
	walkNodes(root, jsLang, "if_statement", func(n *gotreesitter.Node) {
		condition := n.ChildByFieldName("condition", jsLang)
		consequence := n.ChildByFieldName("consequence", jsLang)
		alternative := n.ChildByFieldName("alternative", jsLang)

		if condition == nil {
			t.Error("if_statement.condition is nil")
		} else {
			t.Logf("if_statement.condition: %s", bt.NodeText(condition))
		}

		if consequence == nil {
			t.Error("if_statement.consequence is nil")
		} else {
			t.Logf("if_statement.consequence type: %s", consequence.Type(jsLang))
		}

		if alternative != nil {
			t.Logf("if_statement.alternative type: %s", alternative.Type(jsLang))
		}
	})

	// switch_statement: ChildByFieldName doesn't work - use child index
	// Structure: switch, parenthesized_expression (discriminant), switch_body
	walkNodes(root, jsLang, "switch_statement", func(n *gotreesitter.Node) {
		discriminant := n.ChildByFieldName("discriminant", jsLang)
		body := n.ChildByFieldName("body", jsLang)

		if discriminant != nil {
			t.Logf("switch_statement.discriminant: %s", bt.NodeText(discriminant))
		} else {
			t.Log("switch_statement.discriminant is nil (use Child(1) instead)")
		}

		if body != nil {
			t.Logf("switch_statement.body type: %s", body.Type(jsLang))
		} else {
			t.Log("switch_statement.body is nil (use Child(2) instead)")
		}

		// Access by index: child(0)=switch, child(1)=discriminant, child(2)=body
		if n.ChildCount() >= 3 {
			discriminantByIndex := n.Child(1)
			bodyByIndex := n.Child(2)
			t.Logf("switch_statement by index - discriminant: %s, body type: %s",
				bt.NodeText(discriminantByIndex), bodyByIndex.Type(jsLang))
		}
	})

	// return_statement: ChildByFieldName doesn't work - use child index
	// Structure: return keyword, value expression, semicolon
	walkNodes(root, jsLang, "return_statement", func(n *gotreesitter.Node) {
		value := n.ChildByFieldName("value", jsLang)

		if value != nil {
			t.Logf("return_statement.value: %s", bt.NodeText(value))
		} else {
			t.Log("return_statement.value is nil (use Child(1) instead)")
		}

		// Access by index: child(0)=return, child(1)=value, child(2)=;
		if n.ChildCount() >= 2 {
			valueByIndex := n.Child(1)
			t.Logf("return_statement by index - value: %s", bt.NodeText(valueByIndex))
		}
	})

	// function_declaration: ChildByFieldName works for name, parameters, body
	walkNodes(root, jsLang, "function_declaration", func(n *gotreesitter.Node) {
		name := n.ChildByFieldName("name", jsLang)
		parameters := n.ChildByFieldName("parameters", jsLang)
		body := n.ChildByFieldName("body", jsLang)

		if name == nil {
			t.Error("function_declaration.name is nil")
		} else {
			t.Logf("function_declaration.name: %s", bt.NodeText(name))
		}

		if parameters == nil {
			t.Error("function_declaration.parameters is nil")
		} else {
			t.Logf("function_declaration.parameters type: %s", parameters.Type(jsLang))
		}

		if body == nil {
			t.Error("function_declaration.body is nil")
		} else {
			t.Logf("function_declaration.body type: %s", body.Type(jsLang))
		}
	})

	// arrow_function: ChildByFieldName works for parameters, body
	walkNodes(root, jsLang, "arrow_function", func(n *gotreesitter.Node) {
		parameters := n.ChildByFieldName("parameters", jsLang)
		body := n.ChildByFieldName("body", jsLang)

		if parameters == nil {
			t.Error("arrow_function.parameters is nil")
		} else {
			t.Logf("arrow_function.parameters type: %s", parameters.Type(jsLang))
		}

		if body == nil {
			t.Error("arrow_function.body is nil")
		} else {
			t.Logf("arrow_function.body type: %s", body.Type(jsLang))
		}
	})
}
