//go:build integration

package storage

import (
	"context"
	"testing"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

func TestPoolConnects(t *testing.T) {
	ctx := context.Background()
	cfg := config.DatabaseConfig{URL: "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable"}
	logger := zerolog.Nop()

	pool, err := NewPool(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer ClosePool(pool)
}

func TestPoolPing(t *testing.T) {
	ctx := context.Background()
	cfg := config.DatabaseConfig{URL: "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable"}
	logger := zerolog.Nop()

	pool, err := NewPool(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer ClosePool(pool)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
