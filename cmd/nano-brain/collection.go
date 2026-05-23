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

func collectionBaseURL() string {
	host := os.Getenv("NANO_BRAIN_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("NANO_BRAIN_PORT")
	if port == "" {
		port = "3100"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
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

	resp, err := http.Post(collectionBaseURL()+"/api/v1/collections", "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
	fmt.Println()
}

func runCollectionRemove(args []string) {
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

	reqURL := fmt.Sprintf("%s/api/v1/collections/%s?workspace=%s", collectionBaseURL(), url.PathEscape(name), url.QueryEscape(workspace))
	req, _ := http.NewRequest(http.MethodDelete, reqURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		fmt.Println("collection removed")
		return
	}
	io.Copy(os.Stdout, resp.Body)
	fmt.Println()
}

func runCollectionList(args []string) {
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

	reqURL := fmt.Sprintf("%s/api/v1/collections?workspace=%s", collectionBaseURL(), url.QueryEscape(workspace))
	resp, err := http.Get(reqURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
	fmt.Println()
}

func runCollectionRename(args []string) {
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

	reqURL := fmt.Sprintf("%s/api/v1/collections/%s", collectionBaseURL(), url.PathEscape(oldName))
	req, _ := http.NewRequest(http.MethodPut, reqURL, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
	fmt.Println()
}
