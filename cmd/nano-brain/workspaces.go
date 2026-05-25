package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

func workspacesUsage() {
	fmt.Fprintln(os.Stderr, "Usage: nano-brain workspaces [list|ls] [--json]")
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

	var items []map[string]interface{}
	if err := json.Unmarshal(body, &items); err != nil {
		fmt.Fprintf(stderr, "failed to parse server response: %v\n", err)
		return 1
	}

	if len(items) == 0 {
		fmt.Fprintln(stderr, "No workspaces registered.")
		return 0
	}

	renderWorkspacesTable(items, stdout)
	return 0
}

func renderWorkspacesTable(items []map[string]interface{}, w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "HASH\tNAME\tPATH\tDOCS\tLAST UPDATE")
	for _, it := range items {
		hash := stringField(it, "workspace_hash")
		name := stringField(it, "name")
		path := stringField(it, "root_path")
		docs := intField(it, "document_count")
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
