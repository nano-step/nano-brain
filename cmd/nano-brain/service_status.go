package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
)

// serviceHealthTimeout bounds the direct /health probe so `service status`
// never hangs behind a dead manager process.
const serviceHealthTimeout = 2 * time.Second

// probeServiceHealth performs a direct configured /health probe with a
// bounded timeout and no auto-start side effects. It reports reachability
// separately from readiness so an active-but-unready (e.g. PostgreSQL
// still starting) service is distinguishable from a dead one.
func probeServiceHealth(host string, port int) (reachable, ready bool, version, probeErr string) {
	url := fmt.Sprintf("http://%s:%d/health", host, port)
	client := &http.Client{Timeout: serviceHealthTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return false, false, "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return true, false, "", "health endpoint requires authentication — configure the local /health bypass"
	}
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return true, false, "", "health response could not be read"
	}
	var h struct {
		Ready   bool   `json:"ready"`
		Version string `json:"version"`
	}
	_ = json.Unmarshal(body, &h)
	return true, h.Ready, h.Version, ""
}

// statusEndpoint builds the /health endpoint string for the configured
// service, independent of client-side env overrides.
func statusEndpoint(host string, port int) string {
	return fmt.Sprintf("http://%s:%d/health", host, port)
}

// resolveStatusConfig loads the recorded config for status output without
// triggering any client auto-start recovery. A missing file degrades
// gracefully to the default endpoint values.
func resolveStatusConfig(configPath string) (host string, port int) {
	path, err := resolveInstalledConfig(configPath)
	if err != nil {
		return "localhost", resolvePort()
	}
	cfg, err := config.Load(path)
	if err != nil {
		return "localhost", resolvePort()
	}
	return cfg.Server.Host, cfg.Server.Port
}

// runServiceStatusCmd implements `nano-brain service status` with the
// full status contract and exit codes 0-3.
func runServiceStatusCmd(platform servicePlatform, opts serviceOptions, configPath string) {
	st := serviceStatus{Platform: platform.name()}
	host, port := resolveStatusConfig(configPath)
	st.Endpoint = statusEndpoint(host, port)

	if registered(platform) {
		st.Registered = true
		state, err := platform.supervisorState(context.Background())
		if err != nil {
			st.SupervisorState = "unknown"
			st.Error = err.Error()
		} else {
			st.SupervisorState = state
		}
	}
	var probeErr string
	st.HealthReachable, st.Ready, st.Version, probeErr = probeServiceHealth(host, port)
	if probeErr != "" {
		st.Error = probeErr
	}

	emitServiceStatus(st, opts.jsonOutput)
	os.Exit(statusExitCode(st))
}

// statusExitCode maps a status object to the managed-daemon-service exit
// code contract: 2 = not registered; 0 = registered + active + reachable +
// ready; otherwise 1.
func statusExitCode(st serviceStatus) int {
	switch {
	case !st.Registered:
		return serviceExitNotRegistered
	case st.SupervisorState == "active" && st.HealthReachable && st.Ready:
		return serviceExitOK
	default:
		return serviceExitDegraded
	}
}

// emitServiceStatus prints the status object in JSON or human form.
func emitServiceStatus(st serviceStatus, jsonOut bool) {
	if jsonOut {
		data, _ := json.MarshalIndent(st, "", "  ")
		fmt.Println(string(data))
		return
	}
	fmt.Printf("Platform:           %s\n", st.Platform)
	fmt.Printf("Registered:         %t\n", st.Registered)
	fmt.Printf("Supervisor state:   %s\n", st.SupervisorState)
	fmt.Printf("Health reachable:   %t\n", st.HealthReachable)
	fmt.Printf("Ready:              %t\n", st.Ready)
	if st.Endpoint != "" {
		fmt.Printf("Endpoint:           %s\n", st.Endpoint)
	}
	if st.Version != "" {
		fmt.Printf("Version:            %s\n", st.Version)
	}
	if st.Error != "" {
		fmt.Printf("Error:              %s\n", st.Error)
	}
}

// runServiceLifecycleCmd implements install/uninstall/restart/update with
// the common state-transition and rollback rules.
func runServiceLifecycleCmd(platform servicePlatform, op serviceOperation, opts serviceOptions, configPath string) {
	ctx := context.Background()

	switch op {
	case serviceOpInstall, serviceOpUpdate:
		spec, err := buildServiceSpec(platform, configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(serviceExitDegraded)
		}
		if err := installDefinition(ctx, platform, spec); err != nil {
			fmt.Fprintf(os.Stderr, "Error: nano-brain service %s failed: %s\n", op, err)
			os.Exit(serviceExitDegraded)
		}
		if opts.jsonOutput {
			fmt.Printf("{\"operation\":\"%s\",\"platform\":\"%s\",\"registered\":true}\n", op, platform.name())
		} else {
			fmt.Printf("nano-brain service %s complete — definition at %s (see 'nano-brain service status')\n", op, platform.definitionPath())
		}
	case serviceOpUninstall:
		if !registered(platform) {
			if opts.jsonOutput {
				fmt.Println(`{"operation":"uninstall","registered":false}`)
			} else {
				fmt.Println("nano-brain service is not registered; nothing to uninstall")
			}
			os.Exit(serviceExitOK)
		}
		if err := platform.unregister(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: nano-brain service uninstall failed: %s\n", err)
			os.Exit(serviceExitDegraded)
		}
		if err := os.Remove(platform.definitionPath()); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: definition removal failed: %s\n", err)
			os.Exit(serviceExitDegraded)
		}
		if opts.jsonOutput {
			fmt.Println(`{"operation":"uninstall","registered":false}`)
		} else {
			fmt.Println("nano-brain service uninstalled")
		}
	case serviceOpRestart:
		if !registered(platform) {
			fmt.Fprintln(os.Stderr, "Error: nano-brain service is not registered — run 'nano-brain service install' first")
			os.Exit(serviceExitNotRegistered)
		}
		if err := platform.restartService(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: nano-brain service restart failed: %s\n", err)
			os.Exit(serviceExitDegraded)
		}
		if opts.jsonOutput {
			fmt.Println(`{"operation":"restart","registered":true}`)
		} else {
			fmt.Println("nano-brain service restarted")
		}
	}
}

// installDefinition writes the rendered definition atomically and registers
// it with the native manager. On a manager failure it restores the previous
// definition (or removes the new file when none existed) so the CLI never
// claims success over a half-installed service.
func installDefinition(ctx context.Context, platform servicePlatform, spec serviceSpec) error {
	path := platform.definitionPath()
	prev, prevErr := os.ReadFile(path) // nil when absent; prevErr non-nil
	data, err := platform.renderDefinition(spec)
	if err != nil {
		return fmt.Errorf("render definition: %w", err)
	}
	if err := writeFileAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("write definition: %w", err)
	}
	if err := platform.register(ctx); err != nil {
		if prevErr == nil {
			_ = writeFileAtomic(path, prev, 0o644)
		} else {
			_ = os.Remove(path)
		}
		return err
	}
	return nil
}
