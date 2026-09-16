package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

func TestNewPoolWithRetryMalformedDSNFailsImmediately(t *testing.T) {
	logger := zerolog.Nop()
	cfg := config.DatabaseConfig{URL: "not a url :::"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	_, err := NewPoolWithRetry(ctx, cfg, logger)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error for malformed DSN")
	}
	if !strings.Contains(err.Error(), "database URL") {
		t.Errorf("err = %v, want database URL parse error", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("malformed DSN must fail immediately, took %v", elapsed)
	}
}

func TestNewPoolWithRetryCancellationAborts(t *testing.T) {
	logger := zerolog.Nop()
	// Port 1 refuses connections immediately on localhost.
	cfg := config.DatabaseConfig{URL: "postgres://nano-brain:nano-brain@127.0.0.1:1/nanobrain_test?sslmode=disable"}
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	_, err := NewPoolWithRetry(ctx, cfg, logger)
	if err == nil {
		t.Fatal("expected error when context is cancelled during retry")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("err = %v, want cancellation error", err)
	}
}
