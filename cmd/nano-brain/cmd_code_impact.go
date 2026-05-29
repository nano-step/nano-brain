package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func runCodeImpactCmd(args []string) {
	cliLog.Info().Str("cmd", "code-impact").Msg("cli command started")

	var symbol, workspace string
	var jsonFlag bool
	depth := 2
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--workspace requires a value\n")
				os.Exit(1)
			}
			i++
			workspace = args[i]
		case "--depth":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--depth requires a value\n")
				os.Exit(1)
			}
			i++
			v, err := strconv.Atoi(args[i])
			if err != nil || v < 1 {
				fmt.Fprintf(os.Stderr, "--depth must be a positive integer\n")
				os.Exit(1)
			}
			depth = v
		case "--json":
			jsonFlag = true
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: nano-brain code-impact <symbol> --workspace <hash> [--depth N] [--json]")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "  --depth N    Max depth for impact traversal (default 2, clamped to [1,3] by server)")
			os.Exit(0)
		default:
			if strings.HasPrefix(args[i], "--") {
				fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
				os.Exit(1)
			}
			if symbol == "" {
				symbol = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "unexpected argument: %s\n", args[i])
				os.Exit(1)
			}
		}
	}
	if symbol == "" || workspace == "" {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain code-impact <symbol> --workspace <hash> [--depth N] [--json]")
		os.Exit(1)
	}

	body := map[string]any{
		"workspace": workspace,
		"node":      symbol,
		"max_depth": depth,
	}
	data, err := json.Marshal(body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	resp, _, err := doRequest("POST", getBaseURL()+"/api/v1/graph/impact", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if jsonFlag {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "code-impact").Msg("cli command completed")
		return
	}

	var result struct {
		Node     string `json:"node"`
		Impacted []struct {
			Node     string `json:"node"`
			Depth    int    `json:"depth"`
			EdgeType string `json:"edge_type"`
		} `json:"impacted"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "code-impact").Msg("cli command completed")
		return
	}

	fmt.Printf("Impact analysis for: %s\n", result.Node)
	if len(result.Impacted) == 0 {
		fmt.Println("  (no impacted nodes found)")
	} else {
		fmt.Printf("  %d impacted node(s):\n", len(result.Impacted))
		for _, n := range result.Impacted {
			fmt.Printf("    [depth=%d] %s (%s)\n", n.Depth, n.Node, n.EdgeType)
		}
	}

	cliLog.Info().Str("cmd", "code-impact").Msg("cli command completed")
}
