package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

func runContextCmd(args []string) {
	cliLog.Info().Str("cmd", "context").Msg("cli command started")

	var symbol, workspace string
	var jsonFlag bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--workspace requires a value\n")
				os.Exit(1)
			}
			i++
			workspace = args[i]
		case "--json":
			jsonFlag = true
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
		fmt.Fprintln(os.Stderr, "Usage: nano-brain context <symbol> --workspace <hash> [--json]")
		os.Exit(1)
	}

	base := getBaseURL()

	symURL := fmt.Sprintf("%s/api/v1/symbols?query=%s&workspace=%s&limit=1",
		base, url.QueryEscape(symbol), url.QueryEscape(workspace))
	symResp, _, err := doRequest("GET", symURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching symbols: %v\n", err)
		os.Exit(1)
	}

	var symResult struct {
		Symbols []struct {
			Name       string `json:"name"`
			Kind       string `json:"kind"`
			Language   string `json:"language"`
			SourcePath string `json:"source_path"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(symResp, &symResult); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing symbol response: %v\n", err)
		os.Exit(1)
	}

	if len(symResult.Symbols) == 0 {
		fmt.Fprintf(os.Stderr, "No symbol found for %q\n", symbol)
		os.Exit(1)
	}
	sym := symResult.Symbols[0]
	node := sym.SourcePath + "::" + sym.Name

	outEdges := queryGraphEdges(base, workspace, node, "out")
	inEdges := queryGraphEdges(base, workspace, node, "in")

	if jsonFlag {
		output := map[string]any{
			"symbol":    sym.Name,
			"file":      sym.SourcePath,
			"kind":      sym.Kind,
			"language":  sym.Language,
			"calls_out": outEdges,
			"called_by": inEdges,
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
		cliLog.Info().Str("cmd", "context").Msg("cli command completed")
		return
	}

	fmt.Printf("Symbol: %s\n", sym.Name)
	fmt.Printf("File:   %s\n", sym.SourcePath)
	fmt.Printf("Kind:   %s\n", sym.Kind)
	fmt.Printf("Lang:   %s\n", sym.Language)
	fmt.Println()

	if len(outEdges) > 0 {
		fmt.Printf("Calls out (%d):\n", len(outEdges))
		for _, e := range outEdges {
			fmt.Printf("  -> %s\n", e.Target)
		}
	} else {
		fmt.Println("Calls out: (none)")
	}
	fmt.Println()

	if len(inEdges) > 0 {
		fmt.Printf("Called by (%d):\n", len(inEdges))
		for _, e := range inEdges {
			fmt.Printf("  <- %s\n", e.Source)
		}
	} else {
		fmt.Println("Called by: (none)")
	}

	cliLog.Info().Str("cmd", "context").Msg("cli command completed")
}

type graphEdge struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	EdgeType string `json:"edge_type"`
}

func queryGraphEdges(base, workspace, node, direction string) []graphEdge {
	body := map[string]string{
		"workspace": workspace,
		"node":      node,
		"direction": direction,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil
	}
	resp, _, err := doRequest("POST", base+"/api/v1/graph/query", bytes.NewReader(data))
	if err != nil {
		return nil
	}
	var result struct {
		Edges []graphEdge `json:"edges"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil
	}
	return result.Edges
}
