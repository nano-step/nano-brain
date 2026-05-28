package summarize

import (
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/harvest"
)

func TestSessionMetadata_WorkspaceHashCopiedFromSummaryMeta(t *testing.T) {
	src := harvest.SummaryMeta{
		Source:        "opencode",
		SessionID:     "abc",
		Title:         "Test Session",
		CreatedAt:     time.Now(),
		WorkspaceHash: "test-ws-hash-xyz",
	}

	dst := SessionMetadata{
		Source:        Source(src.Source),
		SessionID:     src.SessionID,
		Title:         src.Title,
		CreatedAt:     src.CreatedAt,
		WorkspaceHash: src.WorkspaceHash,
	}

	if dst.WorkspaceHash != "test-ws-hash-xyz" {
		t.Errorf("WorkspaceHash = %q, want %q", dst.WorkspaceHash, "test-ws-hash-xyz")
	}
	if dst.Source != SourceOpenCode {
		t.Errorf("Source = %q, want %q", dst.Source, SourceOpenCode)
	}
	if dst.SessionID != "abc" {
		t.Errorf("SessionID = %q, want %q", dst.SessionID, "abc")
	}
}
