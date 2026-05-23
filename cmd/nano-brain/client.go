package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

func getBaseURL() string {
	host := os.Getenv("NANO_BRAIN_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("NANO_BRAIN_PORT")
	if port == "" {
		port = "3100"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

func doRequest(method, url string, body io.Reader) ([]byte, int, error) {
	host := os.Getenv("NANO_BRAIN_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("NANO_BRAIN_PORT")
	if port == "" {
		port = "3100"
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "dial tcp") {
			return nil, 0, fmt.Errorf("cannot connect to nano-brain server at %s:%s", host, port)
		}
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return data, resp.StatusCode, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(data))
	}

	return data, resp.StatusCode, nil
}
