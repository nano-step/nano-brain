package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nano-brain/nano-brain/internal/storage"
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
	var jsonFlag, forceFlag bool
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
		case "--force":
			forceFlag = true
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	if forceFlag && root != "" {
		hash, err := storage.WorkspaceHash(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not resolve workspace for path %q: %v\n", root, err)
			os.Exit(1)
		}
		resetData, _ := json.Marshal(map[string]string{"workspace": hash})
		resetResp, _, err := doRequest("POST", getBaseURL()+"/api/v1/reset-workspace", bytes.NewReader(resetData))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resetting workspace: %v\n", err)
			os.Exit(1)
		}
		var resetResult struct {
			DeletedDocuments int64  `json:"deleted_documents"`
			Workspace        string `json:"workspace"`
		}
		if jsonErr := json.Unmarshal(resetResp, &resetResult); jsonErr == nil {
			fmt.Printf("Resetting workspace %s... deleted %d documents.\n", resetResult.Workspace, resetResult.DeletedDocuments)
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

	triggerInitBackground(result.WorkspaceHash, root)

	cliLog.Info().Str("cmd", "init").Str("workspace_hash", result.WorkspaceHash).Msg("cli command completed")
}

func triggerInitBackground(workspaceHash, root string) {
	reindexBody, _ := json.Marshal(map[string]string{"workspace": workspaceHash, "root": root})
	if _, _, err := doRequest("POST", getBaseURL()+"/api/v1/reindex", bytes.NewReader(reindexBody)); err != nil {
		cliLog.Warn().Err(err).Str("workspace", workspaceHash).Msg("auto reindex trigger failed")
	}

	harvestResp, status, err := doRequest("POST", getBaseURL()+"/api/harvest", nil)
	if err != nil {
		if status == 503 {
			cliLog.Info().Msg("harvest skipped: no session harvester configured")
		} else {
			cliLog.Warn().Err(err).Msg("auto harvest trigger failed")
			fmt.Println("Warning: harvest failed:", err)
		}
	} else {
		var result struct {
			Harvested int `json:"harvested"`
			Skipped   int `json:"skipped"`
			Errors    int `json:"errors"`
		}
		if jsonErr := json.Unmarshal(harvestResp, &result); jsonErr == nil {
			fmt.Printf("Harvest: %d harvested, %d skipped, %d errors\n", result.Harvested, result.Skipped, result.Errors)
		}
	}

	fmt.Println()
	fmt.Println("Indexing codebase in background. Run 'nano-brain status' to check progress.")
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

type stubFlags struct {
	query           string
	workspace       string
	scope           string
	tags            []string
	jsonFlag        bool
	createdAfter    string
	createdBefore   string
	updatedAfter    string
	updatedBefore   string
}

func parseTagList(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseStubFlags(args []string) (stubFlags, string) {
	f := stubFlags{scope: "workspace"}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--workspace":
			if i+1 >= len(args) {
				return f, "--workspace requires a value"
			}
			i++
			f.workspace = args[i]
		case strings.HasPrefix(arg, "--workspace="):
			f.workspace = strings.TrimPrefix(arg, "--workspace=")
		case arg == "--scope":
			if i+1 >= len(args) {
				return f, "--scope requires a value"
			}
			i++
			f.scope = args[i]
		case strings.HasPrefix(arg, "--scope="):
			f.scope = strings.TrimPrefix(arg, "--scope=")
		case arg == "--tags":
			if i+1 >= len(args) {
				return f, "--tags requires a value"
			}
			i++
			f.tags = parseTagList(args[i])
		case strings.HasPrefix(arg, "--tags="):
			f.tags = parseTagList(strings.TrimPrefix(arg, "--tags="))
		case arg == "--created-after":
			if i+1 >= len(args) {
				return f, "--created-after requires a value"
			}
			i++
			f.createdAfter = args[i]
		case strings.HasPrefix(arg, "--created-after="):
			f.createdAfter = strings.TrimPrefix(arg, "--created-after=")
		case arg == "--created-before":
			if i+1 >= len(args) {
				return f, "--created-before requires a value"
			}
			i++
			f.createdBefore = args[i]
		case strings.HasPrefix(arg, "--created-before="):
			f.createdBefore = strings.TrimPrefix(arg, "--created-before=")
		case arg == "--updated-after":
			if i+1 >= len(args) {
				return f, "--updated-after requires a value"
			}
			i++
			f.updatedAfter = args[i]
		case strings.HasPrefix(arg, "--updated-after="):
			f.updatedAfter = strings.TrimPrefix(arg, "--updated-after=")
		case arg == "--updated-before":
			if i+1 >= len(args) {
				return f, "--updated-before requires a value"
			}
			i++
			f.updatedBefore = args[i]
		case strings.HasPrefix(arg, "--updated-before="):
			f.updatedBefore = strings.TrimPrefix(arg, "--updated-before=")
		case arg == "--json":
			f.jsonFlag = true
		case strings.HasPrefix(arg, "--"):
			return f, "unknown flag: " + arg
		default:
			if f.query == "" {
				f.query = arg
			} else {
				return f, "unexpected argument: " + arg
			}
		}
	}

	if f.scope != "workspace" && f.scope != "all" {
		return f, fmt.Sprintf("invalid --scope value %q: must be \"workspace\" or \"all\"", f.scope)
	}
	return f, ""
}

func runStubCmd(endpoint string, args []string) {
	cliLog.Info().Str("cmd", endpoint).Msg("cli command started")

	f, errMsg := parseStubFlags(args)
	if errMsg != "" {
		fmt.Fprintf(os.Stderr, "%s\n", errMsg)
		os.Exit(1)
	}

	var workspaceVal string
	if f.scope == "all" {
		workspaceVal = "all"
	} else {
		workspaceVal = f.workspace
	}

	if f.query == "" || workspaceVal == "" {
		fmt.Fprintf(os.Stderr, "Usage: nano-brain %s \"<query>\" --workspace <hash> [--scope all|workspace] [--json]\n", endpoint)
		os.Exit(1)
	}

	bodyMap := map[string]interface{}{
		"query":     f.query,
		"workspace": workspaceVal,
	}
	if len(f.tags) > 0 {
		bodyMap["tags"] = f.tags
	}
	if f.createdAfter != "" {
		bodyMap["created_after"] = f.createdAfter
	}
	if f.createdBefore != "" {
		bodyMap["created_before"] = f.createdBefore
	}
	if f.updatedAfter != "" {
		bodyMap["updated_after"] = f.updatedAfter
	}
	if f.updatedBefore != "" {
		bodyMap["updated_before"] = f.updatedBefore
	}
	data, err := json.Marshal(bodyMap)
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

	if f.jsonFlag {
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

func runReindexCmd(args []string) {
	cliLog.Info().Str("cmd", "reindex").Msg("cli command started")
	var root, workspace string
	var jsonFlag, forceWipe bool

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
		case "--force-wipe":
			forceWipe = true
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	if root == "" {
		fmt.Fprintf(os.Stderr, "Usage: nano-brain reindex --root <path> [--workspace <hash>] [--force-wipe] [--json]\n")
		os.Exit(1)
	}

	if workspace == "" {
		h, err := storage.WorkspaceHash(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not resolve workspace for path %q: %v\n", root, err)
			cliLog.Error().Err(err).Str("cmd", "reindex").Str("root", root).Msg("failed to derive workspace hash")
			os.Exit(1)
		}
		workspace = h
	}

	reqBody := map[string]interface{}{"root": root, "workspace": workspace, "force_wipe": forceWipe}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal request: %v\n", err)
		cliLog.Error().Err(err).Str("cmd", "reindex").Msg("failed to marshal request")
		os.Exit(1)
	}

	resp, _, err := doRequest("POST", getBaseURL()+"/api/v1/reindex", bytes.NewReader(bodyBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		cliLog.Error().Err(err).Str("cmd", "reindex").Msg("reindex request failed")
		os.Exit(1)
	}

	if jsonFlag {
		fmt.Println(string(resp))
		cliLog.Info().Str("cmd", "reindex").Msg("cli command completed")
		return
	}

	fmt.Printf("Reindex queued for collection '%s'\n", root)
	cliLog.Info().Str("cmd", "reindex").Str("root", root).Bool("force_wipe", forceWipe).Msg("cli command completed")
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
