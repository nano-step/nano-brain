package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nano-brain/nano-brain/internal/storage"
)

type workspaceRemoveFlags struct {
	workspace     string
	workspacePath string
	force         bool
	dryRun        bool
	jsonFlag      bool
}

func parseWorkspaceRemoveFlags(args []string) (workspaceRemoveFlags, string) {
	var f workspaceRemoveFlags
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			return f, "help"
		case arg == "--force":
			f.force = true
		case arg == "--dry-run":
			f.dryRun = true
		case arg == "--json":
			f.jsonFlag = true
		case strings.HasPrefix(arg, "--workspace="):
			f.workspace = strings.TrimPrefix(arg, "--workspace=")
		case arg == "--workspace":
			if i+1 >= len(args) {
				return f, "--workspace requires a value"
			}
			i++
			f.workspace = args[i]
		case strings.HasPrefix(arg, "--workspace-path="):
			f.workspacePath = strings.TrimPrefix(arg, "--workspace-path=")
		case arg == "--workspace-path":
			if i+1 >= len(args) {
				return f, "--workspace-path requires a value"
			}
			i++
			f.workspacePath = args[i]
		default:
			return f, "unknown flag: " + arg
		}
	}
	return f, ""
}

func runWorkspacesRemove(args []string) {
	exit := runWorkspacesRemoveWithIO(args, os.Stdout, os.Stderr)
	if exit != 0 {
		os.Exit(exit)
	}
}

func runWorkspacesRemoveWithIO(args []string, stdout, stderr io.Writer) int {
	cliLog.Info().Str("cmd", "workspaces remove").Msg("cli command started")

	f, errMsg := parseWorkspaceRemoveFlags(args)
	if errMsg == "help" {
		printWorkspaceRemoveUsage(stdout)
		return 0
	}
	if errMsg != "" {
		fmt.Fprintln(stderr, errMsg)
		printWorkspaceRemoveUsage(stderr)
		return 2
	}

	if f.workspace == "" && f.workspacePath == "" {
		fmt.Fprintln(stderr, "must specify --workspace=<hash> or --workspace-path=<path>")
		printWorkspaceRemoveUsage(stderr)
		return 2
	}

	hash := f.workspace
	if f.workspacePath != "" {
		h, err := storage.WorkspaceHash(f.workspacePath)
		if err != nil {
			fmt.Fprintf(stderr, "Error: could not derive workspace hash from path %q: %v\n", f.workspacePath, err)
			return 1
		}
		hash = h
	}

	if f.dryRun {
		return workspaceRemoveDryRun(hash, f.jsonFlag, stdout, stderr)
	}

	if !f.force {
		docCount, ok := fetchDocCount(hash, stderr)
		if !ok {
			return 1
		}
		fmt.Fprintf(stderr, "Refusing to remove workspace %s (%d document(s)). Use --force to confirm deletion.\n", hash, docCount)
		return 2
	}

	return workspaceRemoveExecute(hash, f.jsonFlag, stdout, stderr)
}

func fetchDocCount(hash string, stderr io.Writer) (int64, bool) {
	resp, statusCode, err := doRequest("GET", getBaseURL()+"/api/v1/workspaces", nil)
	if err != nil {
		fmt.Fprintf(stderr, "Error fetching workspace info: %v\n", err)
		return 0, false
	}
	if statusCode != 200 {
		fmt.Fprintf(stderr, "Error: server returned status %d\n", statusCode)
		return 0, false
	}
	var items []struct {
		WorkspaceHash string `json:"workspace_hash"`
		DocumentCount int64  `json:"document_count"`
	}
	if err := json.Unmarshal(resp, &items); err != nil {
		return 0, true
	}
	for _, it := range items {
		if it.WorkspaceHash == hash {
			return it.DocumentCount, true
		}
	}
	return 0, true
}

func workspaceRemoveDryRun(hash string, jsonFlag bool, stdout, stderr io.Writer) int {
	docCount, ok := fetchDocCount(hash, stderr)
	if !ok {
		return 1
	}
	if jsonFlag {
		out, _ := json.Marshal(map[string]interface{}{
			"workspace":    hash,
			"deleted_docs": docCount,
			"dry_run":      true,
		})
		fmt.Fprintln(stdout, string(out))
		return 0
	}
	fmt.Fprintf(stdout, "Dry run: workspace %s would be removed (%d document(s) deleted).\n", hash, docCount)
	fmt.Fprintln(stdout, "No changes written.")
	cliLog.Info().Str("cmd", "workspaces remove").Str("workspace", hash).Int64("doc_count", docCount).Bool("dry_run", true).Msg("cli command completed")
	return 0
}

func workspaceRemoveExecute(hash string, jsonFlag bool, stdout, stderr io.Writer) int {
	resp, statusCode, err := doRequest("DELETE", getBaseURL()+"/api/v1/workspaces/"+hash, nil)
	if err != nil {
		if statusCode == 404 {
			fmt.Fprintf(stderr, "Error: workspace %s not found\n", hash)
			return 1
		}
		fmt.Fprintf(stderr, "Error: %v\n", err)
		cliLog.Error().Err(err).Str("cmd", "workspaces remove").Msg("remove request failed")
		return 1
	}

	if jsonFlag {
		fmt.Fprintln(stdout, string(resp))
		cliLog.Info().Str("cmd", "workspaces remove").Str("workspace", hash).Msg("cli command completed")
		return 0
	}

	var result struct {
		Workspace    string `json:"workspace"`
		DeletedDocs  int64  `json:"deleted_docs"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Fprintln(stdout, string(resp))
		return 0
	}
	fmt.Fprintf(stdout, "Workspace %s removed. %d document(s) deleted.\n", result.Workspace, result.DeletedDocs)
	cliLog.Info().Str("cmd", "workspaces remove").Str("workspace", hash).Int64("deleted_docs", result.DeletedDocs).Msg("cli command completed")
	return 0
}

func printWorkspaceRemoveUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: nano-brain workspaces remove --workspace=<hash> [flags]
       nano-brain workspaces remove --workspace-path=<path> [flags]

Permanently remove a workspace and all its documents, chunks, and embeddings.

Flags:
  --workspace=<hash>     Workspace hash to remove
  --workspace-path=<p>   Derive workspace hash from directory path
  --force                Confirm deletion (required without --dry-run)
  --dry-run              Show what would be deleted; make no changes
  --json                 Output raw JSON response
  -h, --help             Show this help
`)
}
