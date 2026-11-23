package evalbench

import (
	"encoding/json"
	"time"
)

const (
	TransportMCP  = "mcp"
	TransportREST = "rest"
	ReportVersion = "1"
)

type WorkspaceInfo struct {
	Hash       string `json:"hash"`
	Name       string `json:"name"`
	RootPath   string `json:"root_path"`
	DocCount   int64  `json:"doc_count"`
	ChunkCount int64  `json:"chunk_count"`
}

type WorkspaceSnapshot struct {
	CapturedAt time.Time                `json:"captured_at"`
	Workspaces map[string]WorkspaceInfo `json:"workspaces"`
}

type WorkspaceMutation struct {
	Slot         string `json:"slot"`
	BeforeDocs   int64  `json:"before_docs"`
	AfterDocs    int64  `json:"after_docs"`
	BeforeChunks int64  `json:"before_chunks"`
	AfterChunks  int64  `json:"after_chunks"`
}

type SnapshotDiff struct {
	Mutated           bool                `json:"mutated"`
	EvaluatedCount    int                 `json:"evaluated_count"`
	Mutations         []WorkspaceMutation `json:"mutations,omitempty"`
	BackgroundChanges int                 `json:"background_changes,omitempty"`
	Note              string              `json:"note,omitempty"`
}

type OrderedItem struct {
	Rank       int    `json:"rank"`
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	Path       string `json:"path,omitempty"`
	Collection string `json:"collection,omitempty"`
	Snippet    string `json:"snippet,omitempty"`
}

type Surfaced struct {
	Names []string
	Paths []string
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

type CallResult struct {
	Tool            string         `json:"tool"`
	LatencyMs       float64        `json:"latency_ms"`
	ResponseBytes   int            `json:"response_bytes"`
	EstimatedTokens int            `json:"estimated_tokens"`
	IsError         bool           `json:"is_error"`
	IsEmpty         bool           `json:"is_empty"`
	ErrorMsg        string         `json:"error_msg,omitempty"`
	Surfaced        Surfaced       `json:"-"`
	OrderedItems    []OrderedItem  `json:"ordered_items,omitempty"`
	Collections     map[string]int `json:"collections,omitempty"`
}

type Task struct {
	ID            string         `json:"id"`
	Category      string         `json:"category"`
	Question      string         `json:"question"`
	Tools         []string       `json:"tools"`
	Input         map[string]any `json:"input"`
	ExpectSymbols []string       `json:"expect_symbols,omitempty"`
	ExpectFiles   []string       `json:"expect_files,omitempty"`
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

type SuiteProfile struct {
	Workspace string            `json:"workspace,omitempty"`
	Dataset   string            `json:"dataset,omitempty"`
	Vars      map[string]string `json:"vars,omitempty"`
}

type ProfilesConfig struct {
	Suites map[string]SuiteProfile `json:"suites"`
}

func (p *ProfilesConfig) UnmarshalJSON(data []byte) error {
	var wrapper struct {
		Suites map[string]SuiteProfile `json:"suites"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Suites) > 0 {
		p.Suites = wrapper.Suites
		return nil
	}
	var flat map[string]SuiteProfile
	if err := json.Unmarshal(data, &flat); err != nil {
		return err
	}
	delete(flat, "suites")
	p.Suites = flat
	return nil
}

type TaskEvalResult struct {
	ID                   string   `json:"id"`
	Category             string   `json:"category"`
	Recall               float64  `json:"recall"`
	FixedRecall          float64  `json:"fixed_recall"`
	AgentRecall          float64  `json:"agent_recall"`
	RecallAt5            float64  `json:"recall_at_5"`
	MRR                  float64  `json:"mrr"`
	Matched              []string `json:"matched,omitempty"`
	Missed               []string `json:"missed,omitempty"`
	FixedBytes           int      `json:"fixed_bytes"`
	FixedTokens          int      `json:"fixed_tokens"`
	AgentBytes           int      `json:"agent_bytes"`
	AgentTokens          int      `json:"agent_tokens"`
	NaiveFileBytes       int      `json:"naive_file_bytes,omitempty"`
	NaiveGrepBytes       int      `json:"naive_grep_bytes,omitempty"`
	NaiveTokens          int      `json:"naive_tokens,omitempty"`
	FilesReadCount       int      `json:"files_read_count,omitempty"`
	HasNaiveBaseline     bool     `json:"has_naive_baseline"`
	TokenSavingsFixedPct float64  `json:"token_savings_fixed_pct,omitempty"`
	TokenSavingsAgentPct float64  `json:"token_savings_agent_pct,omitempty"`
}

type ToolLatencySummary struct {
	Tool       string  `json:"tool"`
	Count      int     `json:"count"`
	ErrorCount int     `json:"error_count"`
	EmptyCount int     `json:"empty_count"`
	P50Ms      float64 `json:"p50_ms"`
	P95Ms      float64 `json:"p95_ms"`
	MeanMs     float64 `json:"mean_ms"`
}

type TokenEfficiencySummary struct {
	TasksEvaluated       int     `json:"tasks_evaluated"`
	TasksWithBaseline    int     `json:"tasks_with_baseline"`
	FixedBytes           int     `json:"fixed_bytes"`
	FixedTokens          int     `json:"fixed_tokens"`
	AgentBytes           int     `json:"agent_bytes"`
	AgentTokens          int     `json:"agent_tokens"`
	NaiveFileBytes       int     `json:"naive_file_bytes"`
	NaiveGrepBytes       int     `json:"naive_grep_bytes"`
	NaiveBytes           int     `json:"naive_bytes"`
	NaiveTokens          int     `json:"naive_tokens"`
	FilesReadCount       int     `json:"files_read_count"`
	TokenSavingsFixedPct float64 `json:"token_savings_fixed_pct"`
	TokenSavingsAgentPct float64 `json:"token_savings_agent_pct"`
}

type SuiteResult struct {
	Suite                  string                 `json:"suite"`
	Status                 string                 `json:"status"`
	SkipReason             string                 `json:"skip_reason,omitempty"`
	Transport              string                 `json:"transport"`
	WorkspaceSlot          string                 `json:"workspace_slot,omitempty"`
	TaskCount              int                    `json:"task_count"`
	OverallRecall          float64                `json:"overall_recall"`
	OverallFixedRecall     float64                `json:"overall_fixed_recall"`
	OverallRecallAt5       float64                `json:"overall_recall_at_5"`
	OverallMRR             float64                `json:"overall_mrr"`
	ByCategory             map[string]float64     `json:"by_category,omitempty"`
	ByCategoryRecallAt5    map[string]float64     `json:"by_category_recall_at_5,omitempty"`
	ByCategoryMRR          map[string]float64     `json:"by_category_mrr,omitempty"`
	ToolLatencies          []ToolLatencySummary   `json:"tool_latencies,omitempty"`
	CollectionDistribution map[string]int         `json:"collection_distribution,omitempty"`
	TokenEfficiency        TokenEfficiencySummary `json:"token_efficiency"`
	Tasks                  []TaskEvalResult       `json:"tasks,omitempty"`
	rawCalls               []CallResult           `json:"-"`
}

type ReplayToolSummary struct {
	Tool               string  `json:"tool"`
	Count              int     `json:"count"`
	RecordedEmptyCount int     `json:"recorded_empty_count"`
	LiveEmptyCount     int     `json:"live_empty_count"`
	RecordedEmptyRate  float64 `json:"recorded_empty_rate"`
	LiveEmptyRate      float64 `json:"live_empty_rate"`
	RecordedErrorCount int     `json:"recorded_error_count"`
	LiveErrorCount     int     `json:"live_error_count"`
	RecordedErrorRate  float64 `json:"recorded_error_rate"`
	LiveErrorRate      float64 `json:"live_error_rate"`
	RecordedP50Ms      float64 `json:"recorded_p50_ms"`
	RecordedP95Ms      float64 `json:"recorded_p95_ms"`
	LiveP50Ms          float64 `json:"live_p50_ms"`
	LiveP95Ms          float64 `json:"live_p95_ms"`
	Top5OverlapMean    float64 `json:"top5_overlap_mean"`
	Top5OverlapSamples int     `json:"top5_overlap_samples"`
}

type ReplayWorkspaceSummary struct {
	Slot          string  `json:"slot"`
	Count         int     `json:"count"`
	LiveEmptyRate float64 `json:"live_empty_rate"`
	LiveErrorRate float64 `json:"live_error_rate"`
	LiveP50Ms     float64 `json:"live_p50_ms"`
	LiveP95Ms     float64 `json:"live_p95_ms"`
}

type ReplayCallDetail struct {
	Tool              string         `json:"tool"`
	WorkspaceSlot     string         `json:"workspace_slot"`
	Args              map[string]any `json:"args,omitempty"`
	RecordedStatus    string         `json:"recorded_status"`
	LiveStatus        string         `json:"live_status"`
	RecordedLatencyMs float64        `json:"recorded_latency_ms"`
	LiveLatencyMs     float64        `json:"live_latency_ms"`
	Top5Overlap       *float64       `json:"top5_overlap,omitempty"`
}

type ReplaySummary struct {
	Status             string                   `json:"status"`
	SkipReason         string                   `json:"skip_reason,omitempty"`
	FilesRead          int                      `json:"files_read"`
	TotalEntries       int                      `json:"total_entries"`
	TotalReplayed      int                      `json:"total_replayed"`
	SkippedMutations   int                      `json:"skipped_mutations"`
	SkippedNonTool     int                      `json:"skipped_non_tool"`
	RecordedEmptyRate  float64                  `json:"recorded_empty_rate"`
	LiveEmptyRate      float64                  `json:"live_empty_rate"`
	RecordedErrorRate  float64                  `json:"recorded_error_rate"`
	LiveErrorRate      float64                  `json:"live_error_rate"`
	RecordedP50Ms      float64                  `json:"recorded_p50_ms"`
	RecordedP95Ms      float64                  `json:"recorded_p95_ms"`
	LiveP50Ms          float64                  `json:"live_p50_ms"`
	LiveP95Ms          float64                  `json:"live_p95_ms"`
	Top5OverlapMean    float64                  `json:"top5_overlap_mean"`
	Top5OverlapSamples int                      `json:"top5_overlap_samples"`
	ByTool             []ReplayToolSummary      `json:"by_tool,omitempty"`
	ByWorkspace        []ReplayWorkspaceSummary `json:"by_workspace,omitempty"`
	Details            []ReplayCallDetail       `json:"details,omitempty"`
}

type EvalReport struct {
	Version                string                 `json:"version"`
	GeneratedAt            time.Time              `json:"generated_at"`
	ServerURL              string                 `json:"server_url"`
	Transport              string                 `json:"transport"`
	StrictImmutable        bool                   `json:"strict_immutable"`
	Mutated                bool                   `json:"mutated"`
	SnapshotDiff           SnapshotDiff           `json:"snapshot_diff"`
	OverallRecall          float64                `json:"overall_recall"`
	OverallFixedRecall     float64                `json:"overall_fixed_recall"`
	OverallRecallAt5       float64                `json:"overall_recall_at_5"`
	OverallMRR             float64                `json:"overall_mrr"`
	Suites                 []SuiteResult          `json:"suites,omitempty"`
	ToolLatencies          []ToolLatencySummary   `json:"tool_latencies,omitempty"`
	CollectionDistribution map[string]int         `json:"collection_distribution,omitempty"`
	TokenEfficiency        TokenEfficiencySummary `json:"token_efficiency"`
	Replay                 *ReplaySummary         `json:"replay,omitempty"`
}

type CompareThresholds struct {
	MaxRecallDrop         float64 `json:"max_recall_drop"`
	MaxMRRDrop            float64 `json:"max_mrr_drop"`
	MaxEmptyRateIncrease  float64 `json:"max_empty_rate_increase"`
	MaxTokenIncreasePct   float64 `json:"max_token_increase_pct"`
	MaxP95LatencyIncrease float64 `json:"max_p95_latency_increase_ms"`
}

type RegressionFinding struct {
	Metric   string  `json:"metric"`
	Baseline float64 `json:"baseline"`
	Current  float64 `json:"current"`
	Delta    float64 `json:"delta"`
	Message  string  `json:"message"`
}

type EvalComparison struct {
	Passed      bool                `json:"passed"`
	Regressions []RegressionFinding `json:"regressions,omitempty"`
}
