package graph

import (
	"fmt"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"github.com/rs/zerolog"
)

var _ ControlFlowExtractor = (*RubyControlFlowExtractor)(nil)

type RubyControlFlowExtractor struct {
	lang *gotreesitter.Language
	log  zerolog.Logger
}

func NewRubyControlFlowExtractor(logger zerolog.Logger) (*RubyControlFlowExtractor, error) {
	return &RubyControlFlowExtractor{
		lang: grammars.RubyLanguage(),
		log:  logger.With().Str("component", "ruby-cfg-extractor").Logger(),
	}, nil
}

func (x *RubyControlFlowExtractor) SupportsCFG(ext string) bool {
	return ext == ".rb"
}

func (x *RubyControlFlowExtractor) ExtractCFGs(filePath string, content []byte) ([]CFG, error) {
	parser := gotreesitter.NewParser(x.lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	root := tree.RootNode()
	relFile := filepath.ToSlash(filePath)
	if filepath.IsAbs(relFile) {
		relFile = filepath.Base(relFile)
	}

	var methods []funcDef
	collectMethods := func(n *gotreesitter.Node) {
		nameNode := n.ChildByFieldName("name", x.lang)
		bodyNode := n.ChildByFieldName("body", x.lang)
		if nameNode == nil || bodyNode == nil {
			return
		}
		methods = append(methods, funcDef{
			name:      bt.NodeText(nameNode),
			startLine: lineForByte(content, n.StartByte()),
			endLine:   lineForByte(content, n.EndByte()),
			bodyNode:  bodyNode,
		})
	}
	walkNodes(root, x.lang, "method", collectMethods)
	walkNodes(root, x.lang, "singleton_method", collectMethods)

	var cfgs []CFG
	for _, md := range methods {
		cfg, err := x.extractOne(bt, content, relFile, md)
		if err != nil {
			return nil, fmt.Errorf("extract cfg %s::%s: %w", relFile, md.name, err)
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

func (x *RubyControlFlowExtractor) extractOne(bt *gotreesitter.BoundTree, content []byte, relFile string, md funcDef) (*CFG, error) {
	entry := relFile + "::" + md.name

	builder := &rubyCfgBuilder{
		bt:      bt,
		lang:    x.lang,
		content: content,
		relFile: relFile,
		entry:   entry,
		nodeID:  0,
		cfg: &CFG{
			Entry:      entry,
			SourceFile: relFile,
			StartLine:  md.startLine,
			EndLine:    md.endLine,
			Status:     "complete",
		},
	}

	startNode := builder.addNode(CFGNode{
		Type:  "start",
		Label: md.name,
		Line:  md.startLine,
	})

	exits := builder.buildBodyStatement(md.bodyNode, map[string]bool{startNode: true}, nil)
	if len(exits) > 0 {
		builder.addMergeToTerminal(exits)
	}

	if builder.capped {
		builder.cfg.Status = "truncated"
	}

	if len(builder.cfg.Nodes) == 1 {
		return nil, nil
	}

	return builder.cfg, nil
}

// ─── CFG builder ────────────────────────────────────────────────────────────

type rubyCfgBuilder struct {
	bt              *gotreesitter.BoundTree
	lang            *gotreesitter.Language
	content         []byte
	relFile         string
	entry           string
	nodeID          int
	cfg             *CFG
	capped          bool
	branchOverrides map[string]string
}

func (b *rubyCfgBuilder) lineOf(n *gotreesitter.Node) int {
	return lineForByte(b.content, n.StartByte())
}

func (b *rubyCfgBuilder) nextID() string {
	id := fmt.Sprintf("n%d", b.nodeID)
	b.nodeID++
	return id
}

func (b *rubyCfgBuilder) addNode(n CFGNode) string {
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

func (b *rubyCfgBuilder) addEdge(from, to, branch string) {
	if from == "" || to == "" {
		return
	}
	if b.branchOverrides != nil {
		if override, ok := b.branchOverrides[from]; ok {
			branch = override
		}
	}
	b.cfg.Edges = append(b.cfg.Edges, CFGEdge{
		From:   from,
		To:     to,
		Branch: branch,
	})
}

func (b *rubyCfgBuilder) addMergeToTerminal(preds map[string]bool) {
	termID := b.addNode(CFGNode{
		Type: "terminal",
		Kind: "return",
		Line: b.cfg.EndLine,
	})
	for p := range preds {
		b.addEdge(p, termID, "next")
	}
}

func (b *rubyCfgBuilder) buildBodyStatement(node *gotreesitter.Node, preds map[string]bool, branchOverrides map[string]string) map[string]bool {
	if node == nil {
		return preds
	}

	oldOverrides := b.branchOverrides
	b.branchOverrides = branchOverrides

	cur := preds
	childCount := int(node.ChildCount())

	for i := 0; i < childCount; i++ {
		stmt := node.Child(i)
		if stmt == nil {
			continue
		}
		stmtType := stmt.Type(b.lang)

		if isRubyIgnoredStatement(stmtType) {
			continue
		}

		switch stmtType {
		case "if":
			cur = b.buildIf(stmt, cur)
		case "while", "until", "for":
			cur = b.buildLoop(stmt, cur)
		case "begin":
			cur = b.buildBegin(stmt, cur)
		case "return":
			cur = b.buildReturn(stmt, cur)
		case "break":
			cur = b.buildBreak(stmt, cur)
		case "next":
			cur = b.buildNext(stmt, cur)
		case "call":
			cur = b.buildCall(stmt, cur)
		case "assignment":
			cur = b.buildAssignment(stmt, cur)
		default:
			cur = b.buildStep(stmt, cur)
		}

		if len(cur) == 0 {
			break
		}
	}

	b.branchOverrides = oldOverrides
	return cur
}

func (b *rubyCfgBuilder) buildIf(node *gotreesitter.Node, preds map[string]bool) map[string]bool {
	condition := node.ChildByFieldName("condition", b.lang)
	consequence := node.ChildByFieldName("consequence", b.lang)
	alternative := node.ChildByFieldName("alternative", b.lang)

	condText := ""
	if condition != nil {
		condText = truncateLabel(b.bt.NodeText(condition))
	}

	decisionID := b.addNode(CFGNode{
		Type:  "decision",
		Label: condText,
		Line:  b.lineOf(node),
	})
	for p := range preds {
		b.addEdge(p, decisionID, "next")
	}

	var thenExits map[string]bool
	if consequence != nil {
		body := b.unwrapThenDo(consequence)
		if body != nil {
			thenExits = b.buildBodyStatement(body, map[string]bool{decisionID: true}, map[string]string{decisionID: "yes"})
		} else {
			thenExits = map[string]bool{decisionID: true}
		}
		if len(thenExits) > 1 || (len(thenExits) == 1 && !thenExits[decisionID]) {
			thenExits = b.relabelPreds(thenExits, decisionID)
		}
	} else {
		thenExits = map[string]bool{decisionID: true}
	}

	var elseExits map[string]bool
	if alternative != nil {
		altType := alternative.Type(b.lang)
		switch altType {
		case "elsif":
			elseExits = b.buildIf(alternative, map[string]bool{decisionID: true})
			elseExits = b.relabelPreds(elseExits, decisionID)
			for e := range b.cfg.Edges {
				if b.cfg.Edges[e].From == decisionID && b.cfg.Edges[e].Branch == "next" {
					b.cfg.Edges[e].Branch = "no"
					break
				}
			}
		case "else":
			body := b.unwrapThenDo(alternative)
			if body != nil {
				elseExits = b.buildBodyStatement(body, map[string]bool{decisionID: true}, map[string]string{decisionID: "no"})
			} else {
				elseExits = map[string]bool{decisionID: true}
			}
			if len(elseExits) > 1 || (len(elseExits) == 1 && !elseExits[decisionID]) {
				elseExits = b.relabelPreds(elseExits, decisionID)
			}
		default:
			elseExits = map[string]bool{decisionID: true}
		}
	} else {
		elseExits = map[string]bool{decisionID: true}
	}

	return mergePreds(thenExits, elseExits)
}

func (b *rubyCfgBuilder) relabelPreds(preds map[string]bool, decisionID string) map[string]bool {
	out := make(map[string]bool, len(preds))
	for p := range preds {
		if p == decisionID {
			continue
		}
		out[p] = true
	}
	return out
}

func (b *rubyCfgBuilder) buildLoop(node *gotreesitter.Node, preds map[string]bool) map[string]bool {
	loopType := node.Type(b.lang)
	loopLabel := "loop"
	switch loopType {
	case "while":
		loopLabel = "while"
	case "until":
		loopLabel = "until"
	case "for":
		loopLabel = "for"
	}

	condition := node.ChildByFieldName("condition", b.lang)
	body := node.ChildByFieldName("body", b.lang)

	condText := ""
	if condition != nil {
		condText = truncateLabel(b.bt.NodeText(condition))
	}

	decisionID := b.addNode(CFGNode{
		Type:  "decision",
		Label: loopLabel + " " + condText,
		Line:  b.lineOf(node),
	})
	for p := range preds {
		b.addEdge(p, decisionID, "next")
	}

	var loopExits map[string]bool
	if body != nil {
		loopBody := b.unwrapThenDo(body)
		if loopBody != nil {
			loopExits = b.buildBodyStatement(loopBody, map[string]bool{decisionID: true}, map[string]string{decisionID: "loop"})
		} else {
			loopExits = map[string]bool{decisionID: true}
		}
	} else {
		loopExits = map[string]bool{decisionID: true}
	}

	for p := range loopExits {
		b.addEdge(p, decisionID, "loop")
	}

	return map[string]bool{decisionID: true}
}

func (b *rubyCfgBuilder) buildBegin(node *gotreesitter.Node, preds map[string]bool) map[string]bool {
	beginID := b.addNode(CFGNode{
		Type:  "step",
		Label: "begin",
		Line:  b.lineOf(node),
	})
	for p := range preds {
		b.addEdge(p, beginID, "next")
	}

	var bodyStmts []*gotreesitter.Node
	var rescueClauses []*gotreesitter.Node
	var ensureNode *gotreesitter.Node

	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type(b.lang) {
		case "begin", "end":
			continue
		case "rescue":
			rescueClauses = append(rescueClauses, child)
		case "ensure":
			ensureNode = child
		default:
			bodyStmts = append(bodyStmts, child)
		}
	}

	bodyPreds := map[string]bool{beginID: true}
	for _, stmt := range bodyStmts {
		if isRubyIgnoredStatement(stmt.Type(b.lang)) {
			continue
		}
		switch stmt.Type(b.lang) {
		case "if":
			bodyPreds = b.buildIf(stmt, bodyPreds)
		case "while", "until", "for":
			bodyPreds = b.buildLoop(stmt, bodyPreds)
		case "begin":
			bodyPreds = b.buildBegin(stmt, bodyPreds)
		case "return":
			bodyPreds = b.buildReturn(stmt, bodyPreds)
		case "break":
			bodyPreds = b.buildBreak(stmt, bodyPreds)
		case "next":
			bodyPreds = b.buildNext(stmt, bodyPreds)
		case "call":
			bodyPreds = b.buildCall(stmt, bodyPreds)
		case "assignment":
			bodyPreds = b.buildAssignment(stmt, bodyPreds)
		default:
			bodyPreds = b.buildStep(stmt, bodyPreds)
		}
	}

	var rescueExits map[string]bool
	for _, rescue := range rescueClauses {
		rescueBody := rescue.ChildByFieldName("body", b.lang)
		if rescueBody != nil {
			rescueExits = b.buildBodyStatement(rescueBody, map[string]bool{beginID: true}, map[string]string{beginID: "error"})
		} else {
			rescueExits = map[string]bool{beginID: true}
		}
	}

	merged := mergePreds(bodyPreds, rescueExits)

	if ensureNode != nil {
		ensureBody := b.unwrapThenDo(ensureNode)
		if ensureBody != nil && len(merged) > 0 {
			merged = b.buildBodyStatement(ensureBody, merged, nil)
		}
	}

	return merged
}

func (b *rubyCfgBuilder) buildReturn(node *gotreesitter.Node, preds map[string]bool) map[string]bool {
	label := "return"
	if argList := node.ChildByFieldName("arguments", b.lang); argList != nil {
		label = "return " + truncateLabel(b.bt.NodeText(argList))
	} else if node.ChildCount() > 1 {
		arg := node.Child(1)
		if arg != nil {
			label = "return " + truncateLabel(b.bt.NodeText(arg))
		}
	}

	termID := b.addNode(CFGNode{
		Type:  "terminal",
		Kind:  "return",
		Label: label,
		Line:  b.lineOf(node),
	})
	if termID == "" {
		return nil
	}

	for p := range preds {
		b.addEdge(p, termID, "next")
	}
	return nil
}

func (b *rubyCfgBuilder) buildBreak(node *gotreesitter.Node, preds map[string]bool) map[string]bool {
	termID := b.addNode(CFGNode{
		Type:  "terminal",
		Kind:  "return",
		Label: "break",
		Line:  b.lineOf(node),
	})
	if termID == "" {
		return nil
	}

	for p := range preds {
		b.addEdge(p, termID, "next")
	}
	return nil
}

func (b *rubyCfgBuilder) buildNext(node *gotreesitter.Node, preds map[string]bool) map[string]bool {
	termID := b.addNode(CFGNode{
		Type:  "terminal",
		Kind:  "return",
		Label: "next",
		Line:  b.lineOf(node),
	})
	if termID == "" {
		return nil
	}

	for p := range preds {
		b.addEdge(p, termID, "next")
	}
	return nil
}

func (b *rubyCfgBuilder) buildCall(node *gotreesitter.Node, preds map[string]bool) map[string]bool {
	text := b.bt.NodeText(node)

	if methodNode := node.ChildByFieldName("method", b.lang); methodNode != nil {
		methodName := b.bt.NodeText(methodNode)
		if methodName == "raise" {
			return b.buildRaise(node, preds)
		}
	}

	stepID := b.addNode(CFGNode{
		Type:  "step",
		Label: truncateLabel(text),
		Line:  b.lineOf(node),
		Call:  extractRubyCallName(text),
	})
	if stepID != "" {
		for p := range preds {
			b.addEdge(p, stepID, "next")
		}
		return map[string]bool{stepID: true}
	}
	return preds
}

func (b *rubyCfgBuilder) buildRaise(node *gotreesitter.Node, preds map[string]bool) map[string]bool {
	text := b.bt.NodeText(node)
	termID := b.addNode(CFGNode{
		Type:  "terminal",
		Kind:  "error",
		Label: truncateLabel(text),
		Line:  b.lineOf(node),
	})
	if termID == "" {
		return nil
	}

	for p := range preds {
		b.addEdge(p, termID, "next")
	}
	return nil
}

func (b *rubyCfgBuilder) buildAssignment(node *gotreesitter.Node, preds map[string]bool) map[string]bool {
	stepID := b.addNode(CFGNode{
		Type:  "step",
		Label: truncateLabel(b.bt.NodeText(node)),
		Line:  b.lineOf(node),
	})
	if stepID != "" {
		for p := range preds {
			b.addEdge(p, stepID, "next")
		}
		return map[string]bool{stepID: true}
	}
	return preds
}

func (b *rubyCfgBuilder) buildStep(node *gotreesitter.Node, preds map[string]bool) map[string]bool {
	text := b.bt.NodeText(node)
	if text == "" {
		return preds
	}

	stepID := b.addNode(CFGNode{
		Type:  "step",
		Label: truncateLabel(text),
		Line:  b.lineOf(node),
	})
	if stepID != "" {
		for p := range preds {
			b.addEdge(p, stepID, "next")
		}
		return map[string]bool{stepID: true}
	}
	return preds
}

func (b *rubyCfgBuilder) unwrapThenDo(node *gotreesitter.Node) *gotreesitter.Node {
	if node == nil {
		return nil
	}
	return node
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func isRubyIgnoredStatement(stmtType string) bool {
	switch stmtType {
	case "def", "end", "do", "then", "else", "elsif",
		"(", ")", "[", "]", "{", "}", ",", ";",
		"comment", "line_comment", "block_comment",
		"->", "=>", "::", ".", ":",
		"hash_key_symbol", "simple_symbol",
		"exception_variable":
		return true
	default:
		return false
	}
}

func extractRubyCallName(text string) string {
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
