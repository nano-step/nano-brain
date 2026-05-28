package summarize

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

func newTestClient(serverURL string, apiKey string) *Client {
	cfg := config.SummarizationConfig{
		ProviderURL: serverURL,
		APIKey:      apiKey,
		Model:       "test-model",
		MaxTokens:   100,
	}
	logger := zerolog.Nop()
	c := New(cfg, logger)
	c.backoff = func(_ int) time.Duration { return time.Millisecond }
	return c
}

func TestChatCompletion_NonStreaming_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"choices": [{"message": {"content": "hello"}}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 2, "total_tokens": 12}
		}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "test-key")
	content, usage, err := c.ChatCompletion(context.Background(), "sys", "usr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "hello" {
		t.Errorf("content = %q, want %q", content, "hello")
	}
	if usage.PromptTokens != 10 || usage.CompletionTokens != 2 || usage.TotalTokens != 12 {
		t.Errorf("usage = %+v, want {10, 2, 12}", usage)
	}
}

func TestChatCompletion_SSE_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{}}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":1,\"total_tokens\":6}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "test-key")
	content, usage, err := c.ChatCompletion(context.Background(), "sys", "usr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "hello" {
		t.Errorf("content = %q, want %q", content, "hello")
	}
	if usage.PromptTokens != 5 || usage.CompletionTokens != 1 || usage.TotalTokens != 6 {
		t.Errorf("usage = %+v, want {5, 1, 6}", usage)
	}
}

func TestChatCompletion_SSE_EmptyLines_Ignored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "\n")
		fmt.Fprint(w, ": this is a comment\n")
		fmt.Fprint(w, "event: ping\n")
		fmt.Fprint(w, "\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		fmt.Fprint(w, "\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "test-key")
	content, _, err := c.ChatCompletion(context.Background(), "sys", "usr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "ok" {
		t.Errorf("content = %q, want %q", content, "ok")
	}
}

func TestChatCompletion_RetryOn429(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, "rate limited")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "test-key")
	content, _, err := c.ChatCompletion(context.Background(), "sys", "usr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "ok" {
		t.Errorf("content = %q, want %q", content, "ok")
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestChatCompletion_RetryOn500(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "server error")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "test-key")
	_, _, err := c.ChatCompletion(context.Background(), "sys", "usr")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestChatCompletion_NoRetryOn400(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "bad request")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "test-key")
	_, _, err := c.ChatCompletion(context.Background(), "sys", "usr")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}
}

func TestChatCompletion_NoRetryOn401(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "unauthorized")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "test-key")
	_, _, err := c.ChatCompletion(context.Background(), "sys", "usr")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}
}

func TestChatCompletion_ContextCancellation(t *testing.T) {
	started := make(chan struct{})
	unblock := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-unblock
	}))
	defer srv.Close()
	defer close(unblock)

	c := newTestClient(srv.URL, "test-key")
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	var retErr error
	go func() {
		_, _, retErr = c.ChatCompletion(ctx, "sys", "usr")
		close(done)
	}()

	<-started
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for cancellation")
	}

	if retErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(retErr.Error(), "context canceled") {
		t.Errorf("error = %v, want context canceled", retErr)
	}
}

func TestChatCompletion_AuthHeader(t *testing.T) {
	t.Run("with_api_key", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{}}`)
		}))
		defer srv.Close()

		c := newTestClient(srv.URL, "test-key")
		_, _, err := c.ChatCompletion(context.Background(), "sys", "usr")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotAuth != "Bearer test-key" {
			t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-key")
		}
	})

	t.Run("without_api_key", func(t *testing.T) {
		var gotAuth string
		var hasAuth bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			_, hasAuth = r.Header["Authorization"]
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{}}`)
		}))
		defer srv.Close()

		c := newTestClient(srv.URL, "")
		_, _, err := c.ChatCompletion(context.Background(), "sys", "usr")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hasAuth {
			t.Errorf("Authorization header should be absent, got %q", gotAuth)
		}
	})
}
