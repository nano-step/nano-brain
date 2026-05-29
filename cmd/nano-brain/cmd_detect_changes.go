package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func runDetectChangesCmd(args []string) {
	cliLog.Info().Str("cmd", "detect-changes").Msg("cli command started")

	var workspace string
	var jsonFlag, allFlag bool
	staged := true
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--workspace requires a value\n")
				os.Exit(1)
			}
			i++
			workspace = args[i]
		case "--staged":
			staged = true
			allFlag = false
		case "--all":
			allFlag = true
			staged = false
		case "--json":
			jsonFlag = true
		default:
			if strings.HasPrefix(args[i], "--") {
				fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "unexpected argument: %s\n", args[i])
			os.Exit(1)
		}
	}
	if workspace == "" {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain detect-changes --workspace <hash> [--staged|--all] [--json]")
		os.Exit(1)
	}

	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintln(os.Stderr, "Error: git is not installed or not in PATH")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	diffArgs := []string{"diff", "--name-only"}
	if staged && !allFlag {
		diffArgs = append(diffArgs, "--staged")
	}
	out, err := exec.CommandContext(ctx, "git", diffArgs...).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running git diff: %v\n", err)
		os.Exit(1)
	}

	files := parseLines(string(out))
	if len(files) == 0 {
		if jsonFlag {
			fmt.Println(`{"files":[],"symbols":[],"impacted":[]}`)
		} else {
			fmt.Println("No changed files detected.")
		}
		cliLog.Info().Str("cmd", "detect-changes").Msg("cli command completed")
		return
	}

	type changedSymbol struct {
		File   string `json:"file"`
		Symbol string `json:"symbol"`
		Line   int    `json:"line"`
	}

	type impactedNode struct {
		Node     string `json:"node"`
		Depth    int    `json:"depth"`
		EdgeType string `json:"edge_type"`
		Via      string `json:"via"`
	}

	var symbols []changedSymbol
	var impacted []impactedNode
	base := getBaseURL()

	for _, file := range files {
		symResp := queryFileSymbols(base, workspace, file)
		for _, sym := range symResp {
			if sym.SourcePath != file {
				continue
			}
			symbols = append(symbols, changedSymbol{
				File:   file,
				Symbol: sym.Name,
			})

			node := file + "::" + sym.Name
			impactBody := map[string]any{
				"workspace": workspace,
				"node":      node,
				"max_depth": 1,
			}
			data, _ := json.Marshal(impactBody)
			resp, _, err := doRequest("POST", base+"/api/v1/graph/impact", bytes.NewReader(data))
			if err != nil {
				continue
			}
			var impResult struct {
				Impacted []struct {
					Node     string `json:"node"`
					Depth    int    `json:"depth"`
					EdgeType string `json:"edge_type"`
				} `json:"impacted"`
			}
			if json.Unmarshal(resp, &impResult) == nil {
				for _, imp := range impResult.Impacted {
					impacted = append(impacted, impactedNode{
						Node:     imp.Node,
						Depth:    imp.Depth,
						EdgeType: imp.EdgeType,
						Via:      sym.Name,
					})
				}
			}
		}
	}

	if jsonFlag {
		output := map[string]any{
			"files":    files,
			"symbols":  symbols,
			"impacted": impacted,
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
		cliLog.Info().Str("cmd", "detect-changes").Msg("cli command completed")
		return
	}

	fmt.Printf("Changed files (%d):\n", len(files))
	for _, f := range files {
		fmt.Printf("  %s\n", f)
	}
	fmt.Println()

	if len(symbols) > 0 {
		fmt.Printf("Changed symbols (%d):\n", len(symbols))
		for _, s := range symbols {
			fmt.Printf("  %s :: %s\n", s.File, s.Symbol)
		}
	} else {
		fmt.Println("Changed symbols: (none found in index)")
	}
	fmt.Println()

	if len(impacted) > 0 {
		fmt.Printf("Impacted nodes (%d):\n", len(impacted))
		for _, n := range impacted {
			fmt.Printf("  %s (via %s, %s)\n", n.Node, n.Via, n.EdgeType)
		}
	} else {
		fmt.Println("Impacted nodes: (none)")
	}

	cliLog.Info().Str("cmd", "detect-changes").Msg("cli command completed")
}

func parseLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

type fileSymbol struct {
	Name       string
	Kind       string
	Language   string
	SourcePath string
}

func queryFileSymbols(base, workspace, file string) []fileSymbol {
	u := fmt.Sprintf("%s/api/v1/symbols?workspace=%s&query=%s&limit=100",
		base, workspace, file)
	resp, _, err := doRequest("GET", u, nil)
	if err != nil {
		return nil
	}
	var result struct {
		Symbols []fileSymbol `json:"symbols"`
	}
	if json.Unmarshal(resp, &result) != nil {
		return nil
	}
	return result.Symbols
}

func getChangedLineRanges(ctx context.Context, file string, staged bool) [][2]int {
	diffArgs := []string{"diff"}
	if staged {
		diffArgs = append(diffArgs, "--staged")
	}
	diffArgs = append(diffArgs, file)
	out, err := exec.CommandContext(ctx, "git", diffArgs...).Output()
	if err != nil {
		return nil
	}
	return parseHunkHeaders(string(out))
}

func parseHunkHeaders(diff string) [][2]int {
	var ranges [][2]int
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "@@ ") {
			continue
		}
		plus := ""
		parts := strings.Split(line, " ")
		for _, p := range parts {
			if strings.HasPrefix(p, "+") && !strings.HasPrefix(p, "+++") {
				plus = p[1:]
				break
			}
		}
		if plus == "" {
			continue
		}
		comma := strings.Index(plus, ",")
		if comma < 0 {
			start, err := strconv.Atoi(plus)
			if err == nil {
				ranges = append(ranges, [2]int{start, start})
			}
			continue
		}
		start, err1 := strconv.Atoi(plus[:comma])
		count, err2 := strconv.Atoi(plus[comma+1:])
		if err1 == nil && err2 == nil && count > 0 {
			ranges = append(ranges, [2]int{start, start + count - 1})
		}
	}
	return ranges
}
