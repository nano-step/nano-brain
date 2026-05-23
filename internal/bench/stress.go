package bench

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"golang.org/x/sync/errgroup"
)

// StressConfig configures a concurrency stress test.
type StressConfig struct {
	Concurrency   int
	DocsPerWriter int
	WorkspaceHash string
}

// StressResult holds the outcome of a stress test run.
type StressResult struct {
	Concurrency       int      `json:"concurrency"`
	DocsPerWriter     int      `json:"docs_per_writer"`
	DocumentsWritten  int      `json:"documents_written"`
	DocumentsVerified int      `json:"documents_verified"`
	Violations        int      `json:"violations"`
	Errors            []string `json:"errors,omitempty"`
	DurationMs        float64  `json:"duration_ms"`
}

// StressUpsertParams mirrors the fields needed by UpsertDocument.
type StressUpsertParams struct {
	WorkspaceHash string
	ContentHash   string
	Title         string
	Content       string
	SourcePath    string
	Collection    string
	Tags          []string
	Metadata      pqtype.NullRawMessage
	SupersedesID  uuid.NullUUID
}

// StressUpsertRow mirrors the return of UpsertDocument.
type StressUpsertRow struct {
	ID            uuid.UUID
	ContentHash   string
	Collection    string
	WorkspaceHash string
}

// StressWriter is the minimal interface RunStress needs for storage.
type StressWriter interface {
	UpsertDocument(ctx context.Context, arg StressUpsertParams) (StressUpsertRow, error)
	CountDocumentsByWorkspace(ctx context.Context, workspaceHash string) (int64, error)
}

// RunStress fan-outs N goroutines that each write DocsPerWriter documents,
// then verifies the expected document count.
func RunStress(ctx context.Context, writer StressWriter, cfg StressConfig) (*StressResult, error) {
	if cfg.Concurrency < 1 {
		return nil, fmt.Errorf("concurrency must be >= 1, got %d", cfg.Concurrency)
	}
	if cfg.DocsPerWriter < 1 {
		return nil, fmt.Errorf("docs-per-writer must be >= 1, got %d", cfg.DocsPerWriter)
	}

	countBefore, err := writer.CountDocumentsByWorkspace(ctx, cfg.WorkspaceHash)
	if err != nil {
		return nil, fmt.Errorf("counting documents before stress: %w", err)
	}

	var (
		mu       sync.Mutex
		errMsgs  []string
		written  int
	)

	var barrier sync.WaitGroup
	barrier.Add(cfg.Concurrency)

	start := time.Now()

	g, gctx := errgroup.WithContext(ctx)
	for gi := 0; gi < cfg.Concurrency; gi++ {
		gi := gi
		g.Go(func() error {
			barrier.Done()
			barrier.Wait()

			for di := 0; di < cfg.DocsPerWriter; di++ {
				title := fmt.Sprintf("stress-g%d-d%d", gi, di)
				hash := fmt.Sprintf("%x", sha256.Sum256([]byte(title)))

				_, upsertErr := writer.UpsertDocument(gctx, StressUpsertParams{
					WorkspaceHash: cfg.WorkspaceHash,
					ContentHash:   hash,
					Title:         title,
					Content:       "",
					SourcePath:    fmt.Sprintf("stress/g%d/doc%d.md", gi, di),
					Collection:    "stress-test",
					Tags:          nil,
				})
				if upsertErr != nil {
					mu.Lock()
					errMsgs = append(errMsgs, fmt.Sprintf("g%d-d%d: %v", gi, di, upsertErr))
					mu.Unlock()
					continue
				}
				mu.Lock()
				written++
				mu.Unlock()
			}
			return nil
		})
	}

	_ = g.Wait()
	duration := time.Since(start)

	countAfter, err := writer.CountDocumentsByWorkspace(ctx, cfg.WorkspaceHash)
	if err != nil {
		return nil, fmt.Errorf("counting documents after stress: %w", err)
	}

	expected := int64(cfg.Concurrency * cfg.DocsPerWriter)
	actualNew := countAfter - countBefore
	violations := 0
	if actualNew != expected {
		violations = 1
	}

	return &StressResult{
		Concurrency:       cfg.Concurrency,
		DocsPerWriter:     cfg.DocsPerWriter,
		DocumentsWritten:  written,
		DocumentsVerified: int(actualNew),
		Violations:        violations,
		Errors:            errMsgs,
		DurationMs:        float64(duration.Milliseconds()),
	}, nil
}
