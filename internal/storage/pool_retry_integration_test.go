//go:build integration

package storage

import (
	"context"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

// proxyConn copies bytes between a client connection and the upstream PG
// listener until either side closes.
func proxyConn(client net.Conn, upstream string) {
	defer client.Close()
	server, err := net.Dial("tcp", upstream)
	if err != nil {
		return
	}
	defer server.Close()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(server, client); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, server); done <- struct{}{} }()
	<-done
}

// startDelayedProxy returns a localhost:0 address that refuses TCP
// connections until delay elapses, then proxies to the upstream PG listener.
// Dials before the delay fail with connection refused — exactly the
// "PostgreSQL temporarily unavailable" scenario a managed service must
// survive.
func startDelayedProxy(t *testing.T, upstream string, delay time.Duration) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("proxy listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	go func() {
		time.Sleep(delay)
		for attempt := 0; attempt < 5; attempt++ {
			ln2, err := net.Listen("tcp", addr)
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			defer ln2.Close()
			for {
				conn, err := ln2.Accept()
				if err != nil {
					return
				}
				go proxyConn(conn, upstream)
			}
		}
	}()
	return addr
}

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	if v := os.Getenv("NANO_BRAIN_TEST_DATABASE_URL"); v != "" {
		return v
	}
	// Prefer localhost when running directly on the host (docker compose PG).
	if conn, err := net.DialTimeout("tcp", "localhost:5432", 500*time.Millisecond); err == nil {
		_ = conn.Close()
		return "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_test?sslmode=disable"
	}
	return "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_test?sslmode=disable"
}

// pgHostPort extracts the host:port a TCP proxy should dial from a postgres
// DSN of the form postgres://user:pass@host:port/db?opts.
func pgHostPort(dsn string) string {
	rest := strings.TrimPrefix(dsn, "postgres://")
	at := strings.IndexByte(rest, '@')
	hostPort := rest
	if at >= 0 {
		hostPort = rest[at+1:]
	}
	if i := strings.IndexByte(hostPort, '/'); i >= 0 {
		hostPort = hostPort[:i]
	}
	return hostPort
}

// TestNewPoolWithRetrySurvivesPostgreSQLDelay is the server-start resilience
// contract: while PostgreSQL is unreachable, retry keeps trying with bounded
// backoff; once it comes back, the pool connects without any supervisor
// restart. Fail-fast NewPool behavior is preserved for one-shot callers.
func TestNewPoolWithRetrySurvivesPostgreSQLDelay(t *testing.T) {
	logger := zerolog.Nop()
	upstream := pgHostPort(testDatabaseURL(t))
	proxyAddr := startDelayedProxy(t, upstream, 3*time.Second)

	cfg := config.DatabaseConfig{URL: "postgres://nanobrain:nanobrain@" + proxyAddr + "/nanobrain_test?sslmode=disable"}

	// While PG is down, fail-fast NewPool must error (single attempt).
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, err := NewPool(ctx, cfg, logger)
	cancel()
	if err == nil {
		t.Fatal("NewPool must fail fast while PostgreSQL is unreachable")
	}

	// The retry helper survives the gap and connects once PG is back.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()
	start := time.Now()
	pool, err := NewPoolWithRetry(ctx2, cfg, logger)
	if err != nil {
		t.Fatalf("NewPoolWithRetry should reconnect after PostgreSQL returns: %v", err)
	}
	defer ClosePool(pool)
	if elapsed := time.Since(start); elapsed < 2*time.Second {
		t.Errorf("retry should have waited for the delayed PG, connected in %v", elapsed)
	}
	if err := pool.Ping(ctx2); err != nil {
		t.Fatalf("pool ping after reconnect: %v", err)
	}
}
