package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

func collectionUsage() {
	fmt.Fprintln(os.Stderr, "Usage: nano-brain collection <add|remove|list|rename> [flags]")
	os.Exit(1)
}



func runCollectionCmd(args []string) {
	if len(args) == 0 {
		collectionUsage()
	}

	switch args[0] {
	case "add":
		runCollectionAdd(args[1:])
	case "remove":
		runCollectionRemove(args[1:])
	case "list":
		runCollectionList(args[1:])
	case "rename":
		runCollectionRename(args[1:])
	default:
		collectionUsage()
	}
}

func runCollectionAdd(args []string) {
	cliLog.Info().Str("cmd", "collection.add").Msg("cli command started")
	var name, path, workspace, glob string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s requires a value\n", args[i])
				os.Exit(1)
			}
			i++
			name = args[i]
		case "--path":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s requires a value\n", args[i])
				os.Exit(1)
			}
			i++
			path = args[i]
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s requires a value\n", args[i])
				os.Exit(1)
			}
			i++
			workspace = args[i]
		case "--glob":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s requires a value\n", args[i])
				os.Exit(1)
			}
			i++
			glob = args[i]
		}
	}
	if name == "" || path == "" || workspace == "" {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain collection add --name <name> --path <path> --workspace <hash> [--glob <pattern>]")
		os.Exit(1)
	}

	body := map[string]string{
		"workspace":    workspace,
		"name":         name,
		"path":         path,
		"glob_pattern": glob,
	}
	data, err := json.Marshal(body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal request: %v\n", err)
		os.Exit(1)
	}

	resp, err := http.Post(getBaseURL()+"/api/v1/collections", "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		cliLog.Error().Err(err).Str("cmd", "collection.add").Msg("request failed")
		os.Exit(1)
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
	fmt.Println()
	cliLog.Info().Str("cmd", "collection.add").Str("name", name).Str("workspace", workspace).Msg("cli command completed")
}

func runCollectionRemove(args []string) {
	cliLog.Info().Str("cmd", "collection.remove").Msg("cli command started")
	var name, workspace string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s requires a value\n", args[i])
				os.Exit(1)
			}
			i++
			name = args[i]
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s requires a value\n", args[i])
				os.Exit(1)
			}
			i++
			workspace = args[i]
		}
	}
	if name == "" || workspace == "" {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain collection remove --name <name> --workspace <hash>")
		os.Exit(1)
	}

	reqURL := fmt.Sprintf("%s/api/v1/collections/%s?workspace=%s", getBaseURL(), url.PathEscape(name), url.QueryEscape(workspace))
	req, _ := http.NewRequest(http.MethodDelete, reqURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		cliLog.Error().Err(err).Str("cmd", "collection.remove").Msg("request failed")
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		fmt.Println("collection removed")
		cliLog.Info().Str("cmd", "collection.remove").Str("name", name).Str("workspace", workspace).Msg("cli command completed")
		return
	}
	io.Copy(os.Stdout, resp.Body)
	fmt.Println()
	cliLog.Info().Str("cmd", "collection.remove").Int("status", resp.StatusCode).Msg("cli command completed")
}

func runCollectionList(args []string) {
	cliLog.Info().Str("cmd", "collection.list").Msg("cli command started")
	var workspace string
	for i := 0; i < len(args); i++ {
		if args[i] == "--workspace" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s requires a value\n", args[i])
				os.Exit(1)
			}
			i++
			workspace = args[i]
		}
	}
	if workspace == "" {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain collection list --workspace <hash>")
		os.Exit(1)
	}

	reqURL := fmt.Sprintf("%s/api/v1/collections?workspace=%s", getBaseURL(), url.QueryEscape(workspace))
	resp, err := http.Get(reqURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		cliLog.Error().Err(err).Str("cmd", "collection.list").Msg("request failed")
		os.Exit(1)
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
	fmt.Println()
	cliLog.Info().Str("cmd", "collection.list").Str("workspace", workspace).Msg("cli command completed")
}

func runCollectionRename(args []string) {
	cliLog.Info().Str("cmd", "collection.rename").Msg("cli command started")
	var oldName, newName, workspace string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--old":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s requires a value\n", args[i])
				os.Exit(1)
			}
			i++
			oldName = args[i]
		case "--new":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s requires a value\n", args[i])
				os.Exit(1)
			}
			i++
			newName = args[i]
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s requires a value\n", args[i])
				os.Exit(1)
			}
			i++
			workspace = args[i]
		}
	}
	if oldName == "" || newName == "" || workspace == "" {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain collection rename --old <old> --new <new> --workspace <hash>")
		os.Exit(1)
	}

	body := map[string]string{
		"workspace": workspace,
		"new_name":  newName,
	}
	data, err := json.Marshal(body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal request: %v\n", err)
		os.Exit(1)
	}

	reqURL := fmt.Sprintf("%s/api/v1/collections/%s", getBaseURL(), url.PathEscape(oldName))
	req, _ := http.NewRequest(http.MethodPut, reqURL, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		cliLog.Error().Err(err).Str("cmd", "collection.rename").Msg("request failed")
		os.Exit(1)
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
	fmt.Println()
	cliLog.Info().Str("cmd", "collection.rename").Str("from", oldName).Str("to", newName).Msg("cli command completed")
}
