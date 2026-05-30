package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// runServeDaemonFn is the daemon launcher hook. Tests override it.
var runServeDaemonFn = runServeDaemon

// promptReader / promptWriter are the I/O streams used by the prompt.
// Tests override them.
var (
	promptReader io.Reader = os.Stdin
	promptWriter io.Writer = os.Stderr
)

// isTTYFn is the TTY detector hook. Tests override it.
var isTTYFn = isTTY

const serverHealthTimeout = 10 * time.Second

func resolveHostPort() (string, int) {
	host := os.Getenv("NANO_BRAIN_HOST")
	if host == "" {
		host = "localhost"
	}
	port := 3100
	if p := os.Getenv("NANO_BRAIN_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 && v <= 65535 {
			port = v
		}
	}
	return host, port
}

func getBaseURL() string {
	host, port := resolveHostPort()
	return fmt.Sprintf("http://%s:%d", host, port)
}

func doRequest(method, url string, body io.Reader) ([]byte, int, error) {
	host, port := resolveHostPort()

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to read request body: %w", err)
		}
	}

	data, status, err := sendRequest(method, url, bodyBytes)
	if err == nil {
		if status >= 400 {
			return data, status, fmt.Errorf("server returned %d: %s", status, string(data))
		}
		return data, status, nil
	}

	if !isConnectionRefused(err) {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}

	if recovered := recoverFromConnectionRefused(host, port); !recovered {
		return nil, 0, fmt.Errorf("cannot connect to nano-brain server at %s:%d", host, port)
	}

	data, status, err = sendRequest(method, url, bodyBytes)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed after auto-start: %w", err)
	}
	if status >= 400 {
		return data, status, fmt.Errorf("server returned %d: %s", status, string(data))
	}
	return data, status, nil
}

func sendRequest(method, url string, bodyBytes []byte) ([]byte, int, error) {
	var bodyReader io.Reader
	if bodyBytes != nil {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

func isConnectionRefused(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") || strings.Contains(msg, "dial tcp")
}

// recoverFromConnectionRefused implements the connect-error recovery flow:
// print formatted error, optionally prompt to auto-start the daemon, wait
// for health, and signal whether the caller should retry the original
// request. Returns true ONLY when the daemon was started and reported
// healthy within serverHealthTimeout.
func recoverFromConnectionRefused(host string, port int) bool {
	msg := formatConnectError(host, port)

	if os.Getenv("NANO_BRAIN_NO_AUTO_START") == "1" || !isTTYFn() {
		fmt.Fprintln(os.Stderr, msg)
		return false
	}

	fmt.Fprintln(os.Stderr, msg)
	if !promptStartServer(promptReader, promptWriter) {
		return false
	}

	runServeDaemonFn(config.ResolveConfigPath(""))

	if err := waitForServerHealthy(serverHealthTimeout); err != nil {
		fmt.Fprintln(os.Stderr, "Server started but did not become healthy in 10s. Check logs: ~/.nano-brain/logs/nano-brain.log")
		return false
	}
	return true
}
