package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/rs/zerolog"
)

func newTestResolver(edges []graph.Edge) *graph.RubyCrossFileResolver {
	idx := graph.BuildClassIndex(edges)
	logger := zerolog.Nop()
	return graph.NewRubyCrossFileResolver(idx, logger)
}

func TestResolveEdges_simple(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "app/models/user.rb", TargetNode: "app/models/user.rb::User", Kind: graph.EdgeContains},
		{SourceNode: "app/models/user.rb", TargetNode: "app/models/user.rb::where", Kind: graph.EdgeContains},
	}

	resolver := newTestResolver(edges)

	controllerContent := []byte(`class UsersController < ApplicationController
  def index
    users = User.where(active: true)
    render json: users
  end
end`)

	fileContents := map[string][]byte{
		"app/controllers/users_controller.rb": controllerContent,
	}

	resolved := resolver.ResolveEdges(edges, fileContents)

	var callEdges []graph.Edge
	for _, e := range resolved {
		if e.Kind == graph.EdgeCalls {
			callEdges = append(callEdges, e)
		}
	}

	if len(callEdges) == 0 {
		t.Fatal("expected at least 1 resolved call edge")
	}

	found := false
	for _, e := range callEdges {
		if e.TargetNode == "app/models/user.rb::where" {
			found = true
			if e.Metadata != nil {
				if _, ok := e.Metadata["unresolved"]; ok {
					t.Error("should not be unresolved")
				}
			}
		}
	}
	if !found {
		t.Error("expected resolved call to app/models/user.rb::where")
		for _, e := range callEdges {
			t.Logf("  got: %s -> %s", e.SourceNode, e.TargetNode)
		}
	}
}

func TestResolveEdges_new_method(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "app/services/payment_service.rb", TargetNode: "app/services/payment_service.rb::PaymentService", Kind: graph.EdgeContains},
		{SourceNode: "app/services/payment_service.rb", TargetNode: "app/services/payment_service.rb::process", Kind: graph.EdgeContains},
	}

	resolver := newTestResolver(edges)

	content := []byte(`class OrderProcessor
  def call
    PaymentService.new.process(order)
  end
end`)

	fileContents := map[string][]byte{
		"app/services/order_processor.rb": content,
	}

	resolved := resolver.ResolveEdges(edges, fileContents)

	var callEdges []graph.Edge
	for _, e := range resolved {
		if e.Kind == graph.EdgeCalls {
			callEdges = append(callEdges, e)
		}
	}

	found := false
	for _, e := range callEdges {
		if e.TargetNode == "app/services/payment_service.rb::process" {
			found = true
		}
	}
	if !found {
		t.Error("expected resolved call to app/services/payment_service.rb::process")
		for _, e := range callEdges {
			t.Logf("  got: %s -> %s", e.SourceNode, e.TargetNode)
		}
	}
}

func TestResolveEdges_unresolved(t *testing.T) {
	edges := []graph.Edge{}

	idx := graph.BuildClassIndex(edges)
	logger := zerolog.Nop()
	resolver := graph.NewRubyCrossFileResolverNoFallback(idx, logger)

	content := []byte(`class MyService
  def run
    UnknownClass.do_something
  end
end`)

	fileContents := map[string][]byte{
		"app/services/my_service.rb": content,
	}

	resolved := resolver.ResolveEdges(edges, fileContents)

	var callEdges []graph.Edge
	for _, e := range resolved {
		if e.Kind == graph.EdgeCalls {
			callEdges = append(callEdges, e)
		}
	}

	found := false
	for _, e := range callEdges {
		if e.TargetNode == "do_something" {
			found = true
			if e.Metadata == nil {
				t.Error("expected metadata with unresolved flag")
			} else if _, ok := e.Metadata["unresolved"]; !ok {
				t.Error("expected unresolved=true in metadata")
			}
		}
	}
	if !found {
		t.Error("expected unresolved call to do_something")
		for _, e := range callEdges {
			t.Logf("  got: %s -> %s", e.SourceNode, e.TargetNode)
		}
	}
}

func TestResolveEdges_ambiguous(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "app/models/user.rb", TargetNode: "app/models/user.rb::User", Kind: graph.EdgeContains},
		{SourceNode: "test/mocks/user.rb", TargetNode: "test/mocks/user.rb::User", Kind: graph.EdgeContains},
	}

	resolver := newTestResolver(edges)

	content := []byte(`class MyService
  def run
    User.create(name: "test")
  end
end`)

	fileContents := map[string][]byte{
		"app/services/my_service.rb": content,
	}

	resolved := resolver.ResolveEdges(edges, fileContents)

	var callEdges []graph.Edge
	for _, e := range resolved {
		if e.Kind == graph.EdgeCalls {
			callEdges = append(callEdges, e)
		}
	}

	ambiguousCount := 0
	for _, e := range callEdges {
		if e.TargetNode == "app/models/user.rb::create" || e.TargetNode == "test/mocks/user.rb::create" {
			ambiguousCount++
			if e.Metadata == nil {
				t.Error("expected metadata with ambiguous flag")
			} else if _, ok := e.Metadata["ambiguous"]; !ok {
				t.Errorf("expected ambiguous=true in metadata for %s", e.TargetNode)
			}
		}
	}
	if ambiguousCount != 2 {
		t.Errorf("expected 2 ambiguous call edges, got %d", ambiguousCount)
	}
}

func TestBuildReconcileEdges(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "app/models/user.rb", TargetNode: "app/models/user.rb::User", Kind: graph.EdgeContains},
		{SourceNode: "app/controllers/users_controller.rb", TargetNode: "app/controllers/users_controller.rb::UsersController", Kind: graph.EdgeContains},
		{SourceNode: "config/routes.rb", TargetNode: "UsersController#create", Kind: graph.EdgeHTTP,
			Metadata: map[string]any{"method": "POST", "path": "/users"}},
	}

	resolver := newTestResolver(edges)
	reconcileEdges := resolver.BuildReconcileEdges(edges)

	if len(reconcileEdges) == 0 {
		t.Fatal("expected at least 1 reconcile edge")
	}

	found := false
	for _, e := range reconcileEdges {
		if e.Kind == graph.EdgeReconcile {
			if e.SourceNode == "UsersController#create" && e.TargetNode == "app/controllers/users_controller.rb::create" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected reconcile edge from UsersController#create to file::create")
		for _, e := range reconcileEdges {
			t.Logf("  kind=%s %s -> %s", e.Kind, e.SourceNode, e.TargetNode)
		}
	}
}

func TestBuildReconcileEdges_namespaced(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "app/controllers/api/v1/users_controller.rb", TargetNode: "app/controllers/api/v1/users_controller.rb::UsersController", Kind: graph.EdgeContains},
		{SourceNode: "config/routes.rb", TargetNode: "Api::V1::UsersController#index", Kind: graph.EdgeHTTP,
			Metadata: map[string]any{"method": "GET", "path": "/api/v1/users"}},
	}

	resolver := newTestResolver(edges)
	reconcileEdges := resolver.BuildReconcileEdges(edges)

	found := false
	for _, e := range reconcileEdges {
		if e.SourceNode == "Api::V1::UsersController#index" &&
			e.TargetNode == "app/controllers/api/v1/users_controller.rb::index" {
			found = true
		}
	}
	if !found {
		t.Error("expected reconcile edge for namespaced controller")
		for _, e := range reconcileEdges {
			t.Logf("  %s -> %s", e.SourceNode, e.TargetNode)
		}
	}
}
