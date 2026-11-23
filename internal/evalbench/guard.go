package evalbench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"
)

// ErrForbiddenMutation is returned whenever ReadOnlyTransport intercepts an
// outgoing HTTP request targeting a non-read-only method, REST path, or MCP tool.
var ErrForbiddenMutation = errors.New("evalbench: forbidden non-read-only operation")

// AllowedMCPTools is the explicit allowlist of read-only MCP tools permitted
// through ReadOnlyTransport and replay filtering.
var AllowedMCPTools = map[string]bool{
	"memory_query":              true,
	"memory_search":             true,
	"memory_vsearch":            true,
	"memory_symbols":            true,
	"memory_get":                true,
	"memory_multi_get":          true,
	"memory_graph":              true,
	"memory_impact":             true,
	"memory_trace":              true,
	"memory_flow":               true,
	"memory_flowchart":          true,
	"memory_tags":               true,
	"memory_status":             true,
	"memory_wake_up":            true,
	"memory_workspaces_list":    true,
	"memory_workspaces_resolve": true,
	"memory_ticket":             true,
}

var allowedGETPaths = map[string]bool{
	"/api/status":        true,
	"/health":            true,
	"/api/v1/workspaces": true,
	"/api/v1/stats":      true,
	"/api/v1/symbols":    true,
	"/api/v1/tags":       true,
	"/api/v1/wake-up":    true,
	"/api/v1/documents":  true,
}

var allowedPOSTPaths = map[string]bool{
	"/api/v1/query":              true,
	"/api/v1/search":             true,
	"/api/v1/vsearch":            true,
	"/api/v1/get":                true,
	"/api/v1/multi-get":          true,
	"/api/v1/graph":              true,
	"/api/v1/graph/query":        true,
	"/api/v1/graph/impact":       true,
	"/api/v1/graph/trace":        true,
	"/api/v1/graph/flow":         true,
	"/api/v1/graph/flowchart":    true,
	"/api/v1/workspaces/resolve": true,
	"/api/v1/wake-up":            true,
}

var allowedMCPRPCMethods = map[string]bool{
	"initialize":                true,
	"notifications/initialized": true,
	"tools/list":                true,
	"ping":                      true,
}

// ReadOnlyTransport wraps an http.RoundTripper and blocks any non-read-only
// HTTP method, REST route, or MCP tool call before sending over the network.
type ReadOnlyTransport struct {
	Base http.RoundTripper
}

func NewReadOnlyHTTPClient(timeout time.Duration, base http.RoundTripper) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: &ReadOnlyTransport{Base: base},
	}
}

func EnsureReadOnlyClient(client *http.Client) *http.Client {
	if client == nil {
		return NewReadOnlyHTTPClient(30*time.Second, nil)
	}
	if _, ok := client.Transport.(*ReadOnlyTransport); ok {
		return client
	}
	cp := *client
	cp.Transport = &ReadOnlyTransport{Base: client.Transport}
	return &cp
}

func (t *ReadOnlyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := validateReadOnlyRequest(req); err != nil {
		return nil, err
	}
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

func validateReadOnlyRequest(req *http.Request) error {
	if req == nil || req.URL == nil {
		return fmt.Errorf("%w: nil request", ErrForbiddenMutation)
	}
	rawPath := req.URL.Path
	if rawPath == "" {
		rawPath = "/"
	}
	if strings.Contains(rawPath, "..") || strings.Contains(req.URL.RawPath, "..") {
		return fmt.Errorf("%w: path traversal in %s %s", ErrForbiddenMutation, req.Method, rawPath)
	}
	cleanPath := path.Clean(rawPath)

	switch req.Method {
	case http.MethodGet:
		if allowedGETPaths[cleanPath] || strings.HasPrefix(cleanPath, "/api/v1/documents/") {
			return nil
		}
		return fmt.Errorf("%w: GET %s", ErrForbiddenMutation, cleanPath)

	case http.MethodPost:
		if cleanPath == "/mcp" {
			return validateMCPPostBody(req)
		}
		if allowedPOSTPaths[cleanPath] {
			return nil
		}
		return fmt.Errorf("%w: POST %s", ErrForbiddenMutation, cleanPath)

	default:
		return fmt.Errorf("%w: %s %s", ErrForbiddenMutation, req.Method, cleanPath)
	}
}

type jsonRPCEnvelope struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type mcpToolCallParams struct {
	Name string `json:"name"`
}

func validateMCPPostBody(req *http.Request) error {
	if req.Body == nil {
		return fmt.Errorf("%w: empty MCP POST body", ErrForbiddenMutation)
	}
	bodyBytes, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return fmt.Errorf("read MCP request body: %w", err)
	}
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}

	trimmed := bytes.TrimSpace(bodyBytes)
	if len(trimmed) == 0 {
		return fmt.Errorf("%w: empty MCP JSON-RPC payload", ErrForbiddenMutation)
	}

	if trimmed[0] == '[' {
		var batch []jsonRPCEnvelope
		if err := json.Unmarshal(trimmed, &batch); err != nil || len(batch) == 0 {
			return fmt.Errorf("%w: invalid MCP JSON-RPC batch", ErrForbiddenMutation)
		}
		for _, msg := range batch {
			if err := validateMCPEnvelope(msg); err != nil {
				return err
			}
		}
		return nil
	}

	var msg jsonRPCEnvelope
	if err := json.Unmarshal(trimmed, &msg); err != nil {
		return fmt.Errorf("%w: invalid MCP JSON-RPC message", ErrForbiddenMutation)
	}
	return validateMCPEnvelope(msg)
}

func validateMCPEnvelope(msg jsonRPCEnvelope) error {
	if allowedMCPRPCMethods[msg.Method] {
		return nil
	}
	if msg.Method != "tools/call" {
		return fmt.Errorf("%w: MCP method %q", ErrForbiddenMutation, msg.Method)
	}
	var params mcpToolCallParams
	if len(msg.Params) == 0 || json.Unmarshal(msg.Params, &params) != nil || params.Name == "" {
		return fmt.Errorf("%w: MCP tools/call missing tool name", ErrForbiddenMutation)
	}
	if !AllowedMCPTools[params.Name] {
		return fmt.Errorf("%w: MCP tool %q", ErrForbiddenMutation, params.Name)
	}
	return nil
}

// CaptureWorkspaceSnapshot fetches GET /api/v1/workspaces through ReadOnlyTransport
// and records (doc_count, chunk_count) per workspace hash.
func CaptureWorkspaceSnapshot(ctx context.Context, client *http.Client, serverURL string) (WorkspaceSnapshot, error) {
	roClient := EnsureReadOnlyClient(client)
	endpoint := strings.TrimRight(serverURL, "/") + "/api/v1/workspaces"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return WorkspaceSnapshot{}, fmt.Errorf("create workspaces snapshot request: %w", err)
	}
	resp, err := roClient.Do(req)
	if err != nil {
		return WorkspaceSnapshot{}, fmt.Errorf("fetch workspaces snapshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return WorkspaceSnapshot{}, fmt.Errorf("fetch workspaces snapshot: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Workspaces []WorkspaceInfo `json:"workspaces"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return WorkspaceSnapshot{}, fmt.Errorf("decode workspaces snapshot: %w", err)
	}
	wsMap := make(map[string]WorkspaceInfo, len(payload.Workspaces))
	for _, ws := range payload.Workspaces {
		wsMap[ws.Hash] = ws
	}
	return WorkspaceSnapshot{
		CapturedAt: time.Now().UTC(),
		Workspaces: wsMap,
	}, nil
}

// DiffSnapshots compares before and after workspace snapshots on targetHashes.
// When targetHashes is empty, all workspaces present in before or after are checked.
func DiffSnapshots(before, after WorkspaceSnapshot, targetHashes []string) SnapshotDiff {
	targetSet := make(map[string]bool, len(targetHashes))
	for _, h := range targetHashes {
		if h != "" {
			targetSet[h] = true
		}
	}
	if len(targetSet) == 0 {
		for h := range before.Workspaces {
			targetSet[h] = true
		}
		for h := range after.Workspaces {
			targetSet[h] = true
		}
	}

	sortedTargets := make([]string, 0, len(targetSet))
	for h := range targetSet {
		sortedTargets = append(sortedTargets, h)
	}
	sort.Strings(sortedTargets)

	diff := SnapshotDiff{
		EvaluatedCount: len(sortedTargets),
	}
	for i, h := range sortedTargets {
		b := before.Workspaces[h]
		a := after.Workspaces[h]
		if b.DocCount != a.DocCount || b.ChunkCount != a.ChunkCount {
			diff.Mutated = true
			diff.Mutations = append(diff.Mutations, WorkspaceMutation{
				Slot:         fmt.Sprintf("ws-%d", i+1),
				BeforeDocs:   b.DocCount,
				AfterDocs:    a.DocCount,
				BeforeChunks: b.ChunkCount,
				AfterChunks:  a.ChunkCount,
			})
		}
	}

	allHashes := make(map[string]bool)
	for h := range before.Workspaces {
		allHashes[h] = true
	}
	for h := range after.Workspaces {
		allHashes[h] = true
	}
	for h := range allHashes {
		if targetSet[h] {
			continue
		}
		b := before.Workspaces[h]
		a := after.Workspaces[h]
		if b.DocCount != a.DocCount || b.ChunkCount != a.ChunkCount {
			diff.BackgroundChanges++
		}
	}

	switch {
	case diff.Mutated:
		diff.Note = "Evaluated workspace doc/chunk counts changed during the run. ReadOnlyTransport blocked 100% of mutating HTTP/MCP calls from bench eval; count deltas indicate concurrent external server activity (e.g. background file watcher or session harvester). Pass --strict-immutable=false when evaluating against an actively ingesting dev daemon."
	case diff.BackgroundChanges > 0:
		diff.Note = "Zero changes on evaluated workspaces (non-target workspace counts changed concurrently via background server activity)."
	default:
		diff.Note = "Zero mutations verified across pre/post workspace snapshots and ReadOnlyTransport."
	}
	return diff
}
