package flow

import (
	"context"
	"fmt"
	"strings"

	"github.com/nano-brain/nano-brain/internal/summarize"
	"github.com/rs/zerolog"
)

const flowSummarySystemPrompt = `You are a flow visualization assistant. Describe the request flow briefly in 1-2 sentences based on the entry point, call chain, and any external integrations. Focus on what the endpoint does and which services it touches. Provide ONLY the summary — no prefixes, labels, or extra commentary.`

type FlowSummarizer interface {
	Summarize(ctx context.Context, entry string, chain []string, integrations []string) (string, error)
}

type LLMFlowSummarizer struct {
	client *summarize.Client
	logger zerolog.Logger
}

func NewLLMFlowSummarizer(client *summarize.Client, logger zerolog.Logger) *LLMFlowSummarizer {
	return &LLMFlowSummarizer{
		client: client,
		logger: logger.With().Str("component", "flow.summarizer").Logger(),
	}
}

func (s *LLMFlowSummarizer) Summarize(ctx context.Context, entry string, chain []string, integrations []string) (string, error) {
	var b strings.Builder
	b.WriteString("Describe this request flow in 1-2 sentences:\n\n")
	b.WriteString("Entry: ")
	b.WriteString(entry)
	b.WriteString("\n")
	if len(chain) > 0 {
		b.WriteString("Chain: ")
		b.WriteString(strings.Join(chain, " -> "))
		b.WriteString("\n")
	}
	if len(integrations) > 0 {
		b.WriteString("Integrations: ")
		b.WriteString(strings.Join(integrations, ", "))
		b.WriteString("\n")
	}

	result, _, err := s.client.ChatCompletion(ctx, flowSummarySystemPrompt, b.String())
	if err != nil {
		return "", fmt.Errorf("flow summarizer: %w", err)
	}
	return strings.TrimSpace(result), nil
}
