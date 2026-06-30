package summarize

import (
	"context"

	"github.com/nano-brain/nano-brain/internal/harvest"
)

// Compile-time guarantee: *HarvestSummarizer satisfies the exact inline
// interface the harvesters (claudecode.go, engine.go) type-assert against on
// their content-unchanged skip path. If the adapter method signature drifts
// (e.g. gains an error return), this fails to compile and the backfill would
// silently stop firing (ok=false at runtime). 999.1 / summary-disk-backfill.
var _ interface {
	EnsureSummaryOnDisk(context.Context, string, harvest.SummaryMeta)
} = (*HarvestSummarizer)(nil)
