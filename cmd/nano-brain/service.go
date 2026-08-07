package main

import (
	"fmt"
	"os"
	"strings"
)

// Service exit codes. The managed-daemon-service status contract defines:
// 0 = registered + active + reachable + ready; 1 = registered but degraded,
// not ready, or a lifecycle command failure; 2 = definition not registered;
// 3 = unsupported platform or unavailable user manager.
const (
	serviceExitOK            = 0
	serviceExitDegraded      = 1
	serviceExitNotRegistered = 2
	serviceExitUnsupported   = 3
)

// serviceOperation enumerates the `nano-brain service` subcommands.
type serviceOperation string

const (
	serviceOpInstall   serviceOperation = "install"
	serviceOpUninstall serviceOperation = "uninstall"
	serviceOpStatus    serviceOperation = "status"
	serviceOpRestart   serviceOperation = "restart"
	serviceOpUpdate    serviceOperation = "update"
)

// serviceOptions holds parsed `nano-brain service` flags.
type serviceOptions struct {
	jsonOutput bool
}

// serviceStatus is the single status object returned by `service status`
// and printed by lifecycle commands on failure. All fields stay present with
// zero values in partial states.
type serviceStatus struct {
	Platform        string `json:"platform"`
	Registered      bool   `json:"registered"`
	SupervisorState string `json:"supervisor_state"`
	HealthReachable bool   `json:"health_reachable"`
	Ready           bool   `json:"ready"`
	Endpoint        string `json:"endpoint"`
	Version         string `json:"version"`
	Error           string `json:"error"`
}

// serviceUsage is the CLI usage line for `nano-brain service`.
func serviceUsage() string {
	return "Usage: nano-brain service <install|uninstall|status|restart|update> [--json]"
}

// parseServiceArgs validates the subcommand and flags without touching any
// platform manager, so tests can exercise parsing in isolation.
func parseServiceArgs(args []string) (serviceOperation, serviceOptions, error) {
	if len(args) == 0 {
		return "", serviceOptions{}, fmt.Errorf("missing service subcommand")
	}
	op := serviceOperation(args[0])
	switch op {
	case serviceOpInstall, serviceOpUninstall, serviceOpStatus, serviceOpRestart, serviceOpUpdate:
	default:
		return "", serviceOptions{}, fmt.Errorf("unknown service subcommand %q", args[0])
	}
	opts := serviceOptions{}
	for _, a := range args[1:] {
		switch a {
		case "--json":
			opts.jsonOutput = true
		default:
			if strings.HasPrefix(a, "--") {
				return "", serviceOptions{}, fmt.Errorf("unknown flag: %s", a)
			}
			return "", serviceOptions{}, fmt.Errorf("unexpected argument: %s", a)
		}
	}
	return op, opts, nil
}

// runServiceCmd is the `nano-brain service` entry point. It parses the
// subcommand, guards the platform, and routes to the common lifecycle or
// status implementation.
func runServiceCmd(args []string, configPath string) {
	cliLog.Info().Str("cmd", "service").Msg("cli command started")
	op, opts, err := parseServiceArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, serviceUsage())
		os.Exit(serviceExitDegraded)
	}

	platform := newServicePlatform()
	if err := platform.usable(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(serviceExitUnsupported)
	}

	switch op {
	case serviceOpStatus:
		runServiceStatusCmd(platform, opts, configPath)
	default:
		runServiceLifecycleCmd(platform, op, opts, configPath)
	}
}
