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

// isContainerFn is the container-environment detector hook. Tests override it.
// Default delegates to isContainer() in guard.go.
var isContainerFn = isContainer

const serverHealthTimeout = 10 * time.Second

func resolveHostPort() (string, int) {
	envHost := os.Getenv("NANO_BRAIN_HOST")
	envPort := os.Getenv("NANO_BRAIN_PORT")
	
	cfg, _ := config.Load(config.ResolveConfigPath(""))
	
	// Host resolution with correct precedence:
	// 1. ENV var (highest)
	// 2. Container auto-detection
	// 3. Config file
	// 4. Hard-coded default (lowest)
	host := envHost
	if host == "" {
		if isContainerFn() {
			host = "host.docker.internal"
		} else if cfg != nil && cfg.Server.Host != "" {
			host = cfg.Server.Host
		} else {
			host = "localhost"
		}
	}
	
	port := 3100
	if envPort != "" {
		if v, err := strconv.Atoi(envPort); err == nil && v > 0 && v <= 65535 {
			port = v
		}
	} else if cfg != nil && cfg.Server.Port > 0 {
		port = cfg.Server.Port
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
