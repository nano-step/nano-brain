package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/health/doctor"
)

func runDoctorCmd(args []string, configPath string) {
	cliLog.Debug().Str("cmd", "doctor").Msg("cli command started")
	var jsonFlag bool
	var onlineFlag bool
	for _, a := range args {
		switch a {
		case "--json":
			jsonFlag = true
		case "--online":
			onlineFlag = true
		}
	}

	if configPath == "" {
		configPath = config.ResolveConfigPath("")
	}

	cfg, cfgErr := config.Load(configPath)
	results := doctor.RunAll(configPath, cfg, cfgErr, resolveBinaryPath())

	if onlineFlag {
		baseURL := getBaseURL()
		serverCheck, status := doctor.CheckServerRunning(baseURL)
		results = append(results, serverCheck)
		if status == nil {
			results = append(results, doctor.Check{Name: "Queue health", Status: "skip", Detail: "server unreachable"})
			results = append(results, doctor.Check{Name: "Version skew", Status: "skip", Detail: "server unreachable"})
			results = append(results, doctor.Check{Name: "MCP reachable", Status: "skip", Detail: "server unreachable"})
		} else {
			results = append(results, doctor.CheckQueueHealth(*status))
			results = append(results, doctor.CheckVersionSkew(Version, status.Version))
			mcpURL := resolveMCPURL("/.dockerenv")
			results = append(results, doctor.CheckMCPReachable(mcpURL))
		}
	}

	allPassed := true
	for _, r := range results {
		if r.Status == "fail" {
			allPassed = false
			break
		}
	}

	if jsonFlag {
		out := struct {
			Checks    []doctor.Check `json:"checks"`
			AllPassed bool           `json:"all_passed"`
		}{results, allPassed}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	} else {
		fmt.Print("\nnano-brain doctor\n\n")
		for _, r := range results {
			label := padRight(r.Name, 22)
			detail := padRight(r.Detail, 28)
			status := "OK"
			if r.Status == "fail" {
				status = "FAIL"
			} else if r.Status == "skip" {
				status = "SKIP"
			} else if r.Status == "warn" {
				status = "WARN"
			}
			fmt.Printf("  %s %s %s\n", label, detail, status)
			if (r.Status == "fail" || r.Status == "warn") && r.Hint != "" {
				for _, line := range strings.Split(r.Hint, "\n") {
					fmt.Printf("    → %s\n", line)
				}
			}
		}
		fmt.Println()
		if allPassed {
			fmt.Println("All checks passed.")
		} else {
			fmt.Println("Some checks failed.")
		}
		fmt.Println()
	}

	cliLog.Debug().Str("cmd", "doctor").Bool("all_passed", allPassed).Int("checks", len(results)).Msg("cli command completed")

	if !allPassed {
		os.Exit(1)
	}
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(".", n-len(s))
}
