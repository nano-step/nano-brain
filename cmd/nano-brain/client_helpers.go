package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const healthPollInterval = 200 * time.Millisecond

// promptStartServer writes the Y/n prompt to writer, reads a single line
// from reader, and returns true on "Y", "y", or empty input (whitespace
// trimmed). Any other response, or a read error, returns false.
func promptStartServer(reader io.Reader, writer io.Writer) bool {
	fmt.Fprint(writer, "Start server now? [Y/n]: ")
	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		return false
	}
	answer := strings.TrimSpace(scanner.Text())
	if answer == "" {
		return true
	}
	switch answer[0] {
	case 'Y', 'y':
		return true
	}
	return false
}

// isTTY reports whether BOTH os.Stdin and os.Stderr are connected to a
// character device (terminal). Stdlib-only: uses os.ModeCharDevice from
// each file's Stat() mode. Returns false on any Stat error.
func isTTY() bool {
	return isCharDevice(os.Stdin) && isCharDevice(os.Stderr)
}

func isCharDevice(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// isNpxLaunched reports whether the CLI was launched via npx/npm. npx and
// npm both leave breadcrumb environment variables in the child process.
func isNpxLaunched() bool {
	if os.Getenv("npm_execpath") != "" {
		return true
	}
	if os.Getenv("npm_package_name") != "" {
		return true
	}
	return false
}

// suggestStartCommand returns the user-facing command to start the
// nano-brain server, tailored to how this binary was launched.
func suggestStartCommand() string {
	if isNpxLaunched() {
		return "npx @nano-step/nano-brain@beta serve -d"
	}
	return "nano-brain serve -d"
}

// formatConnectError builds the 3-line user-facing error shown when the
// CLI cannot reach the server (header, hint, action).
func formatConnectError(host string, port int) string {
	return fmt.Sprintf(
		"Error: cannot connect to nano-brain server at %s:%d\n"+
			"The server does not appear to be running.\n"+
			"Run this to start it: %s",
		host, port, suggestStartCommand(),
	)
}

// waitForServerHealthy polls GET <baseURL>/api/status every healthPollInterval
// and returns nil on the first HTTP 200 response. If timeout elapses before
// success, it returns an error describing the deadline.
func waitForServerHealthy(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := getBaseURL() + "/api/status"

	for {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err == nil {
			resp, doErr := httpClient.Do(req)
			if doErr == nil {
				status := resp.StatusCode
				_ = resp.Body.Close()
				if status == http.StatusOK {
					return nil
				}
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("server did not become healthy within %s", timeout)
		}

		remaining := time.Until(deadline)
		sleep := healthPollInterval
		if remaining < sleep {
			sleep = remaining
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}
	}
}
