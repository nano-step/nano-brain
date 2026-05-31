package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nano-brain/nano-brain/internal/storage"
)

func runGetCmd(args []string) {
	cliLog.Info().Str("cmd", "get").Msg("cli command started")

	var workspaceHash, workspacePath string
	var byID bool
	var jsonFlag bool
	var target string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			printGetUsage()
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
		case arg == "--by-id":
			byID = true
		case arg == "--json":
			jsonFlag = true
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n\n", arg)
			printGetUsage()
			os.Exit(2)
		default:
			if target == "" {
				target = arg
			} else {
				fmt.Fprintf(os.Stderr, "unexpected argument: %s\n\n", arg)
				printGetUsage()
				os.Exit(2)
			}
		}
	}

	if target == "" {
		fmt.Fprintf(os.Stderr, "source_path or id argument is required\n\n")
		printGetUsage()
		os.Exit(2)
	}
	if workspaceHash == "" && workspacePath == "" {
		fmt.Fprintf(os.Stderr, "must specify --workspace=<hash> or --workspace-path=<path>\n\n")
		printGetUsage()
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
	if byID {
		reqBody["id"] = target
	} else {
		reqBody["path"] = target
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	resp, _, reqErr := doRequest("POST", getBaseURL()+"/api/v1/get", bytes.NewReader(data))
	if reqErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", reqErr)
		cliLog.Error().Err(reqErr).Str("cmd", "get").Msg("request failed")
		os.Exit(1)
	}

	if jsonFlag {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "get").Msg("cli command completed")
		return
	}

	var result getDocResult
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "get").Msg("cli command completed")
		return
	}

	printGetResult(result)
	cliLog.Info().Str("cmd", "get").Str("id", result.ID).Msg("cli command completed")
}

type getDocResult struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Content       string   `json:"content"`
	SourcePath    string   `json:"source_path"`
	Collection    string   `json:"collection"`
	Tags          []string `json:"tags"`
	WorkspaceHash string   `json:"workspace_hash"`
	SupersedesID  string   `json:"supersedes_id,omitempty"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

func printGetResult(r getDocResult) {
	fmt.Printf("ID:         %s\n", r.ID)
	if r.Title != "" {
		fmt.Printf("Title:      %s\n", r.Title)
	}
	fmt.Printf("Path:       %s\n", r.SourcePath)
	fmt.Printf("Collection: %s\n", r.Collection)
	if len(r.Tags) > 0 {
		fmt.Printf("Tags:       %s\n", strings.Join(r.Tags, ", "))
	}
	fmt.Printf("Updated:    %s\n", r.UpdatedAt)
	fmt.Println()
	fmt.Println(r.Content)
}

func printGetUsage() {
	fmt.Print(`Usage: nano-brain get <source_path> --workspace=<hash> [flags]
       nano-brain get <uuid>       --workspace=<hash> --by-id [flags]

Fetch a single document by source_path or ID.

Arguments:
  <source_path>          Document source_path (e.g., memory://decision.md)
  <uuid>                 Document UUID (requires --by-id)

Flags:
  --workspace=<hash>     Workspace hash (required if --workspace-path not given)
  --workspace-path=<p>   Derive workspace hash from directory path
  --by-id                Treat the positional argument as a document UUID
  --json                 Output raw JSON response
  -h, --help             Show this help
`)
}
