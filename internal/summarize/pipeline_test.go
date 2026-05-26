package summarize

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

type fakeLLM struct {
	mu          sync.Mutex
	inFlight    atomic.Int32
	maxInFlight atomic.Int32
	calls       []fakeCall
	handler     func(idx int, system, user string) (string, TokenUsage, error)
}

type fakeCall struct{ system, user string }

func (f *fakeLLM) ChatCompletion(ctx context.Context, system, user string) (string, TokenUsage, error) {
	cur := f.inFlight.Add(1)
	defer f.inFlight.Add(-1)
	for {
		prev := f.maxInFlight.Load()
		if cur <= prev || f.maxInFlight.CompareAndSwap(prev, cur) {
			break
		}
	}
	f.mu.Lock()
	idx := len(f.calls)
	f.calls = append(f.calls, fakeCall{system, user})
	f.mu.Unlock()
	if f.handler != nil {
		return f.handler(idx, system, user)
	}
	return defaultMapResponse, TokenUsage{}, nil
}

func (f *fakeLLM) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

const defaultMapResponse = "ACTIVITIES: did stuff\nDECISIONS: (none)\nFILES: foo.go\nPROBLEMS: (none)\nLEARNINGS: (none)"

type fakeLookup struct {
	mu     sync.Mutex
	called int
	err    error
}

func (f *fakeLookup) Lookup(_ context.Context, _ *SessionMetadata) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.called++
	return f.err
}

func newTestPipeline(llm LLMClient, lookup RelationshipLookup, concurrency int) *Pipeline {
	return NewPipeline(llm, lookup, concurrency, zerolog.Nop())
}

func baseMeta() SessionMetadata {
	return SessionMetadata{
		Source:    SourceOpenCode,
		SessionID: "ses_abc123",
		Title:     "Fix the auth bug",
		Agent:     "build",
		CreatedAt: time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC),
	}
}

func shortContent() string {
	return "## user\n\nhow do I fix this?\n\n## assistant\n\nCheck line 42.\n"
}

func longContent(chars int) string {
	line := "This is a line of session content for testing purposes.\n"
	var b strings.Builder
	b.WriteString("## user\n\nfix the bug\n\n## assistant\n\n")
	for b.Len() < chars {
		b.WriteString(line)
	}
	return b.String()
}

func TestPipeline_SingleShotShortcut(t *testing.T) {
	llm := &fakeLLM{handler: func(idx int, system, user string) (string, TokenUsage, error) {
		return "## Goal\nFix auth bug.", TokenUsage{PromptTokens: 10, CompletionTokens: 5}, nil
	}}
	p := newTestPipeline(llm, nil, 1)
	meta := baseMeta()

	result, err := p.Summarize(context.Background(), shortContent(), meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if llm.callCount() != 1 {
		t.Errorf("call count = %d, want 1", llm.callCount())
	}
	if !strings.Contains(result, "## Goal") {
		t.Errorf("result missing Goal section:\n%s", result)
	}
	llm.mu.Lock()
	firstCall := llm.calls[0]
	llm.mu.Unlock()
	if !strings.Contains(firstCall.user, "Summarize this AI coding session") {
		t.Errorf("expected single-shot prompt, got: %s", firstCall.user[:80])
	}
}

func TestPipeline_MapReduceMultiChunk(t *testing.T) {
	var mapCalls, reduceCalls atomic.Int32
	llm := &fakeLLM{handler: func(idx int, system, user string) (string, TokenUsage, error) {
		if system == MapSystemPrompt {
			mapCalls.Add(1)
			return fmt.Sprintf("ACTIVITIES: chunk %d work\nDECISIONS: (none)\nFILES: file%d.go\nPROBLEMS: (none)\nLEARNINGS: (none)", idx, idx), TokenUsage{}, nil
		}
		reduceCalls.Add(1)
		return "## Goal\nMulti-chunk goal.\n\n## Decisions Made\n- Decision 1\n\n## Files Touched\n- file.go\n\n## Problems Encountered\n- None\n\n## Key Learnings\n- Learned stuff", TokenUsage{}, nil
	}}
	p := newTestPipeline(llm, nil, 3)
	meta := baseMeta()
	content := longContent(20000)

	result, err := p.Summarize(context.Background(), content, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mapCalls.Load() < 2 {
		t.Errorf("map calls = %d, want >= 2", mapCalls.Load())
	}
	if reduceCalls.Load() != 1 {
		t.Errorf("reduce calls = %d, want 1", reduceCalls.Load())
	}
	for _, section := range []string{"## Goal", "## Decisions Made", "## Files Touched", "## Problems Encountered", "## Key Learnings"} {
		if !strings.Contains(result, section) {
			t.Errorf("result missing section %q", section)
		}
	}
}

func TestPipeline_MapFailureSkipped(t *testing.T) {
	llm := &fakeLLM{handler: func(idx int, system, user string) (string, TokenUsage, error) {
		if system == MapSystemPrompt && idx == 1 {
			return "", TokenUsage{}, fmt.Errorf("chunk 1 failed")
		}
		if system == MapSystemPrompt {
			return "ACTIVITIES: did stuff\nFILES: foo.go", TokenUsage{}, nil
		}
		return "## Goal\nReduced.", TokenUsage{}, nil
	}}
	p := newTestPipeline(llm, nil, 3)
	meta := baseMeta()
	content := longContent(20000)

	result, err := p.Summarize(context.Background(), content, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "## Goal") {
		t.Errorf("result should contain reduced output:\n%s", result)
	}
}

func TestPipeline_AllMapsFail(t *testing.T) {
	llm := &fakeLLM{handler: func(idx int, system, user string) (string, TokenUsage, error) {
		return "", TokenUsage{}, fmt.Errorf("always fail")
	}}
	p := newTestPipeline(llm, nil, 3)
	meta := baseMeta()
	content := longContent(20000)

	_, err := p.Summarize(context.Background(), content, meta)
	if err == nil {
		t.Fatal("expected error when all map calls fail")
	}
	if !strings.Contains(err.Error(), "all chunks failed") {
		t.Errorf("error = %v, want 'all chunks failed'", err)
	}
}

func TestPipeline_HierarchicalReduce(t *testing.T) {
	var reduceCalls atomic.Int32
	bigChunkSummary := strings.Repeat("x", 2000)
	llm := &fakeLLM{handler: func(idx int, system, user string) (string, TokenUsage, error) {
		if system == MapSystemPrompt {
			return bigChunkSummary, TokenUsage{}, nil
		}
		reduceCalls.Add(1)
		return "## Goal\nHierarchical result.", TokenUsage{}, nil
	}}
	p := newTestPipeline(llm, nil, 5)
	meta := baseMeta()
	content := longContent(200000)

	result, err := p.Summarize(context.Background(), content, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reduceCalls.Load() < 2 {
		t.Errorf("reduce calls = %d, want >= 2 (hierarchical)", reduceCalls.Load())
	}
	if !strings.Contains(result, "## Goal") {
		t.Errorf("result missing Goal section:\n%s", result)
	}
}

func TestPipeline_RecursionDepthLimit(t *testing.T) {
	var totalCalls atomic.Int32
	hugeOutput := strings.Repeat("y", ReduceContextLimit+1000)
	llm := &fakeLLM{handler: func(idx int, system, user string) (string, TokenUsage, error) {
		totalCalls.Add(1)
		if system == MapSystemPrompt {
			return hugeOutput, TokenUsage{}, nil
		}
		return hugeOutput, TokenUsage{}, nil
	}}
	p := newTestPipeline(llm, nil, 5)
	meta := baseMeta()
	content := longContent(200000)

	_, err := p.Summarize(context.Background(), content, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if totalCalls.Load() > 500 {
		t.Errorf("too many calls (%d), likely infinite recursion", totalCalls.Load())
	}
}

func TestPipeline_FormatHeader_OpenCodeFull(t *testing.T) {
	p := newTestPipeline(&fakeLLM{}, nil, 1)
	meta := SessionMetadata{
		Source:      SourceOpenCode,
		SessionID:   "ses_abc123",
		Title:       "Fix the auth bug",
		Agent:       "build",
		ProjectPath: "/Users/tam/proj",
		CreatedAt:   time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC),
		Duration:    12*time.Minute + 30*time.Second,
		ParentID:    "ses_parent",
		ParentTitle: "Main task",
		Children: []RelatedSession{
			{ID: "ses_c1", Title: "Child 1", Agent: "build"},
			{ID: "ses_c2", Title: "Child 2", Agent: "general"},
		},
		Siblings: []RelatedSession{
			{ID: "ses_s1", Title: "Sibling 1"},
		},
	}

	header := p.formatHeader(meta)

	for _, want := range []string{
		"# Session: Fix the auth bug",
		"- Date: 2026-05-26",
		"- Source: opencode",
		"- Session ID: ses_abc123",
		"- Agent: build",
		"- Project: /Users/tam/proj",
		"- Duration: 12m30s",
		"- Parent Session: Main task (ses_parent)",
		"- Child Sessions:",
		"  - Child 1 (build) [ses_c1]",
		"  - Child 2 (general) [ses_c2]",
		"- Sibling Sessions:",
		"  - Sibling 1 [ses_s1]",
	} {
		if !strings.Contains(header, want) {
			t.Errorf("header missing %q\nfull header:\n%s", want, header)
		}
	}
}

func TestPipeline_FormatHeader_OpenCodeMinimal(t *testing.T) {
	p := newTestPipeline(&fakeLLM{}, nil, 1)
	meta := SessionMetadata{
		Source:    SourceOpenCode,
		SessionID: "ses_xyz",
		Title:     "Quick fix",
		CreatedAt: time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC),
	}

	header := p.formatHeader(meta)

	if !strings.Contains(header, "# Session: Quick fix") {
		t.Errorf("missing title in header:\n%s", header)
	}
	for _, absent := range []string{"Parent Session", "Child Sessions", "Sibling Sessions", "Duration"} {
		if strings.Contains(header, absent) {
			t.Errorf("header should not contain %q:\n%s", absent, header)
		}
	}
}

func TestPipeline_FormatHeader_Claude(t *testing.T) {
	p := newTestPipeline(&fakeLLM{}, nil, 1)
	meta := SessionMetadata{
		Source:    SourceClaude,
		Title:     "Investigate crash",
		CreatedAt: time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC),
	}

	header := p.formatHeader(meta)

	if !strings.Contains(header, "# Session: Investigate crash") {
		t.Errorf("missing title:\n%s", header)
	}
	if !strings.Contains(header, "- Source: claude") {
		t.Errorf("missing source:\n%s", header)
	}
	for _, absent := range []string{"Agent", "Project", "Session ID", "Parent", "Child", "Sibling"} {
		if strings.Contains(header, absent) {
			t.Errorf("Claude header should not contain %q:\n%s", absent, header)
		}
	}
}

func TestPipeline_LookupCalled(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		lookup := &fakeLookup{}
		llm := &fakeLLM{handler: func(idx int, system, user string) (string, TokenUsage, error) {
			return "## Goal\nDone.", TokenUsage{}, nil
		}}
		p := newTestPipeline(llm, lookup, 1)
		meta := baseMeta()

		_, err := p.Summarize(context.Background(), shortContent(), meta)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lookup.mu.Lock()
		defer lookup.mu.Unlock()
		if lookup.called != 1 {
			t.Errorf("lookup called %d times, want 1", lookup.called)
		}
	})

	t.Run("error_continues", func(t *testing.T) {
		lookup := &fakeLookup{err: fmt.Errorf("db unavailable")}
		llm := &fakeLLM{handler: func(idx int, system, user string) (string, TokenUsage, error) {
			return "## Goal\nDone.", TokenUsage{}, nil
		}}
		p := newTestPipeline(llm, lookup, 1)
		meta := baseMeta()

		_, err := p.Summarize(context.Background(), shortContent(), meta)
		if err != nil {
			t.Fatalf("pipeline should continue after lookup error: %v", err)
		}
	})
}

func TestPipeline_NoLookup(t *testing.T) {
	llm := &fakeLLM{handler: func(idx int, system, user string) (string, TokenUsage, error) {
		return "## Goal\nDone.", TokenUsage{}, nil
	}}
	p := newTestPipeline(llm, nil, 1)
	meta := baseMeta()

	result, err := p.Summarize(context.Background(), shortContent(), meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "## Goal") {
		t.Errorf("result missing Goal section:\n%s", result)
	}
}

func TestPipeline_ContextCancellation(t *testing.T) {
	started := make(chan struct{}, 10)
	llm := &fakeLLM{handler: func(idx int, system, user string) (string, TokenUsage, error) {
		started <- struct{}{}
		return "", TokenUsage{}, context.Canceled
	}}
	p := newTestPipeline(llm, nil, 3)
	meta := baseMeta()
	content := longContent(20000)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := p.Summarize(ctx, content, meta)
		done <- err
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error after cancellation")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for cancellation")
	}
}

func TestPipeline_ConcurrencyRespected(t *testing.T) {
	gate := make(chan struct{})
	llm := &fakeLLM{handler: func(idx int, system, user string) (string, TokenUsage, error) {
		if system == MapSystemPrompt {
			gate <- struct{}{}
			<-gate
			return defaultMapResponse, TokenUsage{}, nil
		}
		return "## Goal\nDone.", TokenUsage{}, nil
	}}
	p := newTestPipeline(llm, nil, 2)
	meta := baseMeta()
	content := longContent(30000)

	done := make(chan struct{})
	go func() {
		_, _ = p.Summarize(context.Background(), content, meta)
		close(done)
	}()

	go func() {
		for range gate {
			gate <- struct{}{}
		}
	}()

	<-done

	max := llm.maxInFlight.Load()
	if max > 2 {
		t.Errorf("max in-flight = %d, want <= 2", max)
	}
}
