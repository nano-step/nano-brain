package graph_test

import (
	"fmt"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func newRailsDSLEdgeExtractor(t *testing.T) *graph.RailsDSLEdgeExtractor {
	t.Helper()
	ex, err := graph.NewRailsDSLEdgeExtractor()
	if err != nil {
		t.Fatalf("NewRailsDSLEdgeExtractor: %v", err)
	}
	return ex
}

func TestRailsDSLEdgeExtractor_Supports(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
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

func TestRailsDSLEdgeExtractor_RequiresFrameworks(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	fws := ex.RequiresFrameworks()
	if len(fws) != 1 || fws[0] != "rails" {
		t.Errorf("expected [rails], got %v", fws)
	}
}

func TestRailsDSLEdgeExtractor_Language(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	if ex.Language() != "ruby" {
		t.Errorf("expected 'ruby', got %q", ex.Language())
	}
}

func TestRailsDSLEdgeExtractor_EmptyFile(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	edges, err := ex.ExtractEdges("empty.rb", []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges from empty file, got %d", len(edges))
	}
}

func TestRailsDSLEdgeExtractor_CommentsOnly(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	edges, err := ex.ExtractEdges("comments.rb", []byte("# comment\n# another\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
}

func TestRailsDSLEdgeExtractor_Associations(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	src := []byte(`class User < ApplicationRecord
  has_many :orders
  has_one :profile
  belongs_to :company
  has_and_belongs_to_many :roles
end
`)
	edges, err := ex.ExtractEdges("app/models/user.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(edges) != 4 {
		t.Fatalf("expected 4 association edges, got %d: %v", len(edges), edges)
	}

	for _, e := range edges {
		if e.Kind != graph.EdgeCalls {
			t.Errorf("expected EdgeCalls, got %v for edge %+v", e.Kind, e)
		}
		if e.Language != "ruby" {
			t.Errorf("expected Language='ruby', got %q", e.Language)
		}
		if e.SourceFile != "app/models/user.rb" {
			t.Errorf("expected SourceFile='app/models/user.rb', got %q", e.SourceFile)
		}
		if e.Line == 0 {
			t.Errorf("expected non-zero Line for edge %+v", e)
		}
	}

	edgesByKey := map[string]graph.Edge{}
	for _, e := range edges {
		edgesByKey[e.TargetNode] = e
	}

	expectedTargets := []string{"Order", "Profile", "Company", "Role"}
	for _, target := range expectedTargets {
		if _, ok := edgesByKey[target]; !ok {
			t.Errorf("expected edge with TargetNode=%q", target)
		}
	}

	for _, e := range edges {
		if e.SourceNode != "app/models/user.rb::User#"+e.Metadata["association_type"].(string) {
			t.Errorf("unexpected SourceNode: %q", e.SourceNode)
		}
	}

	assocTypes := map[string]bool{}
	for _, e := range edges {
		assocTypes[e.Metadata["association_type"].(string)] = true
	}
	for _, expected := range []string{"has_many", "has_one", "belongs_to", "has_and_belongs_to_many"} {
		if !assocTypes[expected] {
			t.Errorf("expected association_type %q", expected)
		}
	}
}

func TestRailsDSLEdgeExtractor_Callbacks(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	src := []byte(`class ApplicationController < ActionController::Base
  before_action :authenticate!
  after_action :track_event
end
`)
	edges, err := ex.ExtractEdges("app/controllers/application_controller.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(edges) != 2 {
		t.Fatalf("expected 2 callback edges, got %d: %v", len(edges), edges)
	}

	for _, e := range edges {
		if e.Kind != graph.EdgeMiddleware {
			t.Errorf("expected EdgeMiddleware, got %v for edge %+v", e.Kind, e)
		}
	}

	edgesByKey := map[string]graph.Edge{}
	for _, e := range edges {
		edgesByKey[e.TargetNode] = e
	}

	if _, ok := edgesByKey["ApplicationController#authenticate!"]; !ok {
		t.Error("expected edge with TargetNode='ApplicationController#authenticate!'")
	}
	if _, ok := edgesByKey["ApplicationController#track_event"]; !ok {
		t.Error("expected edge with TargetNode='ApplicationController#track_event'")
	}

	for _, e := range edges {
		if e.SourceNode != "app/controllers/application_controller.rb::ApplicationController#"+e.Metadata["callback_type"].(string) {
			t.Errorf("unexpected SourceNode: %q", e.SourceNode)
		}
	}
}

func TestRailsDSLEdgeExtractor_Concerns(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	src := []byte(`class User < ApplicationRecord
  include Authenticatable
  extend FriendlyId
end
`)
	edges, err := ex.ExtractEdges("app/models/user.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(edges) != 2 {
		t.Fatalf("expected 2 concern edges, got %d: %v", len(edges), edges)
	}

	for _, e := range edges {
		if e.Kind != graph.EdgeCalls {
			t.Errorf("expected EdgeCalls, got %v for edge %+v", e.Kind, e)
		}
	}

	edgesByKey := map[string]graph.Edge{}
	for _, e := range edges {
		edgesByKey[e.TargetNode] = e
	}

	if includeEdge, ok := edgesByKey["Authenticatable"]; !ok {
		t.Error("expected edge with TargetNode='Authenticatable'")
	} else {
		if includeEdge.Metadata["concern_type"] != "include" {
			t.Errorf("expected concern_type='include', got %v", includeEdge.Metadata["concern_type"])
		}
	}

	if extendEdge, ok := edgesByKey["FriendlyId"]; !ok {
		t.Error("expected edge with TargetNode='FriendlyId'")
	} else {
		if extendEdge.Metadata["concern_type"] != "extend" {
			t.Errorf("expected concern_type='extend', got %v", extendEdge.Metadata["concern_type"])
		}
	}
}

func TestRailsDSLEdgeExtractor_SidekiqWorker(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	src := []byte(`class EmailWorker
  include Sidekiq::Worker

  def perform(user_id)
    user = User.find(user_id)
    # ...
  end
end

class UserController < ApplicationController
  def create
    @user = User.create(user_params)
    EmailWorker.perform_async(@user.id)
  end
end
`)
	edges, err := ex.ExtractEdges("app/workers/email_worker.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	var integrationEdges []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			integrationEdges = append(integrationEdges, e)
		}
	}

	if len(integrationEdges) != 1 {
		t.Fatalf("expected 1 Sidekiq integration edge, got %d: %v", len(integrationEdges), integrationEdges)
	}

	e := integrationEdges[0]
	if e.TargetNode != "EmailWorker#perform" {
		t.Errorf("expected TargetNode='EmailWorker#perform', got %q", e.TargetNode)
	}
	if e.Metadata["kind"] != "sidekiq_job" {
		t.Errorf("expected kind='sidekiq_job', got %v", e.Metadata["kind"])
	}
	if e.Metadata["worker_class"] != "EmailWorker" {
		t.Errorf("expected worker_class='EmailWorker', got %v", e.Metadata["worker_class"])
	}
}

func TestRailsDSLEdgeExtractor_SidekiqJob(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	src := []byte(`class EmailWorker < Sidekiq::Job
  def perform(user_id)
  end
end

class Trigger
  def send_email
    EmailWorker.perform_async(42)
  end
end
`)
	edges, err := ex.ExtractEdges("app/workers/email_worker.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	var integrationEdges []graph.Edge
	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			integrationEdges = append(integrationEdges, e)
		}
	}

	if len(integrationEdges) != 1 {
		t.Fatalf("expected 1 Sidekiq integration edge, got %d: %v", len(integrationEdges), integrationEdges)
	}

	if integrationEdges[0].TargetNode != "EmailWorker#perform" {
		t.Errorf("expected TargetNode='EmailWorker#perform', got %q", integrationEdges[0].TargetNode)
	}
}

func TestRailsDSLEdgeExtractor_SidekiqUnknownWorker(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	src := []byte(`class Trigger
  def call
    UnknownWorker.perform_async(1)
  end
end
`)
	edges, err := ex.ExtractEdges("app/services/trigger.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			t.Errorf("should not emit integration edge for unknown worker, got %+v", e)
		}
	}
}

func TestRailsDSLEdgeExtractor_SidekiqNoReceiver(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	src := []byte(`class Trigger
  def call
    perform_async(1)
  end
end
`)
	edges, err := ex.ExtractEdges("app/services/trigger.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range edges {
		if e.Kind == graph.EdgeIntegration {
			t.Errorf("should not emit integration edge for bare perform_async, got %+v", e)
		}
	}
}

func TestRailsDSLEdgeExtractor_ModelWithAssociationsAndCallbacks(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	src := []byte(`class Order < ApplicationRecord
  belongs_to :user
  has_many :line_items

  before_save :calculate_total
  after_commit :notify_user, on: :create
end
`)
	edges, err := ex.ExtractEdges("app/models/order.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	var assocEdges, callbackEdges []graph.Edge
	for _, e := range edges {
		switch e.Kind {
		case graph.EdgeCalls:
			assocEdges = append(assocEdges, e)
		case graph.EdgeMiddleware:
			callbackEdges = append(callbackEdges, e)
		}
	}

	if len(assocEdges) != 2 {
		t.Fatalf("expected 2 association edges, got %d", len(assocEdges))
	}
	if len(callbackEdges) != 2 {
		t.Fatalf("expected 2 callback edges, got %d", len(callbackEdges))
	}
}

func TestRailsDSLEdgeExtractor_NestedClass(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	src := []byte(`module Admin
  class User < ApplicationRecord
    has_many :posts
  end
end
`)
	edges, err := ex.ExtractEdges("app/models/admin/user.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d: %v", len(edges), edges)
	}

	if edges[0].TargetNode != "Post" {
		t.Errorf("expected TargetNode='Post', got %q", edges[0].TargetNode)
	}
}

func TestRailsDSLEdgeExtractor_NonRubyFile(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
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

func TestRailsDSLEdgeExtractor_LineNumbers(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	src := []byte(`class User < ApplicationRecord
  has_many :orders
end
`)
	edges, err := ex.ExtractEdges("app/models/user.rb", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Line != 2 {
		t.Errorf("expected Line=2, got %d", edges[0].Line)
	}
}

func TestSingularize(t *testing.T) {
	ex := newRailsDSLEdgeExtractor(t)
	tests := []struct {
		name       string
		assocName  string
		wantTarget string
	}{
		{"basic_plural", "users", "User"},
		{"us_suffix", "status", "Status"},
		{"is_suffix", "basis", "Basis"},
		{"is_suffix_analysis", "analysis", "Analysis"},
		{"ies_rule", "categories", "Category"},
		{"ses_rule", "addresses", "Address"},
		{"ses_rule_statuses", "statuses", "Status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte(fmt.Sprintf(`class TestModel < ApplicationRecord
  has_many :%s
end
`, tt.assocName))
			edges, err := ex.ExtractEdges("app/models/test_model.rb", src)
			if err != nil {
				t.Fatal(err)
			}
			if len(edges) != 1 {
				t.Fatalf("expected 1 edge, got %d", len(edges))
			}
			if edges[0].TargetNode != tt.wantTarget {
				t.Errorf("TargetNode = %q, want %q", edges[0].TargetNode, tt.wantTarget)
			}
		})
	}
}
