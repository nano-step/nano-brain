package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func runInitCmd(args []string, configPath string) {
	cliLog.Info().Str("cmd", "init").Msg("cli command started")
	hasRoot := false
	for _, a := range args {
		if a == "--root" {
			hasRoot = true
			break
		}
	}
	if !hasRoot {
		runInteractiveInit(configPath)
		cliLog.Info().Str("cmd", "init").Msg("cli command completed")
		return
	}

	var root, workspace string
	var jsonFlag bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--root requires a value\n")
				os.Exit(1)
			}
			i++
			root = args[i]
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
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	body := map[string]string{"root_path": root}
	if workspace != "" {
		body["workspace"] = workspace
	}
	data, err := json.Marshal(body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	resp, _, err := doRequest("POST", getBaseURL()+"/api/v1/init", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		cliLog.Error().Err(err).Str("cmd", "init").Msg("init request failed")
		os.Exit(1)
	}

	if jsonFlag {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "init").Msg("cli command completed")
		return
	}

	var result struct {
		WorkspaceHash string `json:"workspace_hash"`
		RootPath      string `json:"root_path"`
		AgentsSnippet string `json:"agents_snippet"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "init").Msg("cli command completed")
		return
	}
	fmt.Printf("Workspace registered: %s\n", result.WorkspaceHash)
	fmt.Printf("Root path: %s\n", result.RootPath)
	fmt.Println()
	fmt.Println(result.AgentsSnippet)
	cliLog.Info().Str("cmd", "init").Str("workspace_hash", result.WorkspaceHash).Msg("cli command completed")
}

func runWriteCmd(args []string) {
	cliLog.Info().Str("cmd", "write").Msg("cli command started")
	var content, tags, collection, workspace string
	var jsonFlag bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--tags":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--tags requires a value\n")
				os.Exit(1)
			}
			i++
			tags = args[i]
		case "--collection":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "--collection requires a value\n")
				os.Exit(1)
			}
			i++
			collection = args[i]
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
			if content == "" {
				content = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "unexpected argument: %s\n", args[i])
				os.Exit(1)
			}
		}
	}
	if content == "" || workspace == "" {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain write \"<content>\" --workspace <hash> [--tags tag1,tag2] [--collection name] [--json]")
		os.Exit(1)
	}

	body := map[string]interface{}{
		"content":   content,
		"workspace": workspace,
	}
	if tags != "" {
		body["tags"] = strings.Split(tags, ",")
	}
	if collection != "" {
		body["collection"] = collection
	}
	data, err := json.Marshal(body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	resp, _, err := doRequest("POST", getBaseURL()+"/api/v1/write", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		cliLog.Error().Err(err).Str("cmd", "write").Msg("write request failed")
		os.Exit(1)
	}

	if jsonFlag {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "write").Msg("cli command completed")
		return
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "write").Msg("cli command completed")
		return
	}
	fmt.Printf("Document written: %s\n", result.ID)
	cliLog.Info().Str("cmd", "write").Str("document_id", result.ID).Msg("cli command completed")
}

func runStubCmd(endpoint string, args []string) {
	cliLog.Info().Str("cmd", endpoint).Msg("cli command started")
	var query, workspace string
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
			if query == "" {
				query = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "unexpected argument: %s\n", args[i])
				os.Exit(1)
			}
		}
	}
	if query == "" || workspace == "" {
		fmt.Fprintf(os.Stderr, "Usage: nano-brain %s \"<query>\" --workspace <hash> [--json]\n", endpoint)
		os.Exit(1)
	}

	body := map[string]string{
		"query":     query,
		"workspace": workspace,
	}
	data, err := json.Marshal(body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	resp, statusCode, reqErr := doRequest("POST", getBaseURL()+"/api/v1/"+endpoint, bytes.NewReader(data))
	if reqErr != nil {
		if statusCode == 404 {
			name := strings.ToUpper(endpoint[:1]) + endpoint[1:]
			fmt.Fprintf(os.Stderr, "%s endpoint not yet implemented\n", name)
			cliLog.Error().Err(reqErr).Str("cmd", endpoint).Int("status", statusCode).Msg("endpoint not implemented")
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", reqErr)
		cliLog.Error().Err(reqErr).Str("cmd", endpoint).Msg("request failed")
		os.Exit(1)
	}

	if jsonFlag {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", endpoint).Msg("cli command completed")
		return
	}
	fmt.Println(string(resp))
	cliLog.Info().Str("cmd", endpoint).Msg("cli command completed")
}

func runHarvestCmd(args []string) {
	cliLog.Info().Str("cmd", "harvest").Msg("cli command started")
	var jsonFlag bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonFlag = true
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	resp, _, err := doRequest("POST", getBaseURL()+"/api/harvest", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		cliLog.Error().Err(err).Str("cmd", "harvest").Msg("harvest request failed")
		os.Exit(1)
	}

	if jsonFlag {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "harvest").Msg("cli command completed")
		return
	}

	var result struct {
		Harvested int `json:"harvested"`
		Skipped   int `json:"skipped"`
		Errors    int `json:"errors"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "harvest").Msg("cli command completed")
		return
	}
	fmt.Printf("Harvest complete: %d harvested, %d skipped, %d errors\n", result.Harvested, result.Skipped, result.Errors)
	cliLog.Info().
		Str("cmd", "harvest").
		Int("harvested", result.Harvested).
		Int("skipped", result.Skipped).
		Int("errors", result.Errors).
		Msg("cli command completed")
}

func runQueryCmd(args []string) {
	runStubCmd("query", args)
}

func runSearchCmd(args []string) {
	runStubCmd("search", args)
}

func runVSearchCmd(args []string) {
	runStubCmd("vsearch", args)
}
