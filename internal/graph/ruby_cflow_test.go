package graph_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/rs/zerolog"
)

func newRubyCFGExtractor(t *testing.T) *graph.RubyControlFlowExtractor {
	t.Helper()
	log := zerolog.Nop()
	ex, err := graph.NewRubyControlFlowExtractor(log)
	if err != nil {
		t.Fatal(err)
	}
	return ex
}

func TestRubyControlFlowExtractor_SupportsCFG(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	tests := []struct {
		ext  string
		want bool
	}{
		{".rb", true},
		{".go", false},
		{".js", false},
		{".ts", false},
		{".py", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := ex.SupportsCFG(tt.ext); got != tt.want {
			t.Errorf("SupportsCFG(%q) = %v, want %v", tt.ext, got, tt.want)
		}
	}
}

func TestRubyControlFlowExtractor_NoMethods(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	cfgs, err := ex.ExtractCFGs("empty.rb", []byte("x = 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 0 {
		t.Errorf("expected 0 CFGs from a file with no methods, got %d", len(cfgs))
	}
}

func TestRubyControlFlowExtractor_EmptyFile(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	cfgs, err := ex.ExtractCFGs("empty.rb", []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 0 {
		t.Errorf("expected 0 CFGs from empty file, got %d", len(cfgs))
	}
}

func TestRubyControlFlowExtractor_SyntaxError(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	if _, err := ex.ExtractCFGs("broken.rb", []byte("def broken( { return")); err != nil {
		t.Fatalf("should not error on partial parse: %v", err)
	}
}

func TestRubyControlFlowExtractor_BasicIfElse(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	src := []byte(`class UsersController < ApplicationController
  def index
    if params[:admin]
      @users = User.admins
    else
      @users = User.all
    end
  end
end
`)
	cfgs, err := ex.ExtractCFGs("app/controllers/users_controller.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	cfg := cfgs[0]

	if want := "app/controllers/users_controller.rb::index"; cfg.Entry != want {
		t.Errorf("Entry = %q, want %q", cfg.Entry, want)
	}
	if cfg.SourceFile != "app/controllers/users_controller.rb" {
		t.Errorf("SourceFile = %q, want app/controllers/users_controller.rb", cfg.SourceFile)
	}

	counts := countNodeTypes(cfg)
	if counts["decision"] < 1 {
		t.Errorf("expected at least 1 decision node, got %d", counts["decision"])
	}
	if counts["terminal"] < 1 {
		t.Errorf("expected at least 1 terminal node, got %d", counts["terminal"])
	}

	branches := countEdgeBranches(cfg)
	if branches["yes"] < 1 {
		t.Error("expected at least 1 'yes' branch edge")
	}
	if branches["no"] < 1 {
		t.Error("expected at least 1 'no' branch edge")
	}
}

func TestRubyControlFlowExtractor_LoopWhile(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	src := []byte(`class ImportService
  def process
    while has_more?
      batch = fetch_batch
      import_batch(batch)
    end
  end
end
`)
	cfgs, err := ex.ExtractCFGs("app/services/import_service.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	cfg := cfgs[0]

	counts := countNodeTypes(cfg)
	if counts["decision"] < 1 {
		t.Errorf("expected at least 1 decision node for while loop, got %d", counts["decision"])
	}

	foundWhile := false
	for _, n := range cfg.Nodes {
		if n.Type == "decision" && strings.HasPrefix(n.Label, "while") {
			foundWhile = true
		}
	}
	if !foundWhile {
		t.Error("expected a decision node labeled 'while ...'")
	}

	branches := countEdgeBranches(cfg)
	if branches["loop"] < 1 {
		t.Error("expected at least 1 'loop' branch edge")
	}
}

func TestRubyControlFlowExtractor_BeginRescue(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	src := []byte(`class UserService
  def find_user(id)
    begin
      user = User.find(id)
    rescue ActiveRecord::RecordNotFound
      render plain: "Not found", status: 404
    end
  end
end
`)
	cfgs, err := ex.ExtractCFGs("app/services/user_service.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	cfg := cfgs[0]

	foundBegin := false
	for _, n := range cfg.Nodes {
		if n.Type == "step" && n.Label == "begin" {
			foundBegin = true
		}
	}
	if !foundBegin {
		t.Error("expected a 'begin' step node")
	}

	branches := countEdgeBranches(cfg)
	if branches["error"] < 1 {
		t.Error("expected at least 1 'error' branch edge for rescue")
	}
}

func TestRubyControlFlowExtractor_MultipleMethods(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	src := []byte(`class UsersController < ApplicationController
  def index
    @users = User.all
  end

  def show
    @user = User.find(params[:id])
  end

  def create
    if user_params[:name].empty?
      render plain: "Name required", status: 422
    else
      @user = User.new(user_params)
      @user.save
    end
  end
end
`)
	cfgs, err := ex.ExtractCFGs("app/controllers/users_controller.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 3 {
		t.Fatalf("expected 3 CFGs (one per method), got %d", len(cfgs))
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

func TestRubyControlFlowExtractor_LargeMethodTruncated(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	var b strings.Builder
	b.WriteString("def big\n")
	for i := 0; i < 260; i++ {
		b.WriteString(fmt.Sprintf("  if x > %d\n", i))
		b.WriteString("    self.do_positive\n")
		b.WriteString("  else\n")
		b.WriteString("    self.do_negative\n")
		b.WriteString("  end\n")
	}
	b.WriteString("end\n")

	cfgs, err := ex.ExtractCFGs("big.rb", []byte(b.String()))
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

func TestRubyControlFlowExtractor_SkipNonRubyFiles(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	if ex.SupportsCFG(".go") {
		t.Error("SupportsCFG('.go') should return false")
	}
	if ex.SupportsCFG(".js") {
		t.Error("SupportsCFG('.js') should return false")
	}
	if ex.SupportsCFG(".py") {
		t.Error("SupportsCFG('.py') should return false")
	}
}

func TestRubyControlFlowExtractor_ReturnTerminal(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	src := []byte(`def compute(x)
  if x > 0
    return x * 2
  end
  return 0
end
`)
	cfgs, err := ex.ExtractCFGs("f.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}

	foundReturn := false
	for _, n := range cfgs[0].Nodes {
		if n.Type == "terminal" && n.Kind == "return" && strings.HasPrefix(n.Label, "return") {
			foundReturn = true
		}
	}
	if !foundReturn {
		t.Error("expected a return terminal with kind=return and a 'return ...' label")
	}
}

func TestRubyControlFlowExtractor_StartNodeLabelIsMethodName(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	src := []byte(`def foo
  x = 1
end
`)
	cfgs, err := ex.ExtractCFGs("f.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	for _, n := range cfgs[0].Nodes {
		if n.Type == "start" {
			if n.Label != "foo" {
				t.Errorf("start node label = %q, want %q", n.Label, "foo")
			}
		}
	}
}

func TestRubyControlFlowExtractor_AbsolutePathStripped(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	src := []byte(`def bar
  x = 2
end
`)
	cfgs, err := ex.ExtractCFGs("/Users/test/work/project/src/handler.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	if cfgs[0].SourceFile != "handler.rb" {
		t.Errorf("SourceFile = %q, want handler.rb", cfgs[0].SourceFile)
	}
	if cfgs[0].Entry != "handler.rb::bar" {
		t.Errorf("Entry = %q, want handler.rb::bar", cfgs[0].Entry)
	}
}

func TestRubyControlFlowExtractor_ElsifChain(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	src := []byte(`def route(x)
  if x > 0
    handle_positive
  elsif x < 0
    handle_negative
  else
    handle_zero
  end
end
`)
	cfgs, err := ex.ExtractCFGs("f.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	counts := countNodeTypes(cfgs[0])
	if counts["decision"] < 2 {
		t.Errorf("expected at least 2 decision nodes for if/elsif chain, got %d", counts["decision"])
	}

	branches := countEdgeBranches(cfgs[0])
	if branches["yes"] < 2 {
		t.Error("expected at least 2 'yes' branch edges for chained decisions")
	}
	if branches["no"] < 1 {
		t.Error("expected at least 1 'no' branch edge for elsif/else")
	}
}

func TestRubyControlFlowExtractor_RaiseTerminal(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	src := []byte(`def validate(x)
  if x.nil?
    raise ArgumentError, "x is nil"
  end
  return x
end
`)
	cfgs, err := ex.ExtractCFGs("f.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}

	foundRaise := false
	for _, n := range cfgs[0].Nodes {
		if n.Type == "terminal" && n.Kind == "error" && strings.HasPrefix(n.Label, "raise") {
			foundRaise = true
		}
	}
	if !foundRaise {
		t.Error("expected a raise terminal with kind=error")
	}
}

func TestRubyControlFlowExtractor_WhileLoopBackEdge(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	src := []byte(`def loop_body
  while running?
    step_work
  end
end
`)
	cfgs, err := ex.ExtractCFGs("f.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}

	cfg := cfgs[0]
	hasLoopEdge := false
	for _, e := range cfg.Edges {
		if e.Branch == "loop" {
			hasLoopEdge = true
		}
	}
	if !hasLoopEdge {
		t.Error("expected at least one 'loop' branch edge for while loop back-edge")
	}
}

func TestRubyControlFlowExtractor_BodyOnlyMethod(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	src := []byte(`def simple
  x = 1
end
`)
	cfgs, err := ex.ExtractCFGs("f.rb", src)
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
		t.Errorf("expected 0 decision nodes for a simple method, got %d", counts["decision"])
	}
}

func TestRubyControlFlowExtractor_IfWithoutElse(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	src := []byte(`def test(x)
  if x > 0
    do_pos
  end
  do_after
end
`)
	cfgs, err := ex.ExtractCFGs("f.rb", src)
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
	if branches["no"] > 0 {
		t.Error("expected 0 'no' branch edges when there is no else block")
	}
}

func TestRubyControlFlowExtractor_BeginRescueEnsure(t *testing.T) {
	ex := newRubyCFGExtractor(t)
	src := []byte(`def find_user(id)
  begin
    user = User.find(id)
  rescue ActiveRecord::RecordNotFound
    render plain: "Not found", status: 404
  ensure
    connection.release
  end
end
`)
	cfgs, err := ex.ExtractCFGs("f.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 CFG, got %d", len(cfgs))
	}
	branches := countEdgeBranches(cfgs[0])
	if branches["error"] < 1 {
		t.Error("expected at least 1 'error' branch edge for rescue")
	}
}
