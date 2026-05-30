package handlers

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/eventbus"
	"github.com/rs/zerolog"
)

// SSE tuning knobs. Tests override these via t.Cleanup to speed up assertions.
var (
	sseHeartbeatInterval = 30 * time.Second
	sseIdleTimeout       = 5 * time.Minute
	sseMaxConnPerIP      = 8
)

// SetSSEHeartbeatInterval overrides the heartbeat interval (for testing).
func SetSSEHeartbeatInterval(d time.Duration) { sseHeartbeatInterval = d }

// SetSSEIdleTimeout overrides the idle timeout (for testing).
func SetSSEIdleTimeout(d time.Duration) { sseIdleTimeout = d }

// SetSSEMaxConnPerIP overrides the per-IP connection cap (for testing).
func SetSSEMaxConnPerIP(n int) { sseMaxConnPerIP = n }

// EventsHandler returns an Echo handler that streams events from the bus as SSE.
func EventsHandler(bus *eventbus.Bus, logger zerolog.Logger) echo.HandlerFunc {
	var (
		mu    sync.Mutex
		perIP = make(map[string]int)
	)

	return func(c echo.Context) error {
		ip := remoteIP(c.Request())
		mu.Lock()
		if perIP[ip] >= sseMaxConnPerIP {
			mu.Unlock()
			return echo.NewHTTPError(http.StatusTooManyRequests, "too many SSE connections from this IP")
		}
		perIP[ip]++
		mu.Unlock()
		defer func() {
			mu.Lock()
			perIP[ip]--
			if perIP[ip] <= 0 {
				delete(perIP, ip)
			}
			mu.Unlock()
		}()

		h := c.Response().Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("X-Accel-Buffering", "no")
		c.Response().WriteHeader(http.StatusOK)

		flusher, ok := c.Response().Writer.(http.Flusher)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "streaming unsupported")
		}

		workspace := c.QueryParam("workspace")
		ch, unsubscribe := bus.Subscribe(workspace)
		defer unsubscribe()

		helloPayload, _ := json.Marshal(map[string]string{
			"workspace": workspace,
			"ts":        time.Now().UTC().Format(time.RFC3339Nano),
		})
		writeSSEEvent(c.Response().Writer, eventbus.Event{
			Type: "hello", Workspace: workspace, Payload: helloPayload, TS: time.Now(),
		}, flusher)

		heartbeat := time.NewTicker(sseHeartbeatInterval)
		defer heartbeat.Stop()
		idle := time.NewTimer(sseIdleTimeout)
		defer idle.Stop()

		for {
			select {
			case <-c.Request().Context().Done():
				return nil
			case <-idle.C:
				logger.Debug().Str("ip", ip).Msg("SSE idle timeout; closing")
				return nil
			case ev, ok := <-ch:
				if !ok {
					return nil
				}
				writeSSEEvent(c.Response().Writer, ev, flusher)
				if !idle.Stop() {
					select {
					case <-idle.C:
					default:
					}
				}
				idle.Reset(sseIdleTimeout)
			case <-heartbeat.C:
				fmt.Fprint(c.Response().Writer, ":\n\n")
				flusher.Flush()
			}
		}
	}
}

func writeSSEEvent(w interface{ Write([]byte) (int, error) }, ev eventbus.Event, flusher http.Flusher) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data)
	flusher.Flush()
}

func remoteIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
