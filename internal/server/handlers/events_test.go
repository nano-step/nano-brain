package handlers_test

import (
	"bufio"
	"context"
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

func setupSSERequest(t *testing.T, bus *eventbus.Bus, workspace string) (*echo.Echo, *httptest.ResponseRecorder, *http.Request, context.CancelFunc) {
	t.Helper()
	e := echo.New()
	h := handlers.EventsHandler(bus, zerolog.Nop())
	e.GET("/api/v1/events", h)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?workspace="+workspace, nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	return e, rec, req, cancel
}

func readSSEEvent(t *testing.T, body string) (eventType, data string) {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		}
	}
	return
}

func TestEventsHandler_HelloDelivered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bus := eventbus.New(ctx)
	defer bus.Close()

	e := echo.New()
	h := handlers.EventsHandler(bus, zerolog.Nop())
	e.GET("/api/v1/events", h)

	reqCtx, reqCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer reqCancel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?workspace=abc123", nil)
	req = req.WithContext(reqCtx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		e.ServeHTTP(rec, req)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	reqCancel()
	<-done

	body := rec.Body.String()
	if !strings.Contains(body, "event: hello") {
		t.Fatalf("expected hello event, got: %s", body)
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got: %s", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("expected no-cache, got: %s", rec.Header().Get("Cache-Control"))
	}
	if rec.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatalf("expected X-Accel-Buffering=no, got: %s", rec.Header().Get("X-Accel-Buffering"))
	}
}

func TestEventsHandler_PerIPCap(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bus := eventbus.New(ctx)
	defer bus.Close()

	h := handlers.EventsHandler(bus, zerolog.Nop())
	e := echo.New()
	e.GET("/api/v1/events", h)

	cancels := make([]context.CancelFunc, 8)
	for i := 0; i < 8; i++ {
		reqCtx, reqCancel := context.WithCancel(context.Background())
		cancels[i] = reqCancel
		req := httptest.NewRequest(http.MethodGet, "/api/v1/events?workspace=ws", nil)
		req.RemoteAddr = "10.0.0.1:9999"
		req = req.WithContext(reqCtx)
		rec := httptest.NewRecorder()
		go e.ServeHTTP(rec, req)
	}

	time.Sleep(50 * time.Millisecond)

	reqCtx9, reqCancel9 := context.WithTimeout(context.Background(), time.Second)
	defer reqCancel9()
	req9 := httptest.NewRequest(http.MethodGet, "/api/v1/events?workspace=ws", nil)
	req9.RemoteAddr = "10.0.0.1:9999"
	req9 = req9.WithContext(reqCtx9)
	rec9 := httptest.NewRecorder()
	e.ServeHTTP(rec9, req9)

	if rec9.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec9.Code)
	}

	for _, c := range cancels {
		c()
	}
}

func TestEventsHandler_WorkspaceFilter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bus := eventbus.New(ctx)
	defer bus.Close()

	e := echo.New()
	h := handlers.EventsHandler(bus, zerolog.Nop())
	e.GET("/api/v1/events", h)

	reqCtx, reqCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer reqCancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?workspace=wsA", nil)
	req = req.WithContext(reqCtx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		e.ServeHTTP(rec, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	bus.Publish(eventbus.Event{Type: "test", Workspace: "wsB"})
	time.Sleep(50 * time.Millisecond)

	reqCancel()
	<-done

	body := rec.Body.String()
	if strings.Contains(body, `"workspace":"wsB"`) {
		t.Fatalf("should not receive event for workspace wsB, got: %s", body)
	}
}

func TestEventsHandler_HeartbeatComment(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bus := eventbus.New(ctx)
	defer bus.Close()

	handlers.SetSSEHeartbeatInterval(50 * time.Millisecond)
	handlers.SetSSEIdleTimeout(5 * time.Second)
	t.Cleanup(func() {
		handlers.SetSSEHeartbeatInterval(30 * time.Second)
		handlers.SetSSEIdleTimeout(5 * time.Minute)
	})

	e := echo.New()
	h := handlers.EventsHandler(bus, zerolog.Nop())
	e.GET("/api/v1/events", h)

	reqCtx, reqCancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer reqCancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?workspace=ws", nil)
	req = req.WithContext(reqCtx)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, ":\n") {
		t.Fatalf("expected heartbeat comment (: line), got: %q", body)
	}
}

func TestEventsHandler_IdleTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bus := eventbus.New(ctx)
	defer bus.Close()

	handlers.SetSSEIdleTimeout(100 * time.Millisecond)
	t.Cleanup(func() { handlers.SetSSEIdleTimeout(5 * time.Minute) })

	e := echo.New()
	h := handlers.EventsHandler(bus, zerolog.Nop())
	e.GET("/api/v1/events", h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?workspace=ws", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	e.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Fatalf("idle timeout should close within ~200ms, took %v", elapsed)
	}
}
