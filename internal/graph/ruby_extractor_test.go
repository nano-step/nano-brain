package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func newRubyExtractor(t *testing.T) *graph.RubyGraphExtractor {
	t.Helper()
	ex, err := graph.NewRubyGraphExtractor()
	if err != nil {
		t.Fatalf("NewRubyGraphExtractor: %v", err)
	}
	return ex
}

func TestRubyGraphExtractor_Supports(t *testing.T) {
	ex := newRubyExtractor(t)
	tests := []struct {
		ext  string
		want bool
	}{
		{".rb", true},
		{".go", false},
		{".ts", false},
		{".py", false},
		{".js", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := ex.Supports(tt.ext); got != tt.want {
			t.Errorf("Supports(%q) = %v, want %v", tt.ext, got, tt.want)
		}
	}
}

func TestRubyGraphExtractor_RequiresFrameworks(t *testing.T) {
	ex := newRubyExtractor(t)
	fws := ex.RequiresFrameworks()
	if len(fws) != 1 || fws[0] != "rails" {
		t.Errorf("expected [rails], got %v", fws)
	}
}

func TestRubyGraphExtractor_EmptyFile(t *testing.T) {
	ex := newRubyExtractor(t)
	edges, err := ex.ExtractEdges("empty.rb", []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges from empty file, got %d", len(edges))
	}
}

func TestRubyGraphExtractor_CommentsOnly(t *testing.T) {
	ex := newRubyExtractor(t)
	edges, err := ex.ExtractEdges("comments.rb", []byte("# comment\n# another\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
}

func TestRubyGraphExtractor_MethodDefinitions(t *testing.T) {
	ex := newRubyExtractor(t)
	src := []byte(`class MyClass
  def alpha
    1
  end

  def beta
    2
  end
end
`)
	edges, err := ex.ExtractEdges("test.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	var contains []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeContains {
			contains = append(contains, e)
		}
	}

	if len(contains) != 3 {
		t.Fatalf("expected 3 contains edges (class + 2 methods), got %d: %v", len(contains), contains)
	}

	names := map[string]bool{}
	for _, e := range contains {
		names[e.TargetNode] = true
	}
	if !names["test.rb::MyClass#alpha"] {
		t.Error("expected contains edge for MyClass#alpha")
	}
	if !names["test.rb::MyClass#beta"] {
		t.Error("expected contains edge for MyClass#beta")
	}
	if !names["test.rb::MyClass"] {
		t.Error("expected contains edge for MyClass")
	}
}

func TestRubyGraphExtractor_SingletonMethodDefinitions(t *testing.T) {
	ex := newRubyExtractor(t)
	src := []byte(`class MyClass
  def self.create_default
    new(name: "default")
  end

  def instance_method
    "hello"
  end
end
`)
	edges, err := ex.ExtractEdges("test.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	var contains []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeContains {
			contains = append(contains, e)
		}
	}

	if len(contains) != 3 {
		t.Fatalf("expected 3 contains edges (class + 2 methods), got %d: %v", len(contains), contains)
	}

	names := map[string]bool{}
	for _, e := range contains {
		names[e.TargetNode] = true
	}
	if !names["test.rb::MyClass#create_default"] {
		t.Error("expected contains edge for MyClass#create_default")
	}
	if !names["test.rb::MyClass#instance_method"] {
		t.Error("expected contains edge for MyClass#instance_method")
	}
	if !names["test.rb::MyClass"] {
		t.Error("expected contains edge for MyClass")
	}
}

func TestRubyGraphExtractor_MethodCalls(t *testing.T) {
	ex := newRubyExtractor(t)
	src := []byte(`class MyService
  def run
    prepare
    execute
    cleanup
  end

  def prepare
    puts "preparing"
  end

  def execute
    puts "executing"
  end

  def cleanup
    puts "cleaning"
  end
end
`)
	edges, err := ex.ExtractEdges("test.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	var calls []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeCalls {
			calls = append(calls, e)
		}
	}

	callees := map[string]bool{}
	for _, e := range calls {
		callees[e.TargetNode] = true
	}

	for _, expected := range []string{"prepare", "execute", "cleanup"} {
		if !callees[expected] {
			t.Errorf("expected call edge to %s", expected)
		}
	}
}

func TestRubyGraphExtractor_BareMethodCalls(t *testing.T) {
	ex := newRubyExtractor(t)
	src := []byte(`class Controller
  def index
    render json: data
  end

  def data
    []
  end
end
`)
	edges, err := ex.ExtractEdges("test.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	var calls []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeCalls {
			calls = append(calls, e)
		}
	}

	found := false
	for _, e := range calls {
		if e.SourceNode == "test.rb::Controller#index" && e.TargetNode == "data" {
			found = true
		}
	}
	if !found {
		t.Error("expected call edge from index to data")
	}
}

func TestRubyGraphExtractor_ReceiverCalls(t *testing.T) {
	ex := newRubyExtractor(t)
	src := []byte(`class Processor
  def process
    items.each { |i| handle(i) }
  end

  def items
    [1, 2, 3]
  end

  def handle(item)
    puts item
  end
end
`)
	edges, err := ex.ExtractEdges("test.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	var calls []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeCalls {
			calls = append(calls, e)
		}
	}

	callees := map[string]bool{}
	for _, e := range calls {
		callees[e.TargetNode] = true
	}

	if !callees["items"] {
		t.Error("expected call edge to items")
	}
	if !callees["handle"] {
		t.Error("expected call edge to handle")
	}
}

func TestRubyGraphExtractor_ChainedCalls(t *testing.T) {
	ex := newRubyExtractor(t)
	src := []byte(`class UserService
  def find_user(email)
    User.where(active: true).find_by(email: email)
  end
end
`)
	edges, err := ex.ExtractEdges("test.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	var calls []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeCalls {
			calls = append(calls, e)
		}
	}

	callees := map[string]bool{}
	for _, e := range calls {
		callees[e.TargetNode] = true
	}

	if callees["User"] {
		t.Error("should not have call edge to User (constant receiver)")
	}
}

func TestRubyGraphExtractor_Fixture(t *testing.T) {
	ex := newRubyExtractor(t)
	fixturePath := filepath.Join("testdata", "ruby", "users_controller.rb")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}

	edges, err := ex.ExtractEdges(fixturePath, content)
	if err != nil {
		t.Fatal(err)
	}

	var contains []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeContains {
			contains = append(contains, e)
		}
	}

	if len(contains) < 4 {
		t.Errorf("expected >=4 contains edges, got %d", len(contains))
	}

	containsMap := map[string]bool{}
	for _, e := range contains {
		containsMap[e.TargetNode] = true
	}
	for _, name := range []string{"index", "create", "build_user", "user_params"} {
		key := fixturePath + "::UsersController#" + name
		if !containsMap[key] {
			t.Errorf("expected contains edge for %s", name)
		}
	}
}

func TestRubyGraphExtractor_FixtureCalls(t *testing.T) {
	ex := newRubyExtractor(t)
	fixturePath := filepath.Join("testdata", "ruby", "users_controller.rb")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}

	edges, err := ex.ExtractEdges(fixturePath, content)
	if err != nil {
		t.Fatal(err)
	}

	var calls []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeCalls {
			calls = append(calls, e)
		}
	}

	callMap := map[string]bool{}
	for _, e := range calls {
		callMap[e.SourceNode+"->"+e.TargetNode] = true
	}

	expectedCalls := []string{
		fixturePath + "::UsersController#create->build_user",
		fixturePath + "::UsersController#build_user->user_params",
	}
	for _, key := range expectedCalls {
		if !callMap[key] {
			t.Errorf("expected call edge %s", key)
		}
	}
}

func TestRubyGraphExtractor_ModelFixture(t *testing.T) {
	ex := newRubyExtractor(t)
	fixturePath := filepath.Join("testdata", "ruby", "user.rb")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}

	edges, err := ex.ExtractEdges(fixturePath, content)
	if err != nil {
		t.Fatal(err)
	}

	var contains []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeContains {
			contains = append(contains, e)
		}
	}

	if len(contains) < 4 {
		t.Errorf("expected >=4 contains edges (full_name, find_by_email, deactivate, send_deactivation_email), got %d", len(contains))
	}

	containsMap := map[string]bool{}
	for _, e := range contains {
		containsMap[e.TargetNode] = true
	}
	for _, name := range []string{"full_name", "find_by_email", "deactivate", "send_deactivation_email"} {
		key := fixturePath + "::User#" + name
		if !containsMap[key] {
			t.Errorf("expected contains edge for %s", name)
		}
	}
}

func TestRubyGraphExtractor_ServiceFixture(t *testing.T) {
	ex := newRubyExtractor(t)
	fixturePath := filepath.Join("testdata", "ruby", "order_processor.rb")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}

	edges, err := ex.ExtractEdges(fixturePath, content)
	if err != nil {
		t.Fatal(err)
	}

	var calls []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeCalls {
			calls = append(calls, e)
		}
	}

	callMap := map[string]bool{}
	for _, e := range calls {
		callMap[e.SourceNode+"->"+e.TargetNode] = true
	}

	expectedCalls := []string{
		fixturePath + "::OrderProcessor#process->validate_order",
		fixturePath + "::OrderProcessor#process->calculate_total",
		fixturePath + "::OrderProcessor#process->apply_discounts",
		fixturePath + "::OrderProcessor#process->save_order",
	}
	for _, key := range expectedCalls {
		if !callMap[key] {
			t.Errorf("expected call edge %s", key)
		}
	}
}

func TestRubyGraphExtractor_LanguageMetadata(t *testing.T) {
	ex := newRubyExtractor(t)
	src := []byte(`class MyClass
  def alpha
    beta
  end

  def beta
    puts "hello"
  end
end
`)
	edges, err := ex.ExtractEdges("test.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(edges) == 0 {
		t.Fatal("expected edges")
	}

	for _, e := range edges {
		if e.Language != "ruby" {
			t.Errorf("expected Language='ruby', got %q for edge %+v", e.Language, e)
		}
	}
}

func TestRubyGraphExtractor_SourceFile(t *testing.T) {
	ex := newRubyExtractor(t)
	src := []byte(`class MyClass
  def alpha
    beta
  end

  def beta
    puts "hello"
  end
end
`)
	edges, err := ex.ExtractEdges("app/models/my_class.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range edges {
		if e.SourceFile != "app/models/my_class.rb" {
			t.Errorf("expected SourceFile='app/models/my_class.rb', got %q", e.SourceFile)
		}
	}
}

func TestRubyGraphExtractor_LineNumbers(t *testing.T) {
	ex := newRubyExtractor(t)
	src := []byte(`class MyClass
  def alpha
    beta
  end

  def beta
    puts "hello"
  end
end
`)
	edges, err := ex.ExtractEdges("test.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range edges {
		if e.Line == 0 {
			t.Errorf("edge has zero Line: %+v", e)
		}
	}
}

func TestRubyGraphExtractor_NonRubyFile(t *testing.T) {
	ex := newRubyExtractor(t)
	if ex.Supports(".go") {
		t.Error("should not support .go")
	}
	if ex.Supports(".ts") {
		t.Error("should not support .ts")
	}
	if ex.Supports(".py") {
		t.Error("should not support .py")
	}
}

func TestRubyGraphExtractor_PrivateMethods(t *testing.T) {
	ex := newRubyExtractor(t)
	src := []byte(`class Service
  def public_method
    private_helper
  end

  private

  def private_helper
    "done"
  end
end
`)
	edges, err := ex.ExtractEdges("test.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	var contains []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeContains {
			contains = append(contains, e)
		}
	}

	names := map[string]bool{}
	for _, e := range contains {
		names[e.TargetNode] = true
	}
	if !names["test.rb::Service#public_method"] {
		t.Error("expected contains edge for Service#public_method")
	}
	if !names["test.rb::Service#private_helper"] {
		t.Error("expected contains edge for Service#private_helper")
	}
}

func TestRubyGraphExtractor_CrossFileCalls(t *testing.T) {
	ex := newRubyExtractor(t)
	src := []byte(`class MyService
  def run
    OtherService.new.process
  end
end
`)
	edges, err := ex.ExtractEdges("test.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	var callees map[string]bool
	for _, e := range edges {
		if e.Kind == graph.EdgeCalls {
			if callees == nil {
				callees = make(map[string]bool)
			}
			callees[e.TargetNode] = true
		}
	}

	if !callees["process"] {
		t.Error("expected cross-file call edge to 'process'")
	}
}

func TestRubyGraphExtractor_ContainsClassModule(t *testing.T) {
	ex := newRubyExtractor(t)

	tests := []struct {
		name     string
		fixture  string
		expected []string
	}{
		{
			name:    "scoped class",
			fixture: "controller.rb",
			expected: []string{
				"Api::V1::UsersController#index",
				"Api::V1::UsersController#create",
				"Api::V1::UsersController#user_params",
				"UsersController",
			},
		},
		{
			name:    "simple class",
			fixture: "user.rb",
			expected: []string{
				"User#full_name",
				"User#active?",
				"User",
			},
		},
		{
			name:    "class with method calls",
			fixture: "payment_service.rb",
			expected: []string{
				"PaymentService#process",
				"PaymentService",
			},
		},
		{
			name:    "simple class with method",
			fixture: "order.rb",
			expected: []string{
				"Order#calculate_total",
				"Order",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixturePath := filepath.Join("testdata", "ruby", "multi_file", tt.fixture)
			content, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatal(err)
			}

			edges, err := ex.ExtractEdges(fixturePath, content)
			if err != nil {
				t.Fatal(err)
			}

			var contains []graph.Edge
			for _, e := range edges {
				if e.Kind == graph.EdgeContains {
					contains = append(contains, e)
				}
			}

			containsMap := map[string]bool{}
			for _, e := range contains {
				containsMap[e.TargetNode] = true
			}

			for _, name := range tt.expected {
				key := fixturePath + "::" + name
				if !containsMap[key] {
					t.Errorf("expected contains edge for %s (key=%s)", name, key)
				}
			}
		})
	}
}
