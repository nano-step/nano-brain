package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/eventbus"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/rs/zerolog"
)

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

// helloRecorder closes wrote on the first write, which the handler makes only
// after it has counted the connection against its IP.
type helloRecorder struct {
	*httptest.ResponseRecorder
	once  sync.Once
	wrote chan struct{}
}

func (r *helloRecorder) Write(p []byte) (int, error) {
	r.once.Do(func() { close(r.wrote) })
	return r.ResponseRecorder.Write(p)
}

// The per-IP cap counts the connection peer. X-Forwarded-For is set by the
// client, so honouring it would let one peer evade the cap, or fill the
// bucket of an address it does not hold (#673).
func TestEventsHandler_PerIPCapIgnoresForwardedFor(t *testing.T) {
	const maxConn = 8 // sseMaxConnPerIP

	tests := []struct {
		name      string
		holdPeer  string
		holdXFF   func(i int) string
		probePeer string
		probeXFF  string
		want      int
	}{
		{
			name:      "different XFF values from one peer count as one IP",
			holdPeer:  "10.0.0.1:9999",
			holdXFF:   func(i int) string { return fmt.Sprintf("198.51.100.%d", i+1) },
			probePeer: "10.0.0.1:9999",
			probeXFF:  "203.0.113.9, 198.51.100.200",
			want:      http.StatusTooManyRequests,
		},
		{
			name:      "spoofed XFF does not occupy another IP's bucket",
			holdPeer:  "10.0.0.9:9999",
			holdXFF:   func(int) string { return "10.0.0.2" },
			probePeer: "10.0.0.2:9999",
			want:      http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			bus := eventbus.New(ctx)
			defer bus.Close()

			e := echo.New()
			e.GET("/api/v1/events", handlers.EventsHandler(bus, zerolog.Nop()))

			holdCtx, release := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			defer wg.Wait()
			defer release()
			for i := 0; i < maxConn; i++ {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/events?workspace=ws", nil).WithContext(holdCtx)
				req.RemoteAddr = tt.holdPeer
				req.Header.Set("X-Forwarded-For", tt.holdXFF(i))
				rec := &helloRecorder{ResponseRecorder: httptest.NewRecorder(), wrote: make(chan struct{})}
				wg.Add(1)
				go func() {
					defer wg.Done()
					e.ServeHTTP(rec, req)
				}()
				select {
				case <-rec.wrote:
				case <-time.After(5 * time.Second):
					t.Fatalf("holder %d never connected", i)
				}
			}

			probeCtx, probeCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer probeCancel()
			probe := httptest.NewRequest(http.MethodGet, "/api/v1/events?workspace=ws", nil).WithContext(probeCtx)
			probe.RemoteAddr = tt.probePeer
			if tt.probeXFF != "" {
				probe.Header.Set("X-Forwarded-For", tt.probeXFF)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, probe)

			if rec.Code != tt.want {
				t.Errorf("status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
