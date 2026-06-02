package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"
)

func workspacesUsage() {
	fmt.Fprintln(os.Stderr, "Usage: nano-brain workspaces [list|ls|current|remove] [flags]")
	os.Exit(1)
}

func runWorkspacesCmd(args []string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		runWorkspacesList(args)
		return
	}
	switch args[0] {
	case "list", "ls":
		runWorkspacesList(args[1:])
	case "current":
		runWorkspacesCurrent(args[1:])
	case "remove", "rm":
		runWorkspacesRemove(args[1:])
	default:
		workspacesUsage()
	}
}

func runWorkspacesList(args []string) {
	exit := runWorkspacesListWithIO(args, os.Stdout, os.Stderr)
	if exit != 0 {
		os.Exit(exit)
	}
}

func runWorkspacesListWithIO(args []string, stdout, stderr io.Writer) int {
	jsonFlag := false
	for _, a := range args {
		if a == "--json" {
			jsonFlag = true
		}
	}

	body, _, err := doRequest("GET", getBaseURL()+"/api/v1/workspaces", nil)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if jsonFlag {
		trimmed := bytes.TrimRight(body, "\n")
		_, _ = stdout.Write(trimmed)
		fmt.Fprintln(stdout)
		return 0
	}

	var resp struct {
		Workspaces []map[string]interface{} `json:"workspaces"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		fmt.Fprintf(stderr, "failed to parse server response: %v\n", err)
		return 1
	}

	if len(resp.Workspaces) == 0 {
		fmt.Fprintln(stderr, "No workspaces registered.")
		return 0
	}

	renderWorkspacesTable(resp.Workspaces, stdout)
	return 0
}

func renderWorkspacesTable(items []map[string]interface{}, w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HASH\tNAME\tPATH\tDOCS\tLAST UPDATE")
	for _, it := range items {
		hash := stringField(it, "hash")
		name := stringField(it, "name")
		path := stringField(it, "root_path")
		docs := intField(it, "doc_count")
		last := lastUpdateField(it["last_document_updated"])

		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n",
			truncateHash(hash),
			truncateName(name, 30),
			truncateLeft(path, 50),
			docs,
			last,
		)
	}
	_ = tw.Flush()
}

func stringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func intField(m map[string]interface{}, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	}
	return 0
}

func lastUpdateField(v interface{}) string {
	s, ok := v.(string)
	if !ok || s == "" {
		return "never"
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.Format("2006-01-02")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Format("2006-01-02")
	}
	return "never"
}

func truncateHash(s string) string {
	if len(s) <= 10 {
		return s
	}
	return s[:10] + ".."
}

func truncateName(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-2] + ".."
}

func truncateLeft(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return ".." + s[len(s)-(max-2):]
}

func runWorkspacesCurrent(args []string) {
	exit := runWorkspacesCurrentWithIO(args, os.Stdout, os.Stderr)
	if exit != 0 {
		os.Exit(exit)
	}
}

func runWorkspacesCurrentWithIO(args []string, stdout, stderr io.Writer) int {
	var (
		pathFlag   string
		exportFlag bool
		jsonFlag   bool
		checkFlag  bool
	)
	for _, a := range args {
		switch {
		case a == "--export":
			exportFlag = true
		case a == "--json":
			jsonFlag = true
		case a == "--check":
			checkFlag = true
		case strings.HasPrefix(a, "--path="):
			pathFlag = strings.TrimPrefix(a, "--path=")
		default:
			fmt.Fprintf(stderr, "unknown flag: %s\n", a)
			return 1
		}
	}

	path := pathFlag
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "failed to detect current directory: %v\n", err)
			return 1
		}
		path = cwd
	} else if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			fmt.Fprintf(stderr, "failed to resolve path %q: %v\n", path, err)
			return 1
		}
		path = abs
	}

	reqBody, _ := json.Marshal(map[string]string{"path": path})
	body, _, err := doRequest("POST", getBaseURL()+"/api/v1/workspaces/resolve", bytes.NewReader(reqBody))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if jsonFlag {
		trimmed := bytes.TrimRight(body, "\n")
		_, _ = stdout.Write(trimmed)
		fmt.Fprintln(stdout)
	}

	var resp struct {
		WorkspaceHash string `json:"workspace_hash"`
		RootPath      string `json:"root_path"`
		Name          string `json:"name"`
		Registered    bool   `json:"registered"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		fmt.Fprintf(stderr, "failed to parse server response: %v\n", err)
		return 1
	}

	if !jsonFlag {
		switch {
		case exportFlag:
			fmt.Fprintf(stdout, "export NANO_BRAIN_WORKSPACE=%s\n", resp.WorkspaceHash)
		default:
			fmt.Fprintln(stdout, resp.WorkspaceHash)
		}
	}

	if checkFlag && !resp.Registered {
		fmt.Fprintf(stderr, "workspace not registered: %s\n", resp.RootPath)
		return 2
	}
	return 0
}
