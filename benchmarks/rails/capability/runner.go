package capability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config holds runtime configuration derived from environment variables.
type Config struct {
	ServerURL string
	Workspace string
	Freeze    bool
}

// DefaultConfig returns configuration with defaults applied.
func DefaultConfig() Config {
	// Defaults target the isolated benchmark server (port 3199, nanobrain_test DB),
	// not the dev server on 3100, so benchmark runs never touch dev data.
	serverURL := os.Getenv("NANO_BRAIN_URL")
	if serverURL == "" {
		serverURL = "http://localhost:3199"
	}
	workspace := os.Getenv("NANO_BRAIN_WORKSPACE")
	if workspace == "" {
		workspace = "rails-app"
	}
	return Config{
		ServerURL: serverURL,
		Workspace: workspace,
		Freeze:    os.Getenv("CAPBENCH_FREEZE") == "1",
	}
}

// --- Dataset types ---

type Task struct {
	ID            string                 `json:"id"`
	Category      string                 `json:"category"`
	Question      string                 `json:"question"`
	Tools         []string               `json:"tools"`
	Input         map[string]interface{} `json:"input"`
	ExpectSymbols []string               `json:"expect_symbols,omitempty"`
	ExpectFiles   []string               `json:"expect_files,omitempty"`
}

type AgentPlan struct {
	Enabled          bool     `json:"enabled"`
	Tools            []string `json:"tools,omitempty"`
	MaxSymbolQueries int      `json:"max_symbol_queries,omitempty"`
}

type Dataset struct {
	Version   int       `json:"version"`
	Workspace string    `json:"workspace"`
	Agent     AgentPlan `json:"agent,omitempty"`
	Tasks     []Task    `json:"tasks"`
}

// --- Result types ---

type TaskResult struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Recall      float64  `json:"recall"`
	FixedRecall float64  `json:"fixed_recall,omitempty"`
	AgentRecall float64  `json:"agent_recall,omitempty"`
	Matched     []string `json:"matched"`
	Missed      []string `json:"missed"`
}

type BenchResults struct {
	Version    string             `json:"version"`
	Overall    float64            `json:"overall"`
	ByCategory map[string]float64 `json:"by_category"`
	Tasks      []TaskResult       `json:"tasks"`
}

// --- API response types ---

type flowChainEntry struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type flowResponse struct {
	Found     bool             `json:"found"`
	Chain     []flowChainEntry `json:"chain"`
	Externals []flowChainEntry `json:"externals"`
}

type impactNode struct {
	Node string `json:"node"`
}

type impactResponse struct {
	Node     string       `json:"node"`
	Impacted []impactNode `json:"impacted"`
}

type traceNode struct {
	Node string `json:"node"`
}

type traceResponse struct {
	Entry string      `json:"entry"`
	Chain []traceNode `json:"chain"`
}

type symbolEntry struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	SourcePath string `json:"source_path"`
}

type symbolsResponse struct {
	Count   int           `json:"count"`
	Symbols []symbolEntry `json:"symbols"`
}

type queryResult struct {
	Title      string `json:"title"`
	SourcePath string `json:"source_path"`
	Snippet    string `json:"snippet"`
}

type queryResponse struct {
	Results []queryResult `json:"results"`
}

// --- HTTP helpers ---

func postJSON(ctx context.Context, client *http.Client, endpoint string, body interface{}, out interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func getJSON(ctx context.Context, client *http.Client, endpoint string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// --- Per-tool callers ---

// Surfaced holds the name-like and path-like strings collected from all tool calls.
type Surfaced struct {
	Names []string // symbol names, function names, chain/external names
	Paths []string // source paths, file parts of node ids
}

func (s *Surfaced) addName(v string) {
	if v != "" {
		s.Names = append(s.Names, v)
	}
}

func (s *Surfaced) addPath(v string) {
	if v != "" {
		s.Paths = append(s.Paths, v)
	}
}

// splitNodeID splits "file::Func" into (file, func). Returns ("", v) if no "::".
func splitNodeID(v string) (string, string) {
	idx := strings.LastIndex(v, "::")
	if idx < 0 {
		return "", v
	}
	return v[:idx], v[idx+2:]
}

func callFlow(ctx context.Context, client *http.Client, cfg Config, input map[string]interface{}) Surfaced {
	entry, _ := input["entry"].(string)
	maxDepth := 8
	if d, ok := input["max_depth"]; ok {
		switch v := d.(type) {
		case float64:
			maxDepth = int(v)
		case int:
			maxDepth = v
		}
	}
	body := map[string]interface{}{
		"workspace": cfg.Workspace,
		"entry":     entry,
		"max_depth": maxDepth,
		"format":    "json",
	}
	var resp flowResponse
	var s Surfaced
	if err := postJSON(ctx, client, cfg.ServerURL+"/api/v1/graph/flow", body, &resp); err != nil {
		return s
	}
	for _, c := range resp.Chain {
		s.addName(c.Name)
	}
	for _, e := range resp.Externals {
		s.addName(e.Name)
	}
	return s
}

func callImpact(ctx context.Context, client *http.Client, cfg Config, input map[string]interface{}) Surfaced {
	node, _ := input["node"].(string)
	maxDepth := 4
	if d, ok := input["max_depth"]; ok {
		switch v := d.(type) {
		case float64:
			maxDepth = int(v)
		case int:
			maxDepth = v
		}
	}
	body := map[string]interface{}{
		"workspace": cfg.Workspace,
		"node":      node,
		"max_depth": maxDepth,
	}
	var resp impactResponse
	var s Surfaced
	if err := postJSON(ctx, client, cfg.ServerURL+"/api/v1/graph/impact", body, &resp); err != nil {
		return s
	}
	for _, n := range resp.Impacted {
		s.addPath(n.Node)
		file, bare := splitNodeID(n.Node)
		s.addName(bare)
		if file != "" {
			s.addPath(file)
		}
	}
	return s
}

func callTrace(ctx context.Context, client *http.Client, cfg Config, input map[string]interface{}) Surfaced {
	node, _ := input["node"].(string)
	maxDepth := 4
	if d, ok := input["max_depth"]; ok {
		switch v := d.(type) {
		case float64:
			maxDepth = int(v)
		case int:
			maxDepth = v
		}
	}
	body := map[string]interface{}{
		"workspace": cfg.Workspace,
		"node":      node,
		"max_depth": maxDepth,
	}
	var resp traceResponse
	var s Surfaced
	if err := postJSON(ctx, client, cfg.ServerURL+"/api/v1/graph/trace", body, &resp); err != nil {
		return s
	}
	for _, n := range resp.Chain {
		s.addPath(n.Node)
		file, bare := splitNodeID(n.Node)
		s.addName(bare)
		if file != "" {
			s.addPath(file)
		}
	}
	return s
}

func callSymbols(ctx context.Context, client *http.Client, cfg Config, input map[string]interface{}) Surfaced {
	query, _ := input["query"].(string)
	endpoint := fmt.Sprintf("%s/api/v1/symbols?workspace=%s&query=%s&limit=20",
		cfg.ServerURL,
		url.QueryEscape(cfg.Workspace),
		url.QueryEscape(query),
	)
	var resp symbolsResponse
	var s Surfaced
	if err := getJSON(ctx, client, endpoint, &resp); err != nil {
		return s
	}
	for _, sym := range resp.Symbols {
		s.addName(sym.Name)
		s.addPath(sym.SourcePath)
	}
	return s
}

func callQuery(ctx context.Context, client *http.Client, cfg Config, input map[string]interface{}) Surfaced {
	query, _ := input["query"].(string)
	body := map[string]interface{}{
		"workspace": cfg.Workspace,
		"query":     query,
		"limit":     20,
	}
	var resp queryResponse
	var s Surfaced
	if err := postJSON(ctx, client, cfg.ServerURL+"/api/v1/query", body, &resp); err != nil {
		return s
	}
	for _, r := range resp.Results {
		s.addPath(r.SourcePath)
		s.addName(r.Title)
		s.addName(r.Snippet)
	}
	return s
}

// --- Task runner ---

// RunTask calls every fixed tool listed in the task, then optionally augments
// that context with deterministic agent-style discovery. Agent discovery never
// looks at expected answers; it uses the task question and input the same way a
// real coding agent would start with broad retrieval before specializing.
func RunTask(ctx context.Context, client *http.Client, cfg Config, agent AgentPlan, task Task) TaskResult {
	var names []string
	var paths []string

	for _, tool := range task.Tools {
		var s Surfaced
		switch tool {
		case "flow":
			s = callFlow(ctx, client, cfg, task.Input)
		case "impact":
			s = callImpact(ctx, client, cfg, task.Input)
		case "trace":
			s = callTrace(ctx, client, cfg, task.Input)
		case "symbols":
			s = callSymbols(ctx, client, cfg, task.Input)
		case "query":
			s = callQuery(ctx, client, cfg, task.Input)
		}
		names = append(names, s.Names...)
		paths = append(paths, s.Paths...)
	}

	fixed := scoreTask(task, names, paths)
	if !agent.Enabled {
		return fixed
	}

	agentNames := append([]string{}, names...)
	agentPaths := append([]string{}, paths...)
	s := callAgentPlan(ctx, client, cfg, agent, task)
	agentNames = append(agentNames, s.Names...)
	agentPaths = append(agentPaths, s.Paths...)

	result := scoreTask(task, agentNames, agentPaths)
	result.FixedRecall = round3(fixed.Recall)
	result.AgentRecall = round3(result.Recall)
	return result
}

func callAgentPlan(ctx context.Context, client *http.Client, cfg Config, plan AgentPlan, task Task) Surfaced {
	var out Surfaced
	for _, tool := range plan.Tools {
		switch tool {
		case "query_question":
			if task.Question != "" {
				s := callQuery(ctx, client, cfg, map[string]interface{}{"query": task.Question})
				out.Names = append(out.Names, s.Names...)
				out.Paths = append(out.Paths, s.Paths...)
			}
		case "query_input":
			if q, ok := task.Input["query"].(string); ok && q != "" && q != task.Question {
				s := callQuery(ctx, client, cfg, map[string]interface{}{"query": q})
				out.Names = append(out.Names, s.Names...)
				out.Paths = append(out.Paths, s.Paths...)
			}
		case "symbols_identifiers":
			for _, q := range identifierQueries(task, plan.MaxSymbolQueries) {
				s := callSymbols(ctx, client, cfg, map[string]interface{}{"query": q})
				out.Names = append(out.Names, s.Names...)
				out.Paths = append(out.Paths, s.Paths...)
			}
		}
	}
	return out
}

func identifierQueries(task Task, max int) []string {
	if max <= 0 {
		max = 8
	}
	text := task.Question
	for _, v := range task.Input {
		if s, ok := v.(string); ok {
			text += " " + s
		}
	}
	seen := make(map[string]bool)
	queries := make([]string, 0, max)
	for _, raw := range strings.FieldsFunc(text, isIdentifierSeparator) {
		term := strings.Trim(raw, "_#:/.-")
		if !looksLikeCodeIdentifier(term) || seen[term] {
			continue
		}
		seen[term] = true
		queries = append(queries, term)
		if len(queries) >= max {
			break
		}
	}
	return queries
}

func isIdentifierSeparator(r rune) bool {
	return !(r == '_' || r == '#' || r == ':' || r == '/' || r == '-' || r == '.' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z')
}

func looksLikeCodeIdentifier(s string) bool {
	if len(s) < 3 {
		return false
	}
	if strings.ContainsAny(s, "_#:/") {
		return true
	}
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}

// scoreTask computes recall for a single task given the surfaced name and path sets.
func scoreTask(task Task, names, paths []string) TaskResult {
	total := len(task.ExpectSymbols) + len(task.ExpectFiles)
	result := TaskResult{
		ID:       task.ID,
		Category: task.Category,
	}
	if total == 0 {
		result.Recall = 1.0
		return result
	}

	for _, expected := range task.ExpectSymbols {
		lower := strings.ToLower(expected)
		if matchesAny(lower, names) {
			result.Matched = append(result.Matched, expected)
		} else {
			result.Missed = append(result.Missed, expected)
		}
	}
	for _, expected := range task.ExpectFiles {
		lower := strings.ToLower(expected)
		if matchesAny(lower, paths) {
			result.Matched = append(result.Matched, expected)
		} else {
			result.Missed = append(result.Missed, expected)
		}
	}

	result.Recall = float64(len(result.Matched)) / float64(total)
	return result
}

// matchesAny returns true if needle is a case-insensitive substring of any candidate.
func matchesAny(needle string, candidates []string) bool {
	for _, c := range candidates {
		if strings.Contains(strings.ToLower(c), needle) {
			return true
		}
	}
	return false
}

// --- Aggregation ---

// Aggregate computes overall and per-category recall means from task results.
func Aggregate(results []TaskResult) BenchResults {
	byCategory := make(map[string][]float64)
	var allRecall []float64

	for _, r := range results {
		byCategory[r.Category] = append(byCategory[r.Category], r.Recall)
		allRecall = append(allRecall, r.Recall)
	}

	catMeans := make(map[string]float64, len(byCategory))
	for cat, vals := range byCategory {
		catMeans[cat] = mean(vals)
	}

	return BenchResults{
		Version:    "1",
		Overall:    round3(mean(allRecall)),
		ByCategory: roundMap(catMeans),
		Tasks:      results,
	}
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

func roundMap(m map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = round3(v)
	}
	return out
}

// CheckReachable returns nil if the server responds to a GET /health request.
func CheckReachable(serverURL string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(serverURL + "/health")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
