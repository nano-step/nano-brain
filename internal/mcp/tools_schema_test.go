package mcp_test

import (
	"encoding/json"
	"testing"
)

// toolInputSchema is a minimal decode target for the JSON Schema object the
// SDK returns as Tool.InputSchema (typed `any` on the wire). We only need
// the "properties" and "required" fields for this assertion.
type toolInputSchema struct {
	Properties map[string]any `json:"properties"`
	Required   []string       `json:"required"`
}

func decodeInputSchema(t *testing.T, raw any) toolInputSchema {
	t.Helper()
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal InputSchema: %v", err)
	}
	var schema toolInputSchema
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatalf("unmarshal InputSchema: %v", err)
	}
	return schema
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// TestToolSchema_WorkspaceNotRequired asserts D-06: the 14 tools that
// previously required "workspace" now list it as present-but-optional in
// their schema (still in properties, dropped from required), while the 4
// excluded tools (which never required "workspace") keep their original
// required-fields contract unchanged (RESEARCH Pitfall 3).
func TestToolSchema_WorkspaceNotRequired(t *testing.T) {
	session, ctx := setupTestClient(t)

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	schemas := make(map[string]toolInputSchema, len(result.Tools))
	for _, tool := range result.Tools {
		schemas[tool.Name] = decodeInputSchema(t, tool.InputSchema)
	}

	// The 14 tools edited by this plan: workspace must be present in
	// properties but absent from required.
	editedTools := []string{
		"memory_query",
		"memory_search",
		"memory_vsearch",
		"memory_get",
		"memory_write",
		"memory_tags",
		"memory_update",
		"memory_wake_up",
		"memory_graph",
		"memory_trace",
		"memory_impact",
		"memory_symbols",
		"memory_flow",
		"memory_flowchart",
	}

	for _, name := range editedTools {
		schema, ok := schemas[name]
		if !ok {
			t.Fatalf("tool %s not registered", name)
		}
		if _, hasWorkspaceProp := schema.Properties["workspace"]; !hasWorkspaceProp {
			t.Errorf("%s: expected \"workspace\" to remain in properties, but it is absent", name)
		}
		if containsString(schema.Required, "workspace") {
			t.Errorf("%s: expected \"workspace\" to be dropped from required, got required=%v", name, schema.Required)
		}
	}

	// memory_workspaces_resolve never required "workspace" (it takes
	// "path"); assert its required contract is unchanged.
	if schema, ok := schemas["memory_workspaces_resolve"]; !ok {
		t.Fatalf("tool memory_workspaces_resolve not registered")
	} else if !containsString(schema.Required, "path") {
		t.Errorf("memory_workspaces_resolve: expected required to include \"path\", got %v", schema.Required)
	}

	// memory_ticket never required "workspace" (it takes "ticket"); assert
	// its required contract is unchanged.
	if schema, ok := schemas["memory_ticket"]; !ok {
		t.Fatalf("tool memory_ticket not registered")
	} else if !containsString(schema.Required, "ticket") {
		t.Errorf("memory_ticket: expected required to include \"ticket\", got %v", schema.Required)
	}

	// memory_workspaces_list and memory_status never had a "workspace"
	// param at all; assert no regression introduces one as required.
	for _, name := range []string{"memory_workspaces_list", "memory_status"} {
		schema, ok := schemas[name]
		if !ok {
			t.Fatalf("tool %s not registered", name)
		}
		if containsString(schema.Required, "workspace") {
			t.Errorf("%s: did not previously require \"workspace\"; regression detected, required=%v", name, schema.Required)
		}
	}
}
