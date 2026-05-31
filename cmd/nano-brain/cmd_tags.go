package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/nano-brain/nano-brain/internal/storage"
)

func runTagsCmd(args []string) {
	cliLog.Info().Str("cmd", "tags").Msg("cli command started")

	var workspaceHash, workspacePath string
	var jsonFlag bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			printTagsUsage()
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
		case arg == "--json":
			jsonFlag = true
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n\n", arg)
			printTagsUsage()
			os.Exit(2)
		}
	}

	if workspaceHash == "" && workspacePath == "" {
		fmt.Fprintf(os.Stderr, "must specify --workspace=<hash> or --workspace-path=<path>\n\n")
		printTagsUsage()
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

	u := getBaseURL() + "/api/v1/tags?workspace=" + url.QueryEscape(workspaceHash)
	resp, _, reqErr := doRequest("GET", u, nil)
	if reqErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", reqErr)
		cliLog.Error().Err(reqErr).Str("cmd", "tags").Msg("request failed")
		os.Exit(1)
	}

	if jsonFlag {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "tags").Msg("cli command completed")
		return
	}

	var items []tagItem
	if err := json.Unmarshal(resp, &items); err != nil {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "tags").Msg("cli command completed")
		return
	}

	if len(items) == 0 {
		fmt.Println("(no tags)")
		cliLog.Info().Str("cmd", "tags").Msg("cli command completed")
		return
	}

	for _, t := range items {
		fmt.Printf("%-30s  %d\n", t.Tag, t.Count)
	}
	cliLog.Info().Str("cmd", "tags").Int("count", len(items)).Msg("cli command completed")
}

type tagItem struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}

func printTagsUsage() {
	fmt.Print(`Usage: nano-brain tags --workspace=<hash> [flags]

List all tags in the workspace with their document counts.

Flags:
  --workspace=<hash>     Workspace hash (required if --workspace-path not given)
  --workspace-path=<p>   Derive workspace hash from directory path
  --json                 Output raw JSON response
  -h, --help             Show this help
`)
}
