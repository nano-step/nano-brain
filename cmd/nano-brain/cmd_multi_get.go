package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nano-brain/nano-brain/internal/storage"
)

func runMultiGetCmd(args []string) {
	cliLog.Info().Str("cmd", "multi-get").Msg("cli command started")

	var workspaceHash, workspacePath string
	var rawPaths, rawIDs string
	var jsonFlag bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			printMultiGetUsage()
			return
		case strings.HasPrefix(arg, "--workspace="):
			workspaceHash = strings.TrimPrefix(arg, "--workspace=")
		case arg == "--workspace":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--workspace requires a value\n")
				os.Exit(2)
			}
			i++
			workspaceHash = args[i]
		case strings.HasPrefix(arg, "--workspace-path="):
			workspacePath = strings.TrimPrefix(arg, "--workspace-path=")
		case arg == "--workspace-path":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--workspace-path requires a value\n")
				os.Exit(2)
			}
			i++
			workspacePath = args[i]
		case strings.HasPrefix(arg, "--paths="):
			rawPaths = strings.TrimPrefix(arg, "--paths=")
		case arg == "--paths":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--paths requires a value\n")
				os.Exit(2)
			}
			i++
			rawPaths = args[i]
		case strings.HasPrefix(arg, "--ids="):
			rawIDs = strings.TrimPrefix(arg, "--ids=")
		case arg == "--ids":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--ids requires a value\n")
				os.Exit(2)
			}
			i++
			rawIDs = args[i]
		case arg == "--json":
			jsonFlag = true
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n\n", arg)
			printMultiGetUsage()
			os.Exit(2)
		}
	}

	if workspaceHash == "" && workspacePath == "" {
		fmt.Fprintf(os.Stderr, "must specify --workspace=<hash> or --workspace-path=<path>\n\n")
		printMultiGetUsage()
		os.Exit(2)
	}
	if rawPaths == "" && rawIDs == "" {
		fmt.Fprintf(os.Stderr, "must specify --paths=<p1,p2,...> or --ids=<id1,id2,...>\n\n")
		printMultiGetUsage()
		os.Exit(2)
	}

	if workspacePath != "" {
		h, err := storage.WorkspaceHash(workspacePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not derive workspace hash from path %q: %v\n", workspacePath, err)
			os.Exit(1)
		}
		workspaceHash = h
	}

	reqBody := map[string]interface{}{
		"workspace": workspaceHash,
	}
	if rawPaths != "" {
		reqBody["paths"] = splitCSV(rawPaths)
	}
	if rawIDs != "" {
		reqBody["ids"] = splitCSV(rawIDs)
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	resp, _, reqErr := doRequest("POST", getBaseURL()+"/api/v1/multi-get", bytes.NewReader(data))
	if reqErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", reqErr)
		cliLog.Error().Err(reqErr).Str("cmd", "multi-get").Msg("request failed")
		os.Exit(1)
	}

	if jsonFlag {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "multi-get").Msg("cli command completed")
		return
	}

	var result multiGetResult
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "multi-get").Msg("cli command completed")
		return
	}

	printMultiGetResult(result)
	cliLog.Info().Str("cmd", "multi-get").
		Int("found", len(result.Results)).
		Int("not_found", len(result.NotFound)).
		Msg("cli command completed")
}

type multiGetResult struct {
	Results  []getDocResult `json:"results"`
	NotFound []string       `json:"not_found"`
}

func printMultiGetResult(r multiGetResult) {
	for i, doc := range r.Results {
		if i > 0 {
			fmt.Println("---")
		}
		printGetResult(doc)
	}
	if len(r.NotFound) > 0 {
		fmt.Printf("\nNot found (%d): %s\n", len(r.NotFound), strings.Join(r.NotFound, ", "))
	}
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func printMultiGetUsage() {
	fmt.Print(`Usage: nano-brain multi-get --workspace=<hash> --paths=<p1,p2,...> [flags]
       nano-brain multi-get --workspace=<hash> --ids=<id1,id2,...>   [flags]

Fetch multiple documents in one round-trip.

Flags:
  --workspace=<hash>     Workspace hash (required if --workspace-path not given)
  --workspace-path=<p>   Derive workspace hash from directory path
  --paths=<p1,p2,...>    Comma-separated source_paths to fetch
  --ids=<id1,id2,...>    Comma-separated UUIDs to fetch
  --json                 Output raw JSON response
  -h, --help             Show this help
`)
}
