package graph

import (
	"fmt"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const maxCFGNodes = 500
const labelMaxLen = 80

var _ ControlFlowExtractor = (*JSControlFlowExtractor)(nil)

type JSControlFlowExtractor struct {
	jsLang  *gotreesitter.Language
	tsLang  *gotreesitter.Language
	tsxLang *gotreesitter.Language
}

func NewJSControlFlowExtractor() (*JSControlFlowExtractor, error) {
	return &JSControlFlowExtractor{
		jsLang:  grammars.JavascriptLanguage(),
		tsLang:  grammars.TypescriptLanguage(),
		tsxLang: grammars.TsxLanguage(),
	}, nil
}

func (x *JSControlFlowExtractor) SupportsCFG(ext string) bool {
	return ext == ".js" || ext == ".jsx" || ext == ".ts" || ext == ".tsx"
}

// funcDef is a function/method/arrow extracted from a file, with its source span.
type funcDef struct {
	name      string
	startLine int
	endLine   int
	bodyNode  *gotreesitter.Node
}

func (x *JSControlFlowExtractor) ExtractCFGs(filePath string, content []byte) ([]CFG, error) {
	ext := filepath.Ext(filePath)
	lang, _ := x.langForExt(ext)

	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	root := tree.RootNode()
	relFile := filepath.ToSlash(filePath)

	var funcs []funcDef

	walkNodes(root, lang, "function_declaration", func(n *gotreesitter.Node) {
		nameNode := n.ChildByFieldName("name", lang)
		bodyNode := n.ChildByFieldName("body", lang)
		if nameNode == nil || bodyNode == nil {
			return
		}
		if bodyNode.Type(lang) != "statement_block" {
			return
		}
		funcs = append(funcs, funcDef{
			name:      bt.NodeText(nameNode),
			startLine: lineForByte(content, n.StartByte()),
			endLine:   lineForByte(content, n.EndByte()),
			bodyNode:  bodyNode,
		})
	})

	walkNodes(root, lang, "method_definition", func(n *gotreesitter.Node) {
		nameNode := n.ChildByFieldName("name", lang)
		bodyNode := n.ChildByFieldName("body", lang)
		if nameNode == nil || bodyNode == nil {
			return
		}
		if bodyNode.Type(lang) != "statement_block" {
			return
		}
		funcs = append(funcs, funcDef{
			name:      bt.NodeText(nameNode),
			startLine: lineForByte(content, n.StartByte()),
			endLine:   lineForByte(content, n.EndByte()),
			bodyNode:  bodyNode,
		})
	})

	walkNodes(root, lang, "arrow_function", func(n *gotreesitter.Node) {
		bodyNode := n.ChildByFieldName("body", lang)
		if bodyNode == nil {
			return
		}
		bodyType := bodyNode.Type(lang)
		if bodyType != "statement_block" && bodyType != "expression" {
			return
		}
		parent := n.Parent()
		if parent != nil && parent.Type(lang) == "variable_declarator" {
			nameNode := parent.ChildByFieldName("name", lang)
			if nameNode != nil {
				funcs = append(funcs, funcDef{
					name:      bt.NodeText(nameNode),
					startLine: lineForByte(content, n.StartByte()),
					endLine:   lineForByte(content, n.EndByte()),
					bodyNode:  n,
				})
			}
		}
	})

	var cfgs []CFG
	for _, fd := range funcs {
		cfg, err := x.extractOne(bt, lang, content, relFile, fd)
		if err != nil {
			return nil, fmt.Errorf("extract cfg %s::%s: %w", relFile, fd.name, err)
		}
		if cfg != nil {
			cfgs = append(cfgs, *cfg)
		}
	}
	if cfgs == nil {
		cfgs = []CFG{}
	}
	return cfgs, nil
}

func (x *JSControlFlowExtractor) extractOne(bt *gotreesitter.BoundTree, lang *gotreesitter.Language, content []byte, relFile string, fd funcDef) (*CFG, error) {
	entry := relFile + "::" + fd.name

	builder := &cfgbuilder{
		bt:      bt,
		lang:    lang,
		content: content,
		relFile: relFile,
		entry:   entry,
		nodeID:  0,
		cfg: &CFG{
			Entry:      entry,
			SourceFile: relFile,
			StartLine:  fd.startLine,
			EndLine:    fd.endLine,
			Status:     "complete",
		},
	}

	startNode := builder.addNode(CFGNode{
		Type:  "start",
		Label: entry,
		Line:  fd.startLine,
	})

	if fd.bodyNode.Type(lang) == "arrow_function" {
		bodyNode := fd.bodyNode.ChildByFieldName("body", lang)
		if bodyNode != nil && bodyNode.Type(lang) == "statement_block" {
			exits := builder.buildBlock(bodyNode, map[string]bool{startNode: true})
			if len(exits) > 0 {
				builder.addMergeToTerminal(exits)
			}
		} else if bodyNode != nil && bodyNode.Type(lang) == "expression" {
			stepID := builder.addNode(CFGNode{
				Type:  "step",
				Label: truncateLabel(bt.NodeText(bodyNode)),
				Line:  lineForByte(content, bodyNode.StartByte()),
			})
			builder.addEdge(startNode, stepID, "next")
			termID := builder.addNode(CFGNode{
				Type: "terminal",
				Kind: "return",
				Line: lineForByte(content, bodyNode.EndByte()),
			})
			builder.addEdge(stepID, termID, "next")
		}
	} else {
		exits := builder.buildBlock(fd.bodyNode, map[string]bool{startNode: true})
		if len(exits) > 0 {
			builder.addMergeToTerminal(exits)
		}
	}

	if builder.capped {
		builder.cfg.Status = "truncated"
	}

	if len(builder.cfg.Nodes) == 1 {
		return nil, nil
	}

	return builder.cfg, nil
}

type cfgbuilder struct {
	bt      *gotreesitter.BoundTree
	lang    *gotreesitter.Language
	content []byte
	relFile string
	entry   string
	nodeID  int
	cfg     *CFG
	capped  bool
}

// lineOf returns the 1-based source line of a node's start.
func (b *cfgbuilder) lineOf(n *gotreesitter.Node) int {
	return lineForByte(b.content, n.StartByte())
}

func (b *cfgbuilder) nextID() string {
	id := fmt.Sprintf("n%d", b.nodeID)
	b.nodeID++
	return id
}

func (b *cfgbuilder) addNode(n CFGNode) string {
	if b.capped {
		return ""
	}
	if len(b.cfg.Nodes) >= maxCFGNodes {
		b.capped = true
		return ""
	}
	n.ID = b.nextID()
	b.cfg.Nodes = append(b.cfg.Nodes, n)
	return n.ID
}

func (b *cfgbuilder) addEdge(from, to, branch string) {
	if from == "" || to == "" {
		return
	}
	b.cfg.Edges = append(b.cfg.Edges, CFGEdge{
		From:   from,
		To:     to,
		Branch: branch,
	})
}

func (b *cfgbuilder) addMergeToTerminal(preds map[string]bool) {
	termID := b.addNode(CFGNode{
		Type: "terminal",
		Kind: "return",
		Line: b.cfg.EndLine,
	})
	for p := range preds {
		b.addEdge(p, termID, "next")
	}
}

func (b *cfgbuilder) addMergeToStep(preds map[string]bool, label string, line int) string {
	if len(preds) == 1 {
		for p := range preds {
			return p
		}
	}
	mergeID := b.addNode(CFGNode{
		Type:  "step",
		Label: label,
		Line:  line,
	})
	for p := range preds {
		b.addEdge(p, mergeID, "next")
	}
	return mergeID
}

func (b *cfgbuilder) buildBlock(block *gotreesitter.Node, preds map[string]bool) map[string]bool {
	if block == nil {
		return preds
	}

	cur := preds
	childCount := int(block.ChildCount())

	for i := 0; i < childCount; i++ {
		stmt := block.Child(i)
		if stmt == nil {
			continue
		}
		stmtType := stmt.Type(b.lang)

		switch stmtType {
		case "if_statement":
			cur = b.buildIf(stmt, cur)
		case "switch_statement":
			cur = b.buildSwitch(stmt, cur)
		case "for_statement", "for_in_statement", "for_of_statement", "while_statement", "do_statement":
			cur = b.buildLoop(stmt, cur, stmtType)
		case "return_statement":
			cur = b.buildReturn(stmt, cur)
		case "throw_statement":
			cur = b.buildThrow(stmt, cur)
		case "try_statement":
			cur = b.buildTry(stmt, cur)
		case "expression_statement":
			cur = b.buildExpression(stmt, cur)
		case "variable_declaration":
			cur = b.buildVarDecl(stmt, cur)
		case "break_statement", "continue_statement":
			continue
		case "debugger_statement":
			continue
		default:
			cur = b.buildDefault(stmt, cur, stmtType)
		}

		if len(cur) == 0 && !b.hasTerminalPaths(cur) {
			break
		}
	}

	return cur
}

func (b *cfgbuilder) hasTerminalPaths(preds map[string]bool) bool {
	return len(preds) > 0
}

func (b *cfgbuilder) buildIf(stmt *gotreesitter.Node, preds map[string]bool) map[string]bool {
	condition := stmt.ChildByFieldName("condition", b.lang)
	consequence := stmt.ChildByFieldName("consequence", b.lang)
	alternative := stmt.ChildByFieldName("alternative", b.lang)

	condText := ""
	if condition != nil {
		condText = truncateLabel(b.bt.NodeText(condition))
	}

	decisionID := b.addNode(CFGNode{
		Type:  "decision",
		Label: condText,
		Line:  b.lineOf(stmt),
	})
	for p := range preds {
		b.addEdge(p, decisionID, "next")
	}

	var thenExits map[string]bool
	if consequence != nil {
		if consequence.Type(b.lang) == "statement_block" {
			thenExits = b.buildBlock(consequence, map[string]bool{decisionID: true})
		} else {
			thenExits = b.buildBlock(wrapInBlock(consequence), map[string]bool{decisionID: true})
		}
	} else {
		thenExits = map[string]bool{decisionID: true}
	}
	thenExits = b.relabelPreds(thenExits, decisionID)

	var elseExits map[string]bool
	if alternative != nil {
		if altBlock, ok := isBlock(alternative, b.lang); ok {
			elseExits = b.buildBlock(altBlock, map[string]bool{decisionID: true})
		} else {
			elseExits = b.buildBlock(wrapInBlock(alternative), map[string]bool{decisionID: true})
		}
	} else {
		elseExits = map[string]bool{decisionID: true}
	}
	elseExits = b.relabelPreds(elseExits, decisionID)

	merged := mergePreds(thenExits, elseExits)
	return merged
}

func (b *cfgbuilder) relabelPreds(preds map[string]bool, decisionID string) map[string]bool {
	out := make(map[string]bool, len(preds))
	for p := range preds {
		if p == decisionID {
			continue
		}
		out[p] = true
	}
	return out
}

func (b *cfgbuilder) buildSwitch(stmt *gotreesitter.Node, preds map[string]bool) map[string]bool {
	discriminant := childByIndex(stmt, 1)
	body := childByIndex(stmt, 2)

	discText := ""
	if discriminant != nil {
		discText = truncateLabel(b.bt.NodeText(discriminant))
	}

	decisionID := b.addNode(CFGNode{
		Type:  "decision",
		Label: "switch (" + discText + ")",
		Line:  b.lineOf(stmt),
	})
	for p := range preds {
		b.addEdge(p, decisionID, "next")
	}

	caseExits := make(map[string]map[string]bool)
	caseOrder := []int{}
	hasDefault := false

	if body != nil {
		for i := 0; i < int(body.ChildCount()); i++ {
			child := body.Child(i)
			if child == nil {
				continue
			}
			switch child.Type(b.lang) {
			case "case":
				hasDefault = false
				valNode := child.ChildByFieldName("value", b.lang)
				caseLabel := ""
				if valNode != nil {
					caseLabel = "case:" + truncateLabel(b.bt.NodeText(valNode))
				} else {
					caseLabel = "default"
					hasDefault = true
				}
				caseExits[caseLabel] = b.buildSwitchCase(child, decisionID)
				caseOrder = append(caseOrder, len(caseOrder))

			case "default":
				hasDefault = true
				caseLabel := "default"
				caseExits[caseLabel] = b.buildSwitchCase(child, decisionID)
				caseOrder = append(caseOrder, len(caseOrder))
			}
		}
	}

	if !hasDefault {
		caseExits["default"] = map[string]bool{decisionID: true}
		caseOrder = append(caseOrder, len(caseOrder)-1)
	}

	var allExits []map[string]bool
	for label, exits := range caseExits {
		for p := range exits {
			if p != decisionID {
				_ = label
			}
		}
		allExits = append(allExits, exits)
	}

	if len(allExits) == 0 {
		return map[string]bool{decisionID: true}
	}

	result := allExits[0]
	for i := 1; i < len(allExits); i++ {
		result = mergePreds(result, allExits[i])
	}
	return result
}

func (b *cfgbuilder) buildSwitchCase(caseNode *gotreesitter.Node, decisionID string) map[string]bool {
	exits := map[string]bool{decisionID: true}

	for i := 0; i < int(caseNode.ChildCount()); i++ {
		child := caseNode.Child(i)
		if child == nil {
			continue
		}
		switch child.Type(b.lang) {
		case "statement_block":
			exits = b.buildBlock(child, exits)
		case "return_statement":
			exits = b.buildReturn(child, exits)
		case "break_statement":
			continue
		case "throw_statement":
			exits = b.buildThrow(child, exits)
		case "expression_statement":
			exits = b.buildExpression(child, exits)
		case "variable_declaration":
			exits = b.buildVarDecl(child, exits)
		}
	}

	return exits
}

func (b *cfgbuilder) buildLoop(stmt *gotreesitter.Node, preds map[string]bool, loopType string) map[string]bool {
	loopLabel := "loop"
	if loopType == "for_statement" || loopType == "for_in_statement" || loopType == "for_of_statement" {
		loopLabel = "for"
	} else if loopType == "while_statement" {
		loopLabel = "while"
	} else if loopType == "do_statement" {
		loopLabel = "do"
	}

	loopID := b.addNode(CFGNode{
		Type:  "step",
		Label: loopLabel + " loop",
		Line:  b.lineOf(stmt),
		Call:  loopType,
	})
	for p := range preds {
		b.addEdge(p, loopID, "next")
	}

	return map[string]bool{loopID: true}
}

func (b *cfgbuilder) buildReturn(stmt *gotreesitter.Node, preds map[string]bool) map[string]bool {
	valueNode := childByIndex(stmt, 1)
	label := "return"
	if valueNode != nil {
		label = "return " + truncateLabel(b.bt.NodeText(valueNode))
	}

	termID := b.addNode(CFGNode{
		Type:  "terminal",
		Kind:  "return",
		Label: label,
		Line:  b.lineOf(stmt),
	})
	if termID == "" {
		return nil
	}

	for p := range preds {
		b.addEdge(p, termID, "next")
	}
	return nil
}

func (b *cfgbuilder) buildThrow(stmt *gotreesitter.Node, preds map[string]bool) map[string]bool {
	valueNode := childByIndex(stmt, 1)
	label := "throw"
	if valueNode != nil {
		label = "throw " + truncateLabel(b.bt.NodeText(valueNode))
	}

	termID := b.addNode(CFGNode{
		Type:  "terminal",
		Kind:  "error",
		Label: label,
		Line:  b.lineOf(stmt),
	})
	if termID == "" {
		return nil
	}

	for p := range preds {
		b.addEdge(p, termID, "next")
	}
	return nil
}

func (b *cfgbuilder) buildTry(stmt *gotreesitter.Node, preds map[string]bool) map[string]bool {
	tryID := b.addNode(CFGNode{
		Type:  "step",
		Label: "try/catch",
		Line:  b.lineOf(stmt),
	})
	for p := range preds {
		b.addEdge(p, tryID, "next")
	}
	return map[string]bool{tryID: true}
}

func (b *cfgbuilder) buildExpression(stmt *gotreesitter.Node, preds map[string]bool) map[string]bool {
	expr := childByIndex(stmt, 0)
	if expr == nil {
		return preds
	}

	if expr.Type(b.lang) == "assignment_expression" {
		return b.buildAssignment(stmt, preds)
	}

	if expr.Type(b.lang) == "call_expression" {
		text := b.bt.NodeText(expr)
		if isEarlyExitCall(text) {
			termID := b.addNode(CFGNode{
				Type: "terminal",
				Kind: "error",
				Line: b.lineOf(stmt),
			})
			if termID == "" {
				return nil
			}
			for p := range preds {
				b.addEdge(p, termID, "next")
			}
			return nil
		}

		stepID := b.addNode(CFGNode{
			Type:  "step",
			Label: truncateLabel(text),
			Line:  b.lineOf(stmt),
			Call:  extractCallName(text),
		})
		if stepID != "" {
			for p := range preds {
				b.addEdge(p, stepID, "next")
			}
			return map[string]bool{stepID: true}
		}
		return preds
	}

	if expr.Type(b.lang) == "ternary_expression" || expr.Type(b.lang) == "conditional_expression" {
		ternaryID := b.addNode(CFGNode{
			Type:  "decision",
			Label: truncateLabel(b.bt.NodeText(expr)),
			Line:  b.lineOf(stmt),
		})
		for p := range preds {
			b.addEdge(p, ternaryID, "next")
		}
		return map[string]bool{ternaryID: true}
	}

	text := b.bt.NodeText(stmt)
	if text == "" {
		return preds
	}

	stepID := b.addNode(CFGNode{
		Type:  "step",
		Label: truncateLabel(text),
		Line:  b.lineOf(stmt),
	})
	if stepID != "" {
		for p := range preds {
			b.addEdge(p, stepID, "next")
		}
		return map[string]bool{stepID: true}
	}
	return preds
}

func (b *cfgbuilder) buildAssignment(stmt *gotreesitter.Node, preds map[string]bool) map[string]bool {
	stepID := b.addNode(CFGNode{
		Type:  "step",
		Label: truncateLabel(b.bt.NodeText(stmt)),
		Line:  b.lineOf(stmt),
	})
	if stepID != "" {
		for p := range preds {
			b.addEdge(p, stepID, "next")
		}
		return map[string]bool{stepID: true}
	}
	return preds
}

func (b *cfgbuilder) buildVarDecl(stmt *gotreesitter.Node, preds map[string]bool) map[string]bool {
	stepID := b.addNode(CFGNode{
		Type:  "step",
		Label: truncateLabel(b.bt.NodeText(stmt)),
		Line:  b.lineOf(stmt),
	})
	if stepID != "" {
		for p := range preds {
			b.addEdge(p, stepID, "next")
		}
		return map[string]bool{stepID: true}
	}
	return preds
}

func (b *cfgbuilder) buildDefault(stmt *gotreesitter.Node, preds map[string]bool, stmtType string) map[string]bool {
	stepID := b.addNode(CFGNode{
		Type:  "step",
		Label: truncateLabel(b.bt.NodeText(stmt)),
		Line:  b.lineOf(stmt),
	})
	if stepID != "" {
		for p := range preds {
			b.addEdge(p, stepID, "next")
		}
		return map[string]bool{stepID: true}
	}
	return preds
}

func (x *JSControlFlowExtractor) langForExt(ext string) (*gotreesitter.Language, string) {
	switch ext {
	case ".ts":
		return x.tsLang, "typescript"
	case ".tsx":
		return x.tsxLang, "typescript"
	default:
		return x.jsLang, "javascript"
	}
}

func childByIndex(node *gotreesitter.Node, idx int) *gotreesitter.Node {
	if node == nil {
		return nil
	}
	if int(node.ChildCount()) <= idx {
		return nil
	}
	return node.Child(idx)
}

func isBlock(node *gotreesitter.Node, lang *gotreesitter.Language) (*gotreesitter.Node, bool) {
	if node == nil {
		return nil, false
	}
	if node.Type(lang) == "statement_block" {
		return node, true
	}
	return nil, false
}

func wrapInBlock(node *gotreesitter.Node) *gotreesitter.Node {
	return node
}

func mergePreds(a, b map[string]bool) map[string]bool {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	result := make(map[string]bool)
	for k := range a {
		result[k] = true
	}
	for k := range b {
		result[k] = true
	}
	return result
}

func truncateLabel(s string) string {
	runes := []rune(s)
	if len(runes) > labelMaxLen {
		return string(runes[:labelMaxLen]) + "…"
	}
	return s
}

func isEarlyExitCall(text string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(lower, ".status(") && (strings.Contains(lower, ".json(") || strings.Contains(lower, ".send(")) {
		return true
	}
	if strings.Contains(lower, ".write(") {
		return true
	}
	if strings.Contains(lower, ".end(") {
		return true
	}
	return false
}

func extractCallName(text string) string {
	parenIdx := strings.Index(text, "(")
	if parenIdx < 0 {
		return text
	}
	name := strings.TrimSpace(text[:parenIdx])
	dotIdx := strings.LastIndex(name, ".")
	if dotIdx >= 0 {
		return name[dotIdx+1:]
	}
	return name
}
