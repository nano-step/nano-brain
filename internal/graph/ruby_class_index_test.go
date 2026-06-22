package graph_test

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func TestBuildClassIndex(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "app/models/user.rb", TargetNode: "app/models/user.rb::User", Kind: graph.EdgeContains},
		{SourceNode: "app/models/user.rb", TargetNode: "app/models/user.rb::full_name", Kind: graph.EdgeContains},
		{SourceNode: "app/controllers/users_controller.rb", TargetNode: "app/controllers/users_controller.rb::UsersController", Kind: graph.EdgeContains},
		{SourceNode: "app/controllers/users_controller.rb", TargetNode: "app/controllers/users_controller.rb::index", Kind: graph.EdgeContains},
		{SourceNode: "app/services/payment_service.rb", TargetNode: "app/services/payment_service.rb::PaymentService", Kind: graph.EdgeContains},
		{SourceNode: "app/services/payment_service.rb", TargetNode: "app/services/payment_service.rb::process", Kind: graph.EdgeContains},
	}

	idx := graph.BuildClassIndex(edges)

	entries := idx.Lookup("User")
	if len(entries) != 1 {
		t.Fatalf("expected 1 User entry, got %d", len(entries))
	}
	if entries[0].FilePath != "app/models/user.rb" {
		t.Errorf("expected User in app/models/user.rb, got %s", entries[0].FilePath)
	}

	entries = idx.Lookup("UsersController")
	if len(entries) != 1 {
		t.Fatalf("expected 1 UsersController entry, got %d", len(entries))
	}
	if entries[0].FilePath != "app/controllers/users_controller.rb" {
		t.Errorf("expected UsersController in app/controllers/users_controller.rb, got %s", entries[0].FilePath)
	}

	entries = idx.Lookup("PaymentService")
	if len(entries) != 1 {
		t.Fatalf("expected 1 PaymentService entry, got %d", len(entries))
	}
}

func TestLookup_exactMatch(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "app/models/user.rb", TargetNode: "app/models/user.rb::User", Kind: graph.EdgeContains},
	}
	idx := graph.BuildClassIndex(edges)

	entries := idx.Lookup("User")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].FilePath != "app/models/user.rb" {
		t.Errorf("expected app/models/user.rb, got %s", entries[0].FilePath)
	}
}

func TestLookup_shortName(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "app/controllers/api/v1/users_controller.rb", TargetNode: "app/controllers/api/v1/users_controller.rb::UsersController", Kind: graph.EdgeContains},
	}
	idx := graph.BuildClassIndex(edges)

	entries := idx.Lookup("Api::V1::UsersController")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for Api::V1::UsersController, got %d", len(entries))
	}
	if entries[0].FilePath != "app/controllers/api/v1/users_controller.rb" {
		t.Errorf("expected app/controllers/api/v1/users_controller.rb, got %s", entries[0].FilePath)
	}
}

func TestLookup_fallback(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "app/models/user.rb", TargetNode: "app/models/user.rb::User", Kind: graph.EdgeContains},
	}
	idx := graph.BuildClassIndex(edges)

	entries := idx.Lookup("PaymentProcessor")
	if len(entries) != 1 {
		t.Fatalf("expected 1 fallback entry, got %d", len(entries))
	}
	if entries[0].FilePath != "app/models/payment_processor.rb" {
		t.Errorf("expected app/models/payment_processor.rb, got %s", entries[0].FilePath)
	}
}

func TestLookup_ambiguous(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "app/models/user.rb", TargetNode: "app/models/user.rb::User", Kind: graph.EdgeContains},
		{SourceNode: "test/mocks/user.rb", TargetNode: "test/mocks/user.rb::User", Kind: graph.EdgeContains},
	}
	idx := graph.BuildClassIndex(edges)

	entries := idx.Lookup("User")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for ambiguous User, got %d", len(entries))
	}
	paths := map[string]bool{}
	for _, e := range entries {
		paths[e.FilePath] = true
	}
	if !paths["app/models/user.rb"] {
		t.Error("expected app/models/user.rb in ambiguous results")
	}
	if !paths["test/mocks/user.rb"] {
		t.Error("expected test/mocks/user.rb in ambiguous results")
	}
}

func TestLookup_notFound(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "app/models/user.rb", TargetNode: "app/models/user.rb::User", Kind: graph.EdgeContains},
	}
	idx := graph.BuildClassIndex(edges)

	entries := idx.Lookup("full_name")
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for method name full_name, got %d", len(entries))
	}
}

func TestBuildClassIndex_skipsMethods(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "app/models/user.rb", TargetNode: "app/models/user.rb::User", Kind: graph.EdgeContains},
		{SourceNode: "app/models/user.rb", TargetNode: "app/models/user.rb::full_name", Kind: graph.EdgeContains},
		{SourceNode: "app/models/user.rb", TargetNode: "app/models/user.rb::save", Kind: graph.EdgeContains},
	}
	idx := graph.BuildClassIndex(edges)

	entries := idx.Lookup("full_name")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for method name full_name, got %d", len(entries))
	}

	entries = idx.Lookup("save")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for method name save, got %d", len(entries))
	}

	entries = idx.Lookup("User")
	if len(entries) != 1 {
		t.Errorf("expected 1 entry for User, got %d", len(entries))
	}
}


func TestLookup_fullName(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "app/controllers/api/v1/users_controller.rb", TargetNode: "app/controllers/api/v1/users_controller.rb::UsersController", Kind: graph.EdgeContains},
	}
	idx := graph.BuildClassIndex(edges)

	// Full namespace should match exactly
	entries := idx.Lookup("Api::V1::UsersController")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for full name, got %d", len(entries))
	}
	if entries[0].FilePath != "app/controllers/api/v1/users_controller.rb" {
		t.Errorf("expected app/controllers/api/v1/users_controller.rb, got %s", entries[0].FilePath)
	}
}

func TestLookup_controllerPreference(t *testing.T) {
	// The main bug: TokensController model + controller collision
	edges := []graph.Edge{
		{SourceNode: "app/models/tokens_controller.rb", TargetNode: "app/models/tokens_controller.rb::TokensController", Kind: graph.EdgeContains},
		{SourceNode: "app/controllers/api/v1/tokens_controller.rb", TargetNode: "app/controllers/api/v1/tokens_controller.rb::TokensController", Kind: graph.EdgeContains},
	}
	idx := graph.BuildClassIndex(edges)

	entries := idx.Lookup("TokensController")
	if len(entries) < 2 {
		t.Fatalf("expected 2 entries for ambiguous TokensController, got %d", len(entries))
	}
	// Controller should be first
	if entries[0].FilePath != "app/controllers/api/v1/tokens_controller.rb" {
		t.Errorf("expected controller first, got %s", entries[0].FilePath)
	}
}

func TestLookup_conventionPaths(t *testing.T) {
	idx := graph.BuildClassIndex(nil)

	tests := []struct {
		className string
		wantPath  string
	}{
		{"PaymentService", "app/services/payment_service.rb"},
		{"ImportJob", "app/jobs/import_job.rb"},
		{"SyncWorker", "app/workers/sync_worker.rb"},
		{"WelcomeMailer", "app/mailers/welcome_mailer.rb"},
		{"UserPolicy", "app/policies/user_policy.rb"},
		{"UserSerializer", "app/serializers/user_serializer.rb"},
		{"Admin::PaymentService", "app/services/admin/payment_service.rb"},
		{"Admin::ImportJob", "app/jobs/admin/import_job.rb"},
		{"Admin::SyncWorker", "app/workers/admin/sync_worker.rb"},
		{"Admin::WelcomeMailer", "app/mailers/admin/welcome_mailer.rb"},
		{"Admin::UserPolicy", "app/policies/admin/user_policy.rb"},
		{"Admin::UserSerializer", "app/serializers/admin/user_serializer.rb"},
	}
	for _, tt := range tests {
		entries := idx.Lookup(tt.className)
		if len(entries) != 1 {
			t.Fatalf("Lookup(%q): expected 1 entry, got %d", tt.className, len(entries))
		}
		if entries[0].FilePath != tt.wantPath {
			t.Errorf("Lookup(%q): got %q, want %q", tt.className, entries[0].FilePath, tt.wantPath)
		}
	}
}

func TestLookup_controllerPreference_namespaced(t *testing.T) {
	// Full namespace lookup should prefer controller even when model exists
	edges := []graph.Edge{
		{SourceNode: "app/models/tokens_controller.rb", TargetNode: "app/models/tokens_controller.rb::TokensController", Kind: graph.EdgeContains},
		{SourceNode: "app/controllers/api/v1/tokens_controller.rb", TargetNode: "app/controllers/api/v1/tokens_controller.rb::TokensController", Kind: graph.EdgeContains},
	}
	idx := graph.BuildClassIndex(edges)

	// Full namespace should resolve to the controller file directly
	entries := idx.Lookup("Api::V1::TokensController")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for full namespace, got %d", len(entries))
	}
	if entries[0].FilePath != "app/controllers/api/v1/tokens_controller.rb" {
		t.Errorf("expected controller file, got %s", entries[0].FilePath)
	}
}

func TestLookup_controllerFallback(t *testing.T) {
	// When no entries exist, railsConventionPath should use app/controllers/ for Controller classes
	edges := []graph.Edge{
		{SourceNode: "app/models/user.rb", TargetNode: "app/models/user.rb::User", Kind: graph.EdgeContains},
	}
	idx := graph.BuildClassIndex(edges)

	entries := idx.Lookup("Admin::UsersController")
	if len(entries) != 1 {
		t.Fatalf("expected 1 fallback entry, got %d", len(entries))
	}
	if entries[0].FilePath != "app/controllers/admin/users_controller.rb" {
		t.Errorf("expected app/controllers/admin/users_controller.rb, got %s", entries[0].FilePath)
	}
}

func TestLookup_adminNamespace(t *testing.T) {
	edges := []graph.Edge{
		{SourceNode: "app/controllers/admin/users_controller.rb", TargetNode: "app/controllers/admin/users_controller.rb::UsersController", Kind: graph.EdgeContains},
	}
	idx := graph.BuildClassIndex(edges)

	entries := idx.Lookup("Admin::UsersController")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for Admin::UsersController, got %d", len(entries))
	}
	if entries[0].FilePath != "app/controllers/admin/users_controller.rb" {
		t.Errorf("expected app/controllers/admin/users_controller.rb, got %s", entries[0].FilePath)
	}
}

func TestCamelToSnake(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"User", "user"},
		{"UsersController", "users_controller"},
		{"PaymentService", "payment_service"},
		{"PaymentProcessor", "payment_processor"},
		{"OrderItem", "order_item"},
		{"HTMLParser", "html_parser"},
		{"APIKey", "api_key"},
	}
	for _, tt := range tests {
		got := graph.CamelToSnake(tt.input)
		if got != tt.want {
			t.Errorf("CamelToSnake(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
