package flow

import (
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
)

func TestDeriveServiceName_RelativePaths(t *testing.T) {
	edges := []graph.Edge{
		{SourceFile: "tradeit-backend/server/trade.js"},
		{SourceFile: "tradeit-backend/handlers/topup.go"},
	}
	got := deriveServiceName(edges)
	if got != "tradeit-backend" {
		t.Errorf("expected %q, got %q", "tradeit-backend", got)
	}
}

func TestDeriveServiceName_AbsolutePaths(t *testing.T) {
	edges := []graph.Edge{
		{SourceFile: "/Users/tamlh/projects/tradeit-backend/server/trade.js"},
		{SourceFile: "/Users/tamlh/projects/tradeit-backend/handlers/topup.go"},
	}
	got := deriveServiceName(edges)
	if got != "Users" {
		t.Errorf("expected %q, got %q", "Users", got)
	}
}

func TestDeriveServiceName_WindowsPaths(t *testing.T) {
	edges := []graph.Edge{
		{SourceFile: "C:/Users/tamlh/projects/tradeit-backend/server/trade.js"},
	}
	got := deriveServiceName(edges)
	if got != "Users" {
		t.Errorf("expected %q, got %q", "Users", got)
	}
}

func TestDeriveServiceName_EmptyPaths(t *testing.T) {
	edges := []graph.Edge{
		{SourceFile: ""},
		{SourceFile: ""},
	}
	got := deriveServiceName(edges)
	if got != "Backend" {
		t.Errorf("expected fallback %q, got %q", "Backend", got)
	}
}

func TestDeriveServiceName_LeadingSlashes(t *testing.T) {
	edges := []graph.Edge{
		{SourceFile: "///Users/foo/bar.go"},
	}
	got := deriveServiceName(edges)
	if got != "Users" {
		t.Errorf("expected %q, got %q", "Users", got)
	}
}

func TestDeriveServiceName_EmptyAfterStrip(t *testing.T) {
	edges := []graph.Edge{
		{SourceFile: "/"},
		{SourceFile: "///"},
	}
	got := deriveServiceName(edges)
	if got != "Backend" {
		t.Errorf("expected fallback %q, got %q", "Backend", got)
	}
}

func TestDeriveServiceName_MixedPaths(t *testing.T) {
	edges := []graph.Edge{
		{SourceFile: "tradeit-backend/server/trade.js"},
		{SourceFile: "tradeit-backend/handlers/topup.go"},
		{SourceFile: "/Users/tamlh/projects/other/handler.go"},
	}
	got := deriveServiceName(edges)
	if got != "tradeit-backend" {
		t.Errorf("expected %q, got %q", "tradeit-backend", got)
	}
}
