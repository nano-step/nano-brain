package evalbench

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefaultSuiteDatasets maps built-in suite names to their repository-relative dataset files.
var DefaultSuiteDatasets = map[string]string{
	"go":         "benchmarks/capability/dataset.json",
	"typescript": "benchmarks/typescript/capability/dataset.json",
	"vue":        "benchmarks/vue/capability/dataset.template.json",
	"rails":      "benchmarks/rails/capability/dataset.json",
}

// SuiteRunConfig configures execution of a single Tier 1 capability suite.
type SuiteRunConfig struct {
	SuiteName     string
	DatasetPath   string
	Transport     string
	WorkspaceSpec string
	WorkspaceSlot string
	Vars          map[string]string
}

// NormalizeSuiteName normalizes CLI suite aliases (e.g. "ts" -> "typescript").
func NormalizeSuiteName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	if s == "ts" {
		return "typescript"
	}
	return s
}

// LoadDataset reads a capability dataset JSON file read-only and substitutes
// __KEY__ placeholders in memory without writing any temporary file to disk.
func LoadDataset(datasetPath string, vars map[string]string) (Dataset, error) {
	raw, err := os.ReadFile(datasetPath)
	if err != nil {
		return Dataset{}, fmt.Errorf("read dataset %s: %w", datasetPath, err)
	}
	var ds Dataset
	if err := json.Unmarshal(raw, &ds); err != nil {
		return Dataset{}, fmt.Errorf("parse dataset %s: %w", datasetPath, err)
	}
	if len(vars) > 0 {
		applyDatasetVars(&ds, vars)
	}
	return ds, nil
}

func applyDatasetVars(ds *Dataset, vars map[string]string) {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})

	sub := func(s string) string {
		for _, k := range keys {
			placeholder := k
			if !strings.HasPrefix(placeholder, "__") {
				placeholder = "__" + k + "__"
			}
			s = strings.ReplaceAll(s, placeholder, vars[k])
		}
		return s
	}

	ds.Workspace = sub(ds.Workspace)
	for i := range ds.Tasks {
		t := &ds.Tasks[i]
		t.Question = sub(t.Question)
		for j := range t.ExpectSymbols {
			t.ExpectSymbols[j] = sub(t.ExpectSymbols[j])
		}
		for j := range t.ExpectFiles {
			t.ExpectFiles[j] = sub(t.ExpectFiles[j])
		}
		if t.Input != nil {
			t.Input = substituteMapVars(t.Input, sub)
		}
	}
}

func substituteMapVars(in map[string]any, sub func(string) string) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = substituteAnyVars(v, sub)
	}
	return out
}

func substituteAnyVars(v any, sub func(string) string) any {
	switch val := v.(type) {
	case string:
		return sub(val)
	case map[string]any:
		return substituteMapVars(val, sub)
	case []any:
		cp := make([]any, len(val))
		for i, elem := range val {
			cp[i] = substituteAnyVars(elem, sub)
		}
		return cp
	default:
		return v
	}
}

// RunSuite executes a single capability suite read-only against client.
// Returns the SuiteResult, the resolved workspace hash (empty if skipped), and any fatal error.
func RunSuite(ctx context.Context, client *EvalClient, cfg SuiteRunConfig, snapshot WorkspaceSnapshot) (SuiteResult, string, error) {
	transport := cfg.Transport
	if transport == "" {
		transport = TransportMCP
	}
	res := SuiteResult{
		Suite:         cfg.SuiteName,
		Transport:     transport,
		WorkspaceSlot: cfg.WorkspaceSlot,
	}

	if _, statErr := os.Stat(cfg.DatasetPath); statErr != nil {
		if os.IsNotExist(statErr) {
			res.Status = "skipped"
			res.SkipReason = fmt.Sprintf("dataset not found: %s", cfg.DatasetPath)
			return res, "", nil
		}
		return res, "", fmt.Errorf("stat dataset %s: %w", cfg.DatasetPath, statErr)
	}

	ds, err := LoadDataset(cfg.DatasetPath, cfg.Vars)
	if err != nil {
		return res, "", err
	}
	if len(ds.Tasks) == 0 {
		res.Status = "skipped"
		res.SkipReason = "dataset contains 0 tasks"
		return res, "", nil
	}

	wsSpec := cfg.WorkspaceSpec
	if wsSpec == "" {
		wsSpec = ds.Workspace
	}
	wsInfo, ok, err := client.ResolveWorkspaceReadOnly(ctx, transport, wsSpec, snapshot)
	if err != nil {
		return res, "", fmt.Errorf("resolve workspace for suite %s: %w", cfg.SuiteName, err)
	}
	if !ok {
		res.Status = "skipped"
		res.SkipReason = "target workspace is not registered on server"
		return res, "", nil
	}

	// Check if unreplaced __PROJECT__ placeholders remain in tasks.
	if hasUnresolvedPlaceholder(ds) {
		res.Status = "skipped"
		res.SkipReason = "dataset has unresolved template placeholder (pass --var PROJECT=<dir> or --profiles)"
		return res, "", nil
	}

	var fileIdx *repoFileIndex
	if wsInfo.RootPath != "" {
		if info, statErr := os.Stat(wsInfo.RootPath); statErr == nil && info.IsDir() {
			fileIdx = buildRepoFileIndex(wsInfo.RootPath)
		}
	}

	var allCalls []CallResult
	taskResults := make([]TaskEvalResult, 0, len(ds.Tasks))
	for _, task := range ds.Tasks {
		tr, calls := runCapabilityTask(ctx, client, transport, wsInfo.Hash, ds.Agent, task, fileIdx)
		taskResults = append(taskResults, tr)
		allCalls = append(allCalls, calls...)
	}

	res = aggregateSuiteResult(cfg.SuiteName, transport, cfg.WorkspaceSlot, taskResults, allCalls)
	return res, wsInfo.Hash, nil
}

func hasUnresolvedPlaceholder(ds Dataset) bool {
	for _, t := range ds.Tasks {
		for _, f := range t.ExpectFiles {
			if strings.Contains(f, "__PROJECT__") {
				return true
			}
		}
		if node, ok := t.Input["node"].(string); ok && strings.Contains(node, "__PROJECT__") {
			return true
		}
	}
	return false
}

func runCapabilityTask(
	ctx context.Context,
	client *EvalClient,
	transport string,
	workspaceHash string,
	agent AgentPlan,
	task Task,
	fileIdx *repoFileIndex,
) (TaskEvalResult, []CallResult) {
	var fixedNames []string
	var fixedPaths []string
	var fixedBytes int
	var allCalls []CallResult

	for _, tool := range task.Tools {
		cr, _ := client.CallTaskTool(ctx, transport, workspaceHash, tool, task.Input)
		allCalls = append(allCalls, cr)
		fixedBytes += cr.ResponseBytes
		fixedNames = append(fixedNames, cr.Surfaced.Names...)
		fixedPaths = append(fixedPaths, cr.Surfaced.Paths...)
	}

	fixedMatched, fixedMissed, fixedRecall := scoreTaskRecall(task, fixedNames, fixedPaths)

	agentNames := append([]string(nil), fixedNames...)
	agentPaths := append([]string(nil), fixedPaths...)
	agentBytes := fixedBytes

	if agent.Enabled {
		agentCalls := runAgentDiscovery(ctx, client, transport, workspaceHash, agent, task)
		for _, cr := range agentCalls {
			allCalls = append(allCalls, cr)
			agentBytes += cr.ResponseBytes
			agentNames = append(agentNames, cr.Surfaced.Names...)
			agentPaths = append(agentPaths, cr.Surfaced.Paths...)
		}
	}

	matched := fixedMatched
	missed := fixedMissed
	recall := fixedRecall
	if agent.Enabled {
		matched, missed, recall = scoreTaskRecall(task, agentNames, agentPaths)
	}

	recallAt5, mrr := computeRankingMetrics(task, allCalls)

	out := TaskEvalResult{
		ID:          task.ID,
		Category:    task.Category,
		Recall:      round3(recall),
		FixedRecall: round3(fixedRecall),
		AgentRecall: round3(recall),
		RecallAt5:   round3(recallAt5),
		MRR:         round3(mrr),
		Matched:     matched,
		Missed:      missed,
		FixedBytes:  fixedBytes,
		FixedTokens: EstimateTokens(fixedBytes),
		AgentBytes:  agentBytes,
		AgentTokens: EstimateTokens(agentBytes),
	}

	if fileIdx != nil {
		fileBytes, grepBytes, filesRead := fileIdx.computeNaiveBaseline(task, agentPaths)
		if filesRead > 0 && (fileBytes+grepBytes) > 0 {
			naiveTokens := EstimateTokens(fileBytes + grepBytes)
			if naiveTokens > 0 {
				out.HasNaiveBaseline = true
				out.NaiveFileBytes = fileBytes
				out.NaiveGrepBytes = grepBytes
				out.NaiveTokens = naiveTokens
				out.FilesReadCount = filesRead
				out.TokenSavingsFixedPct = round2((1.0 - float64(out.FixedTokens)/float64(naiveTokens)) * 100.0)
				out.TokenSavingsAgentPct = round2((1.0 - float64(out.AgentTokens)/float64(naiveTokens)) * 100.0)
			}
		}
	}

	return out, allCalls
}

func runAgentDiscovery(
	ctx context.Context,
	client *EvalClient,
	transport string,
	workspaceHash string,
	plan AgentPlan,
	task Task,
) []CallResult {
	var calls []CallResult
	for _, tool := range plan.Tools {
		switch tool {
		case "query_question":
			if task.Question != "" {
				cr, _ := client.CallTaskTool(ctx, transport, workspaceHash, "query", map[string]any{"query": task.Question})
				calls = append(calls, cr)
			}
		case "query_input":
			if q, ok := task.Input["query"].(string); ok && q != "" && q != task.Question {
				cr, _ := client.CallTaskTool(ctx, transport, workspaceHash, "query", map[string]any{"query": q})
				calls = append(calls, cr)
			}
		case "symbols_identifiers":
			for _, q := range identifierQueries(task, plan.MaxSymbolQueries) {
				cr, _ := client.CallTaskTool(ctx, transport, workspaceHash, "symbols", map[string]any{"query": q})
				calls = append(calls, cr)
			}
		}
	}
	return calls
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

func scoreTaskRecall(task Task, names, paths []string) ([]string, []string, float64) {
	total := len(task.ExpectSymbols) + len(task.ExpectFiles)
	if total == 0 {
		return nil, nil, 1.0
	}
	var matched, missed []string
	for _, expected := range task.ExpectSymbols {
		if matchesAnySubstring(strings.ToLower(expected), names) {
			matched = append(matched, expected)
		} else {
			missed = append(missed, expected)
		}
	}
	for _, expected := range task.ExpectFiles {
		if matchesAnySubstring(strings.ToLower(expected), paths) {
			matched = append(matched, expected)
		} else {
			missed = append(missed, expected)
		}
	}
	return matched, missed, float64(len(matched)) / float64(total)
}

func matchesAnySubstring(needleLower string, candidates []string) bool {
	for _, c := range candidates {
		if strings.Contains(strings.ToLower(c), needleLower) {
			return true
		}
	}
	return false
}

func computeRankingMetrics(task Task, calls []CallResult) (recallAt5 float64, mrr float64) {
	totalExpected := len(task.ExpectSymbols) + len(task.ExpectFiles)
	if totalExpected == 0 {
		return 1.0, 1.0
	}

	var top5Names []string
	var top5Paths []string
	firstMatchRank := 0
	seqRank := 0

	for _, cr := range calls {
		for _, item := range cr.OrderedItems {
			seqRank++
			if item.Rank <= 5 {
				if item.Name != "" {
					top5Names = append(top5Names, item.Name)
				}
				if item.Snippet != "" {
					top5Names = append(top5Names, item.Snippet)
				}
				if item.Path != "" {
					top5Paths = append(top5Paths, item.Path)
				}
				if item.ID != "" {
					top5Paths = append(top5Paths, item.ID)
				}
			}
			if firstMatchRank == 0 && itemMatchesAnyExpectation(item, task) {
				firstMatchRank = seqRank
			}
		}
	}

	top5Hits := 0
	for _, sym := range task.ExpectSymbols {
		if matchesAnySubstring(strings.ToLower(sym), top5Names) {
			top5Hits++
		}
	}
	for _, f := range task.ExpectFiles {
		if matchesAnySubstring(strings.ToLower(f), top5Paths) {
			top5Hits++
		}
	}
	recallAt5 = float64(top5Hits) / float64(totalExpected)
	if firstMatchRank > 0 {
		mrr = 1.0 / float64(firstMatchRank)
	}
	return recallAt5, mrr
}

func itemMatchesAnyExpectation(item OrderedItem, task Task) bool {
	nameLow := strings.ToLower(item.Name)
	snipLow := strings.ToLower(item.Snippet)
	pathLow := strings.ToLower(item.Path)
	idLow := strings.ToLower(item.ID)

	for _, sym := range task.ExpectSymbols {
		needle := strings.ToLower(sym)
		if strings.Contains(nameLow, needle) || strings.Contains(snipLow, needle) {
			return true
		}
	}
	for _, f := range task.ExpectFiles {
		needle := strings.ToLower(f)
		if strings.Contains(pathLow, needle) || strings.Contains(idLow, needle) {
			return true
		}
	}
	return false
}

const maxNaiveBaselineFileSize = 2 << 20 // 2 MiB

var skippedBaselineDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".opencode":    true,
	"dist":         true,
	".nuxt":        true,
	".output":      true,
	"coverage":     true,
	".cache":       true,
	".omc":         true,
	".sisyphus":    true,
	".worktrees":   true,
	"bin":          true,
}

type indexedFileEntry struct {
	relPath  string
	relLower string
	dirRel   string
	absPath  string
}

type repoFileIndex struct {
	rootSlash string
	files     []indexedFileEntry
	byRelLow  map[string]indexedFileEntry
	byDir     map[string][]indexedFileEntry
	cache     map[string][]byte
}

func buildRepoFileIndex(rootPath string) *repoFileIndex {
	cleanRoot := filepath.Clean(rootPath)
	idx := &repoFileIndex{
		rootSlash: strings.TrimRight(filepath.ToSlash(cleanRoot), "/"),
		byRelLow:  make(map[string]indexedFileEntry),
		byDir:     make(map[string][]indexedFileEntry),
		cache:     make(map[string][]byte),
	}

	_ = filepath.WalkDir(cleanRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if p != cleanRoot && skippedBaselineDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil || info.Size() <= 0 || info.Size() > maxNaiveBaselineFileSize {
			return nil
		}
		rel, relErr := filepath.Rel(cleanRoot, p)
		if relErr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		relLow := strings.ToLower(relSlash)
		dirSlash := filepath.ToSlash(filepath.Dir(rel))
		if dirSlash == "." {
			dirSlash = ""
		}
		entry := indexedFileEntry{
			relPath:  relSlash,
			relLower: relLow,
			dirRel:   dirSlash,
			absPath:  p,
		}
		idx.files = append(idx.files, entry)
		idx.byRelLow[relLow] = entry
		idx.byDir[dirSlash] = append(idx.byDir[dirSlash], entry)
		return nil
	})
	return idx
}

func (idx *repoFileIndex) readCached(entry indexedFileEntry) ([]byte, bool) {
	if b, ok := idx.cache[entry.relLower]; ok {
		return b, true
	}
	b, err := os.ReadFile(entry.absPath)
	if err != nil {
		return nil, false
	}
	idx.cache[entry.relLower] = b
	return b, true
}

func (idx *repoFileIndex) computeNaiveBaseline(task Task, surfacedPaths []string) (fileBytes int, grepBytes int, filesRead int) {
	matched := idx.resolveGroundTruthFiles(task, surfacedPaths)
	if len(matched) == 0 {
		return 0, 0, 0
	}

	seenDirs := make(map[string]bool)
	grepPool := make([]indexedFileEntry, 0, len(matched)*2)
	seenPool := make(map[string]bool)

	for _, entry := range matched {
		data, ok := idx.readCached(entry)
		if !ok {
			continue
		}
		fileBytes += len(data)
		filesRead++
		if !seenPool[entry.relLower] {
			seenPool[entry.relLower] = true
			grepPool = append(grepPool, entry)
		}
		if !seenDirs[entry.dirRel] {
			seenDirs[entry.dirRel] = true
			siblings := idx.byDir[entry.dirRel]
			limit := 12
			if len(siblings) < limit {
				limit = len(siblings)
			}
			for _, sib := range siblings[:limit] {
				if !seenPool[sib.relLower] {
					seenPool[sib.relLower] = true
					grepPool = append(grepPool, sib)
				}
			}
		}
	}

	if filesRead == 0 {
		return 0, 0, 0
	}

	if len(task.ExpectSymbols) > 0 {
		const maxGrepLines = 200
		grepLines := 0
		for _, entry := range grepPool {
			if grepLines >= maxGrepLines {
				break
			}
			data, ok := idx.readCached(entry)
			if !ok {
				continue
			}
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			lineNo := 0
			for scanner.Scan() {
				lineNo++
				line := scanner.Text()
				lineLow := strings.ToLower(line)
				for _, sym := range task.ExpectSymbols {
					if sym != "" && strings.Contains(lineLow, strings.ToLower(sym)) {
						grepBytes += len(entry.relPath) + 8 + len(line)
						grepLines++
						break
					}
				}
				if grepLines >= maxGrepLines {
					break
				}
			}
		}
	}

	return fileBytes, grepBytes, filesRead
}

func (idx *repoFileIndex) resolveGroundTruthFiles(task Task, surfacedPaths []string) []indexedFileEntry {
	seen := make(map[string]bool)
	var out []indexedFileEntry
	add := func(e indexedFileEntry) {
		if !seen[e.relLower] {
			seen[e.relLower] = true
			out = append(out, e)
		}
	}

	for _, rawExp := range task.ExpectFiles {
		exp := strings.TrimPrefix(strings.TrimSpace(filepath.ToSlash(rawExp)), "./")
		exp = strings.TrimPrefix(exp, "/")
		if exp == "" {
			continue
		}
		expLow := strings.ToLower(exp)
		if exact, ok := idx.byRelLow[expLow]; ok {
			add(exact)
			continue
		}

		var candidates []indexedFileEntry
		suffix := "/" + expLow
		dirPrefix := expLow + "/"
		for _, f := range idx.files {
			if strings.HasSuffix(f.relLower, suffix) || strings.HasPrefix(f.relLower, dirPrefix) {
				candidates = append(candidates, f)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		selected := idx.rankCandidatesBySymbols(candidates, task.ExpectSymbols, 3)
		for _, s := range selected {
			add(s)
		}
	}

	if len(out) > 0 || len(task.ExpectSymbols) == 0 {
		return out
	}

	var fromSurfaced []indexedFileEntry
	for _, rawPath := range surfacedPaths {
		cleaned := cleanSurfacedFilePath(rawPath, idx.rootSlash)
		if cleaned == "" {
			continue
		}
		if e, ok := idx.byRelLow[strings.ToLower(cleaned)]; ok {
			fromSurfaced = append(fromSurfaced, e)
		}
	}
	for _, s := range idx.rankCandidatesBySymbols(fromSurfaced, task.ExpectSymbols, 3) {
		add(s)
	}
	return out
}

func (idx *repoFileIndex) rankCandidatesBySymbols(candidates []indexedFileEntry, symbols []string, maxKeep int) []indexedFileEntry {
	if len(candidates) <= maxKeep && len(symbols) == 0 {
		return candidates
	}
	type scored struct {
		entry indexedFileEntry
		hits  int
	}
	seen := make(map[string]bool, len(candidates))
	scoredList := make([]scored, 0, len(candidates))
	for _, c := range candidates {
		if seen[c.relLower] {
			continue
		}
		seen[c.relLower] = true
		hits := 0
		if len(symbols) > 0 {
			if data, ok := idx.readCached(c); ok {
				low := strings.ToLower(string(data))
				for _, sym := range symbols {
					if sym != "" && strings.Contains(low, strings.ToLower(sym)) {
						hits++
					}
				}
			}
		}
		scoredList = append(scoredList, scored{entry: c, hits: hits})
	}
	sort.Slice(scoredList, func(i, j int) bool {
		if scoredList[i].hits != scoredList[j].hits {
			return scoredList[i].hits > scoredList[j].hits
		}
		if len(scoredList[i].entry.relPath) != len(scoredList[j].entry.relPath) {
			return len(scoredList[i].entry.relPath) < len(scoredList[j].entry.relPath)
		}
		return scoredList[i].entry.relPath < scoredList[j].entry.relPath
	})
	if len(scoredList) > maxKeep {
		scoredList = scoredList[:maxKeep]
	}
	res := make([]indexedFileEntry, 0, len(scoredList))
	for _, s := range scoredList {
		res = append(res, s.entry)
	}
	return res
}

func cleanSurfacedFilePath(raw, rootSlash string) string {
	s := strings.TrimSpace(raw)
	if s == "" || strings.Contains(s, "://") {
		return ""
	}
	if qIdx := strings.IndexByte(s, '?'); qIdx >= 0 {
		s = s[:qIdx]
	}
	if file, _ := splitNodeID(s); file != "" {
		s = file
	}
	s = filepath.ToSlash(s)
	if rootSlash != "" && strings.HasPrefix(s, rootSlash+"/") {
		s = strings.TrimPrefix(s, rootSlash+"/")
	}
	s = strings.TrimPrefix(s, "./")
	s = strings.TrimPrefix(s, "/")
	return s
}

func aggregateSuiteResult(
	suiteName string,
	transport string,
	workspaceSlot string,
	tasks []TaskEvalResult,
	calls []CallResult,
) SuiteResult {
	byCat := make(map[string][]float64)
	byCatR5 := make(map[string][]float64)
	byCatMRR := make(map[string][]float64)
	var recalls, fixedRecalls, r5s, mrrs []float64

	var tok TokenEfficiencySummary
	tok.TasksEvaluated = len(tasks)
	var baselineFixedTokens, baselineAgentTokens int

	for _, t := range tasks {
		byCat[t.Category] = append(byCat[t.Category], t.Recall)
		byCatR5[t.Category] = append(byCatR5[t.Category], t.RecallAt5)
		byCatMRR[t.Category] = append(byCatMRR[t.Category], t.MRR)
		recalls = append(recalls, t.Recall)
		fixedRecalls = append(fixedRecalls, t.FixedRecall)
		r5s = append(r5s, t.RecallAt5)
		mrrs = append(mrrs, t.MRR)

		tok.FixedBytes += t.FixedBytes
		tok.FixedTokens += t.FixedTokens
		tok.AgentBytes += t.AgentBytes
		tok.AgentTokens += t.AgentTokens
		if t.HasNaiveBaseline && t.NaiveTokens > 0 {
			tok.TasksWithBaseline++
			tok.NaiveFileBytes += t.NaiveFileBytes
			tok.NaiveGrepBytes += t.NaiveGrepBytes
			tok.NaiveBytes += t.NaiveFileBytes + t.NaiveGrepBytes
			tok.NaiveTokens += t.NaiveTokens
			tok.FilesReadCount += t.FilesReadCount
			baselineFixedTokens += t.FixedTokens
			baselineAgentTokens += t.AgentTokens
		}
	}

	if tok.NaiveTokens > 0 {
		tok.TokenSavingsFixedPct = round2((1.0 - float64(baselineFixedTokens)/float64(tok.NaiveTokens)) * 100.0)
		tok.TokenSavingsAgentPct = round2((1.0 - float64(baselineAgentTokens)/float64(tok.NaiveTokens)) * 100.0)
	}

	colDist := make(map[string]int)
	for _, c := range calls {
		for k, n := range c.Collections {
			colDist[k] += n
		}
	}
	if len(colDist) == 0 {
		colDist = nil
	}

	return SuiteResult{
		Suite:                  suiteName,
		Status:                 "ok",
		Transport:              transport,
		WorkspaceSlot:          workspaceSlot,
		TaskCount:              len(tasks),
		OverallRecall:          round3(meanFloat64(recalls)),
		OverallFixedRecall:     round3(meanFloat64(fixedRecalls)),
		OverallRecallAt5:       round3(meanFloat64(r5s)),
		OverallMRR:             round3(meanFloat64(mrrs)),
		ByCategory:             roundMapFloat64(byCat),
		ByCategoryRecallAt5:    roundMapFloat64(byCatR5),
		ByCategoryMRR:          roundMapFloat64(byCatMRR),
		ToolLatencies:          SummarizeToolLatencies(calls),
		CollectionDistribution: colDist,
		TokenEfficiency:        tok,
		Tasks:                  tasks,
		rawCalls:               calls,
	}
}

// BuildEvalReport aggregates multi-suite capability results, tool latencies,
// collection distribution, token efficiency, replay summary, and snapshot diff.
func BuildEvalReport(
	serverURL string,
	transport string,
	strictImmutable bool,
	suites []SuiteResult,
	replay *ReplaySummary,
	diff SnapshotDiff,
) EvalReport {
	var recalls, fixedRecalls, r5s, mrrs []float64
	var tok TokenEfficiencySummary
	var baselineFixedTokens, baselineAgentTokens int
	colDist := make(map[string]int)

	var combinedCalls []CallResult
	allHaveRawCalls := true

	toolAgg := make(map[string]*ToolLatencySummary)
	p50WeightedSum := make(map[string]float64)
	p95WeightedSum := make(map[string]float64)
	meanWeightedSum := make(map[string]float64)

	for _, s := range suites {
		if s.Status != "ok" {
			continue
		}
		if len(s.rawCalls) > 0 {
			combinedCalls = append(combinedCalls, s.rawCalls...)
		} else if len(s.ToolLatencies) > 0 {
			allHaveRawCalls = false
		}
		for _, t := range s.Tasks {
			recalls = append(recalls, t.Recall)
			fixedRecalls = append(fixedRecalls, t.FixedRecall)
			r5s = append(r5s, t.RecallAt5)
			mrrs = append(mrrs, t.MRR)
			if t.HasNaiveBaseline && t.NaiveTokens > 0 {
				baselineFixedTokens += t.FixedTokens
				baselineAgentTokens += t.AgentTokens
			}
		}
		tok.TasksEvaluated += s.TokenEfficiency.TasksEvaluated
		tok.TasksWithBaseline += s.TokenEfficiency.TasksWithBaseline
		tok.FixedBytes += s.TokenEfficiency.FixedBytes
		tok.FixedTokens += s.TokenEfficiency.FixedTokens
		tok.AgentBytes += s.TokenEfficiency.AgentBytes
		tok.AgentTokens += s.TokenEfficiency.AgentTokens
		tok.NaiveFileBytes += s.TokenEfficiency.NaiveFileBytes
		tok.NaiveGrepBytes += s.TokenEfficiency.NaiveGrepBytes
		tok.NaiveBytes += s.TokenEfficiency.NaiveBytes
		tok.NaiveTokens += s.TokenEfficiency.NaiveTokens
		tok.FilesReadCount += s.TokenEfficiency.FilesReadCount

		for k, v := range s.CollectionDistribution {
			colDist[k] += v
		}
		for _, tl := range s.ToolLatencies {
			agg, ok := toolAgg[tl.Tool]
			if !ok {
				agg = &ToolLatencySummary{Tool: tl.Tool}
				toolAgg[tl.Tool] = agg
			}
			agg.Count += tl.Count
			agg.ErrorCount += tl.ErrorCount
			agg.EmptyCount += tl.EmptyCount
			p50WeightedSum[tl.Tool] += tl.P50Ms * float64(tl.Count)
			if tl.P95Ms > p95WeightedSum[tl.Tool] {
				p95WeightedSum[tl.Tool] = tl.P95Ms
			}
			meanWeightedSum[tl.Tool] += tl.MeanMs * float64(tl.Count)
		}
	}

	if tok.NaiveTokens > 0 {
		tok.TokenSavingsFixedPct = round2((1.0 - float64(baselineFixedTokens)/float64(tok.NaiveTokens)) * 100.0)
		tok.TokenSavingsAgentPct = round2((1.0 - float64(baselineAgentTokens)/float64(tok.NaiveTokens)) * 100.0)
	}
	if len(colDist) == 0 {
		colDist = nil
	}

	var toolLatencies []ToolLatencySummary
	if allHaveRawCalls && len(combinedCalls) > 0 {
		toolLatencies = SummarizeToolLatencies(combinedCalls)
	} else {
		toolLatencies = make([]ToolLatencySummary, 0, len(toolAgg))
		for name, agg := range toolAgg {
			if agg.Count > 0 {
				agg.P50Ms = round2(p50WeightedSum[name] / float64(agg.Count))
				agg.P95Ms = round2(p95WeightedSum[name])
				agg.MeanMs = round2(meanWeightedSum[name] / float64(agg.Count))
			}
			toolLatencies = append(toolLatencies, *agg)
		}
		sort.Slice(toolLatencies, func(i, j int) bool {
			return toolLatencies[i].Tool < toolLatencies[j].Tool
		})
	}

	return EvalReport{
		Version:                ReportVersion,
		GeneratedAt:            time.Now().UTC(),
		ServerURL:              serverURL,
		Transport:              transport,
		StrictImmutable:        strictImmutable,
		Mutated:                diff.Mutated,
		SnapshotDiff:           diff,
		OverallRecall:          round3(meanFloat64(recalls)),
		OverallFixedRecall:     round3(meanFloat64(fixedRecalls)),
		OverallRecallAt5:       round3(meanFloat64(r5s)),
		OverallMRR:             round3(meanFloat64(mrrs)),
		Suites:                 suites,
		ToolLatencies:          toolLatencies,
		CollectionDistribution: colDist,
		TokenEfficiency:        tok,
		Replay:                 replay,
	}
}

// SummarizeToolLatencies aggregates CallResult slices by tool name with p50/p95/mean latency.
func SummarizeToolLatencies(calls []CallResult) []ToolLatencySummary {
	type bucket struct {
		latencies []float64
		errors    int
		empties   int
	}
	byTool := make(map[string]*bucket)
	for _, c := range calls {
		b, ok := byTool[c.Tool]
		if !ok {
			b = &bucket{}
			byTool[c.Tool] = b
		}
		b.latencies = append(b.latencies, c.LatencyMs)
		if c.IsError {
			b.errors++
		}
		if c.IsEmpty {
			b.empties++
		}
	}
	out := make([]ToolLatencySummary, 0, len(byTool))
	for tool, b := range byTool {
		out = append(out, ToolLatencySummary{
			Tool:       tool,
			Count:      len(b.latencies),
			ErrorCount: b.errors,
			EmptyCount: b.empties,
			P50Ms:      round2(percentileFloat64(b.latencies, 0.50)),
			P95Ms:      round2(percentileFloat64(b.latencies, 0.95)),
			MeanMs:     round2(meanFloat64(b.latencies)),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Tool < out[j].Tool
	})
	return out
}

func percentileFloat64(vals []float64, q float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	idx := int(math.Ceil(q*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func meanFloat64(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func roundMapFloat64(in map[string][]float64) map[string]float64 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]float64, len(in))
	for k, vals := range in {
		out[k] = round3(meanFloat64(vals))
	}
	return out
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
