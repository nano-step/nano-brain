package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nano-brain/nano-brain/internal/storage"
)

func runWakeUpCmd(args []string) {
	cliLog.Info().Str("cmd", "wake-up").Msg("cli command started")

	var workspaceHash, workspacePath string
	var limit int
	var jsonFlag bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			printWakeUpUsage()
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
		case strings.HasPrefix(arg, "--limit="):
			v, err := strconv.Atoi(strings.TrimPrefix(arg, "--limit="))
			if err != nil || v < 1 {
				fmt.Fprintf(os.Stderr, "--limit must be a positive integer\n")
				os.Exit(2)
			}
			limit = v
		case arg == "--limit":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--limit requires a value\n")
				os.Exit(2)
			}
			i++
			v, err := strconv.Atoi(args[i])
			if err != nil || v < 1 {
				fmt.Fprintf(os.Stderr, "--limit must be a positive integer\n")
				os.Exit(2)
			}
			limit = v
		case arg == "--json":
			jsonFlag = true
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n\n", arg)
			printWakeUpUsage()
			os.Exit(2)
		}
	}

	if workspaceHash == "" && workspacePath == "" {
		fmt.Fprintf(os.Stderr, "must specify --workspace=<hash> or --workspace-path=<path>\n\n")
		printWakeUpUsage()
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
	if limit > 0 {
		reqBody["limit"] = limit
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		cliLog.Error().Err(err).Str("cmd", "wake-up").Msg("marshal failed")
		os.Exit(1)
	}

	resp, _, reqErr := doRequest("POST", getBaseURL()+"/api/v1/wake-up", bytes.NewReader(data))
	if reqErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", reqErr)
		cliLog.Error().Err(reqErr).Str("cmd", "wake-up").Msg("request failed")
		os.Exit(1)
	}

	if jsonFlag {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "wake-up").Msg("cli command completed")
		return
	}

	var result wakeUpResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "wake-up").Msg("cli command completed")
		return
	}

	printWakeUpResponse(result)
	cliLog.Info().Str("cmd", "wake-up").Msg("cli command completed")
}

type wakeUpResponse struct {
	Summary           string              `json:"summary"`
	RecentMemories    []wakeUpMemory      `json:"recent_memories"`
	ActiveCollections []wakeUpCollection  `json:"active_collections"`
	Stats             wakeUpStats         `json:"stats"`
}

type wakeUpMemory struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Snippet string   `json:"snippet"`
	Tags    []string `json:"tags"`
	Date    string   `json:"date"`
}

type wakeUpCollection struct {
	Name          string `json:"name"`
	DocumentCount int64  `json:"document_count"`
	LastUpdated   string `json:"last_updated"`
}

type wakeUpStats struct {
	TotalDocuments int64  `json:"total_documents"`
	TotalChunks    int64  `json:"total_chunks"`
	LastActivity   string `json:"last_activity"`
}

func printWakeUpResponse(r wakeUpResponse) {
	fmt.Println(r.Summary)

	if len(r.ActiveCollections) > 0 {
		fmt.Println("\nCollections:")
		for _, c := range r.ActiveCollections {
			fmt.Printf("  %-20s  %d docs", c.Name, c.DocumentCount)
			if c.LastUpdated != "" {
				fmt.Printf("  (last updated: %s)", c.LastUpdated)
			}
			fmt.Println()
		}
	}

	fmt.Printf("\nStats: %d documents, %d chunks", r.Stats.TotalDocuments, r.Stats.TotalChunks)
	if r.Stats.LastActivity != "" {
		fmt.Printf("  last activity: %s", r.Stats.LastActivity)
	}
	fmt.Println()

	if len(r.RecentMemories) > 0 {
		fmt.Printf("\nRecent memories (%d):\n", len(r.RecentMemories))
		for _, m := range r.RecentMemories {
			fmt.Printf("  [%s] %s\n", m.Date, m.Title)
			if m.Snippet != "" {
				snippet := m.Snippet
				if len(snippet) > 120 {
					snippet = snippet[:117] + "..."
				}
				fmt.Printf("    %s\n", strings.ReplaceAll(snippet, "\n", " "))
			}
			if len(m.Tags) > 0 {
				fmt.Printf("    tags: %s\n", strings.Join(m.Tags, ", "))
			}
		}
	}
}

func printWakeUpUsage() {
	fmt.Print(`Usage: nano-brain wake-up --workspace=<hash> [flags]
       nano-brain wake-up --workspace-path=<path> [flags]

Show a workspace briefing: recent memories, active collections, and stats.

Flags:
  --workspace=<hash>     Workspace hash (required if --workspace-path not given)
  --workspace-path=<p>   Derive workspace hash from directory path
  --limit=<N>            Number of recent memories to show (default: 10, max: 50)
  --json                 Output raw JSON response
  -h, --help             Show this help
`)
}
