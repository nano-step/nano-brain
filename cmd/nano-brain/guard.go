package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

func isContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}
	return false
}

func resolvePort() int {
	if p := os.Getenv("NANO_BRAIN_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 && v <= 65535 {
			return v
		}
	}
	return 3100
}

func autoConfigContainer() bool {
	if !isContainer() {
		return false
	}
	if os.Getenv("NANO_BRAIN_HOST") != "" {
		return false
	}
	os.Setenv("NANO_BRAIN_HOST", "host.docker.internal")
	fmt.Fprintf(os.Stderr, "Detected container environment. Using host.docker.internal:%d for server communication.\n", resolvePort())
	return true
}

func probeServer(addr string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/api/status", addr))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

func checkExistingServer(port int) error {
	addr := fmt.Sprintf("localhost:%d", port)
	if probeServer(addr) {
		return fmt.Errorf("nano-brain server already running at %s", addr)
	}
	if os.Getenv("NANO_BRAIN_HOST") == "" {
		dockerAddr := fmt.Sprintf("host.docker.internal:%d", port)
		if probeServer(dockerAddr) {
			return fmt.Errorf("nano-brain server already running at %s", dockerAddr)
		}
	}
	return nil
}

func guardBeforeStart() error {
	autoConfigContainer()
	if os.Getenv("NANO_BRAIN_ALLOW_DUPLICATE_SERVER") == "1" {
		return nil
	}
	return checkExistingServer(resolvePort())
}
