//go:build integration

package handlers_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/eventbus"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/rs/zerolog"
)

func TestEventsIntegration_ReindexPublishesSequence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := eventbus.New(ctx)
	defer bus.Close()

	handlers.SetSSEIdleTimeout(10 * time.Second)
	t.Cleanup(func() { handlers.SetSSEIdleTimeout(5 * time.Minute) })

	e := echo.New()
	e.GET("/api/v1/events", handlers.EventsHandler(bus, zerolog.Nop()))

	ts := httptest.NewServer(e)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/events?workspace=integ-ws")
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	readEvent := func(timeout time.Duration) (string, map[string]any) {
		deadline := time.After(timeout)
		var evType, evData string
		for {
			select {
			case <-deadline:
				t.Fatalf("timed out waiting for event (have type=%q)", evType)
			default:
			}
			if !scanner.Scan() {
				t.Fatalf("scanner ended, err=%v", scanner.Err())
			}
			line := scanner.Text()
			if strings.HasPrefix(line, "event: ") {
				evType = strings.TrimPrefix(line, "event: ")
			}
			if strings.HasPrefix(line, "data: ") {
				evData = strings.TrimPrefix(line, "data: ")
			}
			if line == "" && evType != "" {
				var m map[string]any
				_ = json.Unmarshal([]byte(evData), &m)
				return evType, m
			}
		}
	}

	evType, _ := readEvent(2 * time.Second)
	if evType != "hello" {
		t.Fatalf("first event should be hello, got %q", evType)
	}

	bus.Publish(eventbus.Event{
		Type:      "reindex",
		Workspace: "integ-ws",
		Payload:   json.RawMessage(`{"state":"started"}`),
	})
	bus.Publish(eventbus.Event{
		Type:      "reindex",
		Workspace: "integ-ws",
		Payload:   json.RawMessage(`{"state":"completed","enqueued":5}`),
	})

	evType, payload := readEvent(2 * time.Second)
	if evType != "reindex" {
		t.Fatalf("expected reindex event, got %q", evType)
	}
	if state, ok := payload["state"].(string); !ok || state != "started" {
		t.Fatalf("expected state=started, got %v", payload)
	}

	evType, payload = readEvent(2 * time.Second)
	if evType != "reindex" {
		t.Fatalf("expected reindex event, got %q", evType)
	}
	if state, ok := payload["state"].(string); !ok || state != "completed" {
		t.Fatalf("expected state=completed, got %v", payload)
	}

	cancel()
}
