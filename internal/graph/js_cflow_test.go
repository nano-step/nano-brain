package graph_test

import (
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func newCFGExtractor(t *testing.T) *graph.JSControlFlowExtractor {
	t.Helper()
	ex, err := graph.NewJSControlFlowExtractor()
	if err != nil {
		t.Fatal(err)
	}
	return ex
}

// countNodeTypes returns a map of node Type -> count across one CFG.
func countNodeTypes(cfg graph.CFG) map[string]int {
	m := make(map[string]int)
	for _, n := range cfg.Nodes {
		m[n.Type]++
	}
	return m
}

func TestJSControlFlowExtractor_SupportsCFG(t *testing.T) {
	ex := newCFGExtractor(t)
	tests := []struct {
		ext  string
		want bool
	}{
		{".js", true},
		{".jsx", true},
		{".ts", true},
		{".tsx", true},
		{".go", false},
		{".py", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := ex.SupportsCFG(tt.ext); got != tt.want {
			t.Errorf("SupportsCFG(%q) = %v, want %v", tt.ext, got, tt.want)
		}
	}
}

func TestJSControlFlowExtractor_NoFunctions(t *testing.T) {
	ex := newCFGExtractor(t)
	cfgs, err := ex.ExtractCFGs("empty.js", []byte("const x = 1;\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 0 {
		t.Errorf("expected 0 CFGs from a file with no functions, got %d", len(cfgs))
	}
}

func TestJSControlFlowExtractor_EmptyFile(t *testing.T) {
	ex := newCFGExtractor(t)
	cfgs, err := ex.ExtractCFGs("empty.js", []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 0 {
		t.Errorf("expected 0 CFGs from empty file, got %d", len(cfgs))
	}
}

func TestJSControlFlowExtractor_SyntaxError(t *testing.T) {
	ex := newCFGExtractor(t)
	// Partial/broken source must not error or panic.
	if _, err := ex.ExtractCFGs("broken.js", []byte("function broken( { return")); err != nil {
		t.Fatalf("should not error on partial parse: %v", err)
	}
}

func TestJSControlFlowExtractor_GuardClauseProducesDecisionAndTerminals(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function purchase(req, res) {
  if (!req.id) {
    return res.status(400);
  }
  return res.status(200);
}
`
	cfgs, err := ex.ExtractCFGs("handlers/purchase.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	cfg := cfgs[0]

	if want := "handlers/purchase.ts::purchase"; cfg.Entry != want {
		t.Errorf("Entry = %q, want %q", cfg.Entry, want)
	}
	if cfg.SourceFile != "handlers/purchase.ts" {
		t.Errorf("SourceFile = %q, want handlers/purchase.ts", cfg.SourceFile)
	}

	counts := countNodeTypes(cfg)
	if counts["decision"] < 1 {
		t.Errorf("expected at least 1 decision node, got %d", counts["decision"])
	}
	if counts["terminal"] < 1 {
		t.Errorf("expected at least 1 terminal node, got %d", counts["terminal"])
	}

	// At least one return terminal must carry a descriptive, non-empty label.
	foundLabeledReturn := false
	for _, n := range cfg.Nodes {
		if n.Type == "terminal" && n.Kind == "return" && strings.HasPrefix(n.Label, "return") {
			foundLabeledReturn = true
		}
	}
	if !foundLabeledReturn {
		t.Error("expected a return terminal with a 'return ...' label")
	}
}

func TestJSControlFlowExtractor_ThrowTerminal(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function validate(x) {
  if (!x) {
    throw new Error("missing x");
  }
  return x;
}
`
	cfgs, err := ex.ExtractCFGs("v.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}

	foundError := false
	for _, n := range cfgs[0].Nodes {
		if n.Type == "terminal" && n.Kind == "error" && strings.HasPrefix(n.Label, "throw") {
			foundError = true
		}
	}
	if !foundError {
		t.Error("expected a throw terminal with kind=error and a 'throw ...' label")
	}
}

func TestJSControlFlowExtractor_SwitchProducesDecision(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function route(x) {
  switch (x) {
    case 1: return "a";
    case 2: return "b";
    default: return "c";
  }
}
`
	cfgs, err := ex.ExtractCFGs("r.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}

	foundSwitch := false
	for _, n := range cfgs[0].Nodes {
		if n.Type == "decision" && strings.HasPrefix(n.Label, "switch (") {
			foundSwitch = true
		}
	}
	if !foundSwitch {
		t.Error("expected a decision node labeled 'switch (...)'")
	}
}

func TestJSControlFlowExtractor_BatchFiveFunctions(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function a() { return 1; }
function b() { return 2; }
function c() { return 3; }
function d() { return 4; }
function e() { return 5; }
`
	cfgs, err := ex.ExtractCFGs("multi.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 5 {
		t.Fatalf("expected 5 CFGs (one per function), got %d", len(cfgs))
	}
	for _, cfg := range cfgs {
		if len(cfg.Nodes) == 0 {
			t.Errorf("CFG %q has no nodes", cfg.Entry)
		}
		if cfg.Status != "complete" {
			t.Errorf("CFG %q status = %q, want complete", cfg.Entry, cfg.Status)
		}
	}
}

func TestJSControlFlowExtractor_LargeFunctionTruncated(t *testing.T) {
	ex := newCFGExtractor(t)
	var b strings.Builder
	b.WriteString("function big() {\n")
	for i := 0; i < 700; i++ {
		b.WriteString("  doThing();\n")
	}
	b.WriteString("}\n")

	cfgs, err := ex.ExtractCFGs("big.js", []byte(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	cfg := cfgs[0]
	if cfg.Status != "truncated" {
		t.Errorf("status = %q, want truncated", cfg.Status)
	}
	if len(cfg.Nodes) > 500 {
		t.Errorf("node count %d exceeds the 500 cap", len(cfg.Nodes))
	}
}

func TestJSControlFlowExtractor_NoBranchesYieldsEmptyCFG(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `function identity(x) { let y = x + 1; return y; }`
	cfgs, err := ex.ExtractCFGs("simple.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	cfg := cfgs[0]
	if cfg.Status != "complete" {
		t.Errorf("status = %q, want complete", cfg.Status)
	}
	counts := countNodeTypes(cfg)
	if counts["decision"] > 0 {
		t.Errorf("expected 0 decision nodes for a function with no branches, got %d", counts["decision"])
	}
}

func TestJSControlFlowExtractor_EarlyReturnIdiom(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function handler(req, res) {
  if (err) return;
  res.json({ ok: true });
}
`
	cfgs, err := ex.ExtractCFGs("handler.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	cfg := cfgs[0]
	foundDecision := false
	foundTerminal := false
	for _, n := range cfg.Nodes {
		if n.Type == "decision" {
			foundDecision = true
		}
		if n.Type == "terminal" {
			foundTerminal = true
		}
	}
	if !foundDecision {
		t.Error("expected a decision node for the early-return guard")
	}
	if !foundTerminal {
		t.Error("expected a terminal node for the early return")
	}
}

func TestJSControlFlowExtractor_ArrowFunctionAssigned(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
const handler = (req, res) => {
  if (!req.ok) {
    return res.status(400);
  }
  res.send("ok");
};
`
	cfgs, err := ex.ExtractCFGs("a.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG for the assigned arrow function, got %d", len(cfgs))
	}
	if cfgs[0].Entry != "a.ts::handler" {
		t.Errorf("Entry = %q, want a.ts::handler", cfgs[0].Entry)
	}
}

func TestJSControlFlowExtractor_NoJunkNodes(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function clean(req, res) {
  // just a last line of defense
  if (!req.id) {
    return res.status(400);
  }
  return res.status(200);
}
`
	cfgs, err := ex.ExtractCFGs("clean.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	for _, n := range cfgs[0].Nodes {
		if n.Label == "{" || n.Label == "}" || n.Label == ";" {
			t.Errorf("junk node found: type=%q label=%q", n.Type, n.Label)
		}
		if n.Type == "step" && strings.HasPrefix(n.Label, "//") {
			t.Errorf("comment node found as step: label=%q", n.Label)
		}
	}
}

func TestJSControlFlowExtractor_StartNodeLabelIsFunctionName(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `function foo() { return 1; }`
	cfgs, err := ex.ExtractCFGs("handlers/test.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	startNodes := 0
	for _, n := range cfgs[0].Nodes {
		if n.Type == "start" {
			startNodes++
			if n.Label != "foo" {
				t.Errorf("start node label = %q, want %q", n.Label, "foo")
			}
		}
	}
	if startNodes != 1 {
		t.Errorf("expected 1 start node, got %d", startNodes)
	}
}

func TestJSControlFlowExtractor_AbsolutePathStripped(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `function bar() { return 2; }`
	cfgs, err := ex.ExtractCFGs("/Users/test/work/project/src/handler.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	if cfgs[0].SourceFile != "handler.ts" {
		t.Errorf("SourceFile = %q, want handler.ts", cfgs[0].SourceFile)
	}
	if cfgs[0].Entry != "handler.ts::bar" {
		t.Errorf("Entry = %q, want handler.ts::bar", cfgs[0].Entry)
	}
}

func TestJSControlFlowExtractor_WrappedArrowHandler(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
const getBalance = catchAsync(async (req, res) => {
  if (!req.user) {
    return res.status(401);
  }
  res.json({ balance: 100 });
});
`
	cfgs, err := ex.ExtractCFGs("routes.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG for wrapped arrow handler, got %d", len(cfgs))
	}
	if cfgs[0].Entry != "routes.ts::getBalance" {
		t.Errorf("Entry = %q, want routes.ts::getBalance", cfgs[0].Entry)
	}
	counts := countNodeTypes(cfgs[0])
	if counts["decision"] < 1 {
		t.Error("expected at least 1 decision node for the guard clause")
	}
}

// countEdgeBranches returns a map of branch label -> count across one CFG.
func countEdgeBranches(cfg graph.CFG) map[string]int {
	m := make(map[string]int)
	for _, e := range cfg.Edges {
		m[e.Branch]++
	}
	return m
}

func TestJSControlFlowExtractor_IfElseBranchLabels(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function test(x) {
  if (x > 0) {
    console.log("pos");
  } else {
    console.log("neg");
  }
}
`
	cfgs, err := ex.ExtractCFGs("f.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	branches := countEdgeBranches(cfgs[0])
	if branches["yes"] < 1 {
		t.Error("expected at least 1 'yes' branch edge for if-then")
	}
	if branches["no"] < 1 {
		t.Error("expected at least 1 'no' branch edge for if-else")
	}
}

func TestJSControlFlowExtractor_IfOnlyBranchLabels(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function test(x) {
  if (x > 0) {
    console.log("pos");
  }
  console.log("done");
}
`
	cfgs, err := ex.ExtractCFGs("f.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	branches := countEdgeBranches(cfgs[0])
	if branches["yes"] < 1 {
		t.Error("expected at least 1 'yes' branch edge for if-then")
	}
	// No else block means no "no" edge — the decision falls through implicitly.
	if branches["no"] > 0 {
		t.Error("expected 0 'no' branch edges when there is no else block")
	}
}

func TestJSControlFlowExtractor_IfElseIfBranchLabels(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function test(x) {
  if (x > 0) {
    console.log("pos");
  } else if (x < 0) {
    console.log("neg");
  } else {
    console.log("zero");
  }
}
`
	cfgs, err := ex.ExtractCFGs("f.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	branches := countEdgeBranches(cfgs[0])
	if branches["yes"] < 2 {
		t.Error("expected at least 2 'yes' branch edges for chained decisions")
	}
	if branches["no"] < 1 {
		t.Error("expected at least 1 'no' branch edge for else/else-if")
	}
}

func TestJSControlFlowExtractor_SwitchBranchLabels(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function route(x) {
  switch (x) {
    case 1: return "a";
    case 2: return "b";
    default: return "c";
  }
}
`
	cfgs, err := ex.ExtractCFGs("r.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	branches := countEdgeBranches(cfgs[0])
	if branches["case:1"] < 1 {
		t.Error("expected 'case:1' branch edge")
	}
	if branches["case:2"] < 1 {
		t.Error("expected 'case:2' branch edge")
	}
}

func TestJSControlFlowExtractor_SwitchDefaultBranchLabels(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function route(x) {
  switch (x) {
    case 1: return "a";
    default: return "b";
  }
}
`
	cfgs, err := ex.ExtractCFGs("r.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	branches := countEdgeBranches(cfgs[0])
	if branches["case:1"] < 1 {
		t.Error("expected 'case:1' branch edge")
	}
	if branches["default"] < 1 {
		t.Error("expected 'default' branch edge")
	}
}

func TestJSControlFlowExtractor_SwitchNoDefaultBranchLabels(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function route(x) {
  switch (x) {
    case 1: return "a";
    case 2: return "b";
  }
}
`
	cfgs, err := ex.ExtractCFGs("r.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	branches := countEdgeBranches(cfgs[0])
	if branches["case:1"] < 1 {
		t.Error("expected 'case:1' branch edge")
	}
	if branches["case:2"] < 1 {
		t.Error("expected 'case:2' branch edge")
	}
	// No default case and no explicit default edge.
	if branches["default"] > 0 {
		t.Error("expected no 'default' branch edge when no default case exists")
	}
}

func TestJSControlFlowExtractor_TernaryBranchLabels(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function test(x) {
  x > 0 ? console.log("pos") : console.log("neg");
}
`
	cfgs, err := ex.ExtractCFGs("f.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	branches := countEdgeBranches(cfgs[0])
	if branches["yes"] < 1 {
		t.Error("expected at least 1 'yes' branch edge for ternary then-branch")
	}
	if branches["no"] < 1 {
		t.Error("expected at least 1 'no' branch edge for ternary else-branch")
	}
}

func TestJSControlFlowExtractor_NestedArrowFunctionNames(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
const outer = () => {
  const inner = () => {
    doSomething();
  };
};
`
	cfgs, err := ex.ExtractCFGs("f.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 2 {
		t.Fatalf("expected 2 CFGs (outer + inner), got %d", len(cfgs))
	}
	names := make(map[string]bool)
	for _, c := range cfgs {
		names[c.Entry] = true
	}
	if !names["f.js::outer"] {
		t.Error("expected a CFG for outer")
	}
	if !names["f.js::inner"] {
		t.Error("expected a CFG for inner")
	}
}

func TestJSControlFlowExtractor_TryCatchEmptyBlocks(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function test() {
  try {
  } catch (e) {
  }
}
`
	cfgs, err := ex.ExtractCFGs("f.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	cfg := cfgs[0]
	if len(cfg.Nodes) < 2 {
		t.Fatalf("expected at least 2 nodes (start + try/catch), got %d", len(cfg.Nodes))
	}
	if len(cfg.Edges) < 1 {
		t.Fatal("expected at least 1 edge (start -> try/catch), got 0")
	}
	tryCatchNode := false
	for _, n := range cfg.Nodes {
		if n.Type == "decision" && n.Label == "try/catch" {
			tryCatchNode = true
		}
	}
	if !tryCatchNode {
		t.Error("expected a try/catch decision node")
	}
}

func TestJSControlFlowExtractor_TryCatchBranchLabels(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function test() {
  try {
    risky();
  } catch (e) {
    handle(e);
  }
}
`
	cfgs, err := ex.ExtractCFGs("f.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	branches := countEdgeBranches(cfgs[0])
	if branches["try"] < 1 {
		t.Error("expected at least 1 'try' branch edge")
	}
	if branches["catch"] < 1 {
		t.Error("expected at least 1 'catch' branch edge")
	}
}

func TestJSControlFlowExtractor_GuardClauseContinuation(t *testing.T) {
	src := []byte(`
function example(req) {
  if (!req.id) {
    return { success: false };
  }
  const data = fetchData(req.id);
  return { success: true, data };
}`)
	extractor := newCFGExtractor(t)
	cfgs, err := extractor.ExtractCFGs("test.js", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) == 0 {
		t.Fatal("expected at least 1 CFG")
	}
	cfg := cfgs[0]
	if len(cfg.Nodes) < 4 {
		t.Errorf("expected at least 4 nodes, got %d", len(cfg.Nodes))
	}

	hasIncoming := map[string]bool{}
	for _, e := range cfg.Edges {
		hasIncoming[e.To] = true
	}
	hasStep := false
	for _, n := range cfg.Nodes {
		if n.Type == "step" {
			hasStep = true
			if !hasIncoming[n.ID] {
				t.Errorf("step node %s has no incoming edge — continuation is disconnected", n.ID)
			}
		}
	}
	if !hasStep {
		t.Error("expected at least one step node for fetchData")
	}
}

func TestJSControlFlowExtractor_NestedGuardClauses(t *testing.T) {
	src := []byte(`
function validate(a, b) {
  if (!a) { return { error: 'a missing' }; }
  if (!b) { return { error: 'b missing' }; }
  return { ok: true, a, b };
}`)
	extractor := newCFGExtractor(t)
	cfgs, err := extractor.ExtractCFGs("test.js", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) == 0 {
		t.Fatal("expected at least 1 CFG")
	}
	cfg := cfgs[0]
	terminals := 0
	for _, n := range cfg.Nodes {
		if n.Type == "terminal" {
			terminals++
		}
	}
	if terminals < 3 {
		t.Errorf("expected at least 3 terminal nodes (2 guard returns + 1 success), got %d", terminals)
	}
}

func TestJSControlFlowExtractor_IfWithoutElseThenContinues(t *testing.T) {
	src := []byte(`
function compute(x) {
  if (x > 0) {
    x = x * 2;
  }
  return x;
}`)
	extractor := newCFGExtractor(t)
	cfgs, err := extractor.ExtractCFGs("test.js", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) == 0 {
		t.Fatal("expected at least 1 CFG")
	}
	cfg := cfgs[0]
	if len(cfg.Nodes) < 4 {
		t.Errorf("expected at least 4 nodes, got %d", len(cfg.Nodes))
	}
	hasIncoming := map[string]bool{}
	for _, e := range cfg.Edges {
		hasIncoming[e.To] = true
	}
	for _, n := range cfg.Nodes {
		if n.Type == "terminal" {
			if !hasIncoming[n.ID] {
				t.Errorf("terminal node %s has no incoming edge — unreachable", n.ID)
			}
		}
	}
}

func TestJSControlFlowExtractor_TryCatchFinallyBranchLabels(t *testing.T) {
	ex := newCFGExtractor(t)
	src := `
function test() {
  try {
    risky();
  } catch (e) {
    handle(e);
  } finally {
    cleanup();
  }
}
`
	cfgs, err := ex.ExtractCFGs("f.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	branches := countEdgeBranches(cfgs[0])
	if branches["try"] < 1 {
		t.Error("expected at least 1 'try' branch edge")
	}
	if branches["catch"] < 1 {
		t.Error("expected at least 1 'catch' branch edge")
	}
}
