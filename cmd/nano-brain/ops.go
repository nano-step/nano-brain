package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
)

// defaultLogPath returns the default log file path.
func defaultLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "nano-brain.log"
	}
	return filepath.Join(home, ".nano-brain", "logs", "nano-brain.log")
}

// runLogsCmd implements the "logs" CLI command.
func runLogsCmd(args []string) {
	count := 50
	follow := false
	jsonFlag := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "-n requires a value\n")
				os.Exit(1)
			}
			i++
			v, err := strconv.Atoi(args[i])
			if err != nil || v < 1 {
				fmt.Fprintf(os.Stderr, "-n value must be a positive integer\n")
				os.Exit(1)
			}
			count = v
		case "-f", "--follow":
			follow = true
		case "--json":
			jsonFlag = true
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	logPath := resolveLogPath()

	if follow {
		if err := tailFollow(logPath, jsonFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	lines, err := tailLines(logPath, count)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if jsonFlag {
		out, _ := json.Marshal(map[string]interface{}{
			"file":  logPath,
			"lines": lines,
			"count": len(lines),
		})
		fmt.Println(string(out))
		return
	}

	for _, line := range lines {
		fmt.Println(line)
	}
}

// resolveLogPath determines the log file path from config or defaults.
func resolveLogPath() string {
	cfg, err := config.Load(config.DefaultConfigPath())
	if err == nil && cfg.Logging.File != "" {
		p := cfg.Logging.File
		if strings.HasPrefix(p, "~") {
			if home, err := os.UserHomeDir(); err == nil {
				p = filepath.Join(home, p[1:])
			}
		}
		return p
	}
	return defaultLogPath()
}

const tailChunkSize = 64 * 1024

// tailLines reads the last n lines from a file by reading backward from EOF
// in fixed-size chunks. Memory usage is capped regardless of file size.
func tailLines(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open log file %s: %w", path, err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("cannot stat log file: %w", err)
	}
	fileSize := stat.Size()
	if fileSize == 0 {
		return nil, nil
	}

	var collected []string
	remaining := fileSize
	var leftover []byte

	for remaining > 0 && len(collected) < n {
		chunkSize := int64(tailChunkSize)
		if chunkSize > remaining {
			chunkSize = remaining
		}
		offset := remaining - chunkSize
		remaining = offset

		buf := make([]byte, chunkSize)
		if _, err := f.ReadAt(buf, offset); err != nil && err != io.EOF {
			return nil, fmt.Errorf("error reading log file: %w", err)
		}

		if len(leftover) > 0 {
			buf = append(buf, leftover...)
			leftover = nil
		}

		lines := strings.Split(string(buf), "\n")

		if offset > 0 {
			leftover = []byte(lines[0])
			lines = lines[1:]
		}

		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		collected = append(lines, collected...)
	}

	if len(leftover) > 0 && string(leftover) != "" {
		collected = append([]string{string(leftover)}, collected...)
	}

	if len(collected) > n {
		collected = collected[len(collected)-n:]
	}
	return collected, nil
}

// tailFollow implements simple tail -f behavior.
func tailFollow(path string, jsonFlag bool) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open log file %s: %w", path, err)
	}
	defer f.Close()

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("cannot seek to end: %w", err)
	}

	if !jsonFlag {
		fmt.Fprintf(os.Stderr, "Following %s (Ctrl+C to stop)\n", path)
	}

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			return err
		}
		line = strings.TrimRight(line, "\n\r")
		if line == "" {
			continue
		}
		fmt.Println(line)
	}
}

// runDockerCmd implements the "docker" CLI command.
func runDockerCmd(args []string) {
	jsonFlag := false
	var subCmd string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonFlag = true
		case "start", "stop", "status":
			subCmd = args[i]
		default:
			fmt.Fprintf(os.Stderr, "unknown argument: %s\n", args[i])
			os.Exit(1)
		}
	}

	if subCmd == "" {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain docker <start|stop|status> [--json]")
		os.Exit(1)
	}

	composeDir := resolveComposeDir()

	switch subCmd {
	case "start":
		runDockerStart(composeDir, jsonFlag)
	case "stop":
		runDockerStop(composeDir, jsonFlag)
	case "status":
		runDockerStatus(composeDir, jsonFlag)
	}
}

// resolveComposeDir determines the docker-compose directory.
func resolveComposeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".nano-brain")
}

func runDockerStart(dir string, jsonFlag bool) {
	cmd := exec.Command("docker", "compose", "up", "-d")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if jsonFlag {
			j, _ := json.Marshal(map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
				"output": string(out),
			})
			fmt.Println(string(j))
		} else {
			fmt.Fprintf(os.Stderr, "Error starting docker compose: %v\n%s", err, string(out))
		}
		os.Exit(1)
	}

	if jsonFlag {
		j, _ := json.Marshal(map[string]interface{}{
			"status": "started",
			"output": string(out),
		})
		fmt.Println(string(j))
	} else {
		fmt.Println("Docker compose started")
		if len(out) > 0 {
			fmt.Print(string(out))
		}
	}
}

func runDockerStop(dir string, jsonFlag bool) {
	cmd := exec.Command("docker", "compose", "down")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if jsonFlag {
			j, _ := json.Marshal(map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
				"output": string(out),
			})
			fmt.Println(string(j))
		} else {
			fmt.Fprintf(os.Stderr, "Error stopping docker compose: %v\n%s", err, string(out))
		}
		os.Exit(1)
	}

	if jsonFlag {
		j, _ := json.Marshal(map[string]interface{}{
			"status": "stopped",
			"output": string(out),
		})
		fmt.Println(string(j))
	} else {
		fmt.Println("Docker compose stopped")
		if len(out) > 0 {
			fmt.Print(string(out))
		}
	}
}

func runDockerStatus(dir string, jsonFlag bool) {
	cmd := exec.Command("docker", "compose", "ps", "--format", "json")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if jsonFlag {
			j, _ := json.Marshal(map[string]interface{}{
				"status": "error",
				"error":  err.Error(),
				"output": string(out),
			})
			fmt.Println(string(j))
		} else {
			fmt.Fprintf(os.Stderr, "Error getting docker compose status: %v\n%s", err, string(out))
		}
		os.Exit(1)
	}

	if jsonFlag {
		fmt.Print(string(out))
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	fmt.Printf("%-30s %-15s %-20s\n", "NAME", "STATUS", "PORTS")
	fmt.Println(strings.Repeat("-", 65))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var svc struct {
			Name   string `json:"Name"`
			State  string `json:"State"`
			Status string `json:"Status"`
			Ports  string `json:"Ports"`
		}
		if err := json.Unmarshal([]byte(line), &svc); err != nil {
			fmt.Println(line)
			continue
		}
		status := svc.State
		if svc.Status != "" {
			status = svc.Status
		}
		fmt.Printf("%-30s %-15s %-20s\n", svc.Name, status, svc.Ports)
	}
}

// runStatusCmd implements the enhanced "status" CLI command.
func runStatusCmd(args []string) {
	jsonFlag := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonFlag = true
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			os.Exit(1)
		}
	}

	resp, _, err := doRequest("GET", getBaseURL()+"/api/status", nil)
	if err != nil {
		if jsonFlag {
			j, _ := json.Marshal(map[string]interface{}{
				"status": "unreachable",
				"error":  err.Error(),
			})
			fmt.Println(string(j))
		} else {
			fmt.Fprintf(os.Stderr, "Cannot reach nano-brain server: %v\n", err)
		}
		os.Exit(1)
	}

	if jsonFlag {
		fmt.Println(string(resp))
		return
	}

	var status map[string]interface{}
	if err := json.Unmarshal(resp, &status); err != nil {
		fmt.Println(string(resp))
		return
	}

	fmt.Println("nano-brain status")
	fmt.Println(strings.Repeat("=", 40))

	if v, ok := status["status"]; ok {
		fmt.Printf("  Server:       %v\n", v)
	}
	if v, ok := status["version"]; ok {
		fmt.Printf("  Version:      %v\n", v)
	}
	if v, ok := status["uptime"]; ok {
		fmt.Printf("  Uptime:       %v\n", v)
	}

	if db, ok := status["database"].(map[string]interface{}); ok {
		fmt.Println("\nDatabase:")
		if v, ok := db["status"]; ok {
			fmt.Printf("  Status:       %v\n", v)
		}
		if v, ok := db["pool_size"]; ok {
			fmt.Printf("  Pool size:    %v\n", v)
		}
		if v, ok := db["active_conns"]; ok {
			fmt.Printf("  Active conns: %v\n", v)
		}
		if v, ok := db["idle_conns"]; ok {
			fmt.Printf("  Idle conns:   %v\n", v)
		}
	}

	if v, ok := status["workspaces"]; ok {
		fmt.Printf("\nWorkspaces:     %v\n", v)
	}
	if v, ok := status["workspace_count"]; ok {
		fmt.Printf("\nWorkspaces:     %v\n", v)
	}

	if v, ok := status["collections"]; ok {
		fmt.Printf("Collections:    %v\n", v)
	}
	if v, ok := status["collection_count"]; ok {
		fmt.Printf("Collections:    %v\n", v)
	}

	if eq, ok := status["embedding_queue"].(map[string]interface{}); ok {
		fmt.Println("\nEmbedding Queue:")
		if v, ok := eq["depth"]; ok {
			fmt.Printf("  Queue depth:  %v\n", v)
		}
		if v, ok := eq["pending"]; ok {
			fmt.Printf("  Pending:      %v\n", v)
		}
	}
}

// runGooseMigrateCmd runs pending goose migrations from embedded SQL files.
func runGooseMigrateCmd(jsonFlag bool) {
	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Database.URL == "" {
		fmt.Fprintln(os.Stderr, "Error: database.url not configured")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting dialect: %v\n", err)
		os.Exit(1)
	}

	current, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current version: %v\n", err)
		os.Exit(1)
	}

	goose.SetLogger(goose.NopLogger())

	if err := goose.UpContext(ctx, db, "."); err != nil {
		if jsonFlag {
			j, _ := json.Marshal(map[string]interface{}{
				"status":           "error",
				"error":            err.Error(),
				"previous_version": current,
			})
			fmt.Println(string(j))
		} else {
			fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
		}
		os.Exit(1)
	}

	after, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting post-migration version: %v\n", err)
		os.Exit(1)
	}

	if jsonFlag {
		j, _ := json.Marshal(map[string]interface{}{
			"status":           "ok",
			"previous_version": current,
			"current_version":  after,
			"migrations_run":   after - current,
		})
		fmt.Println(string(j))
	} else {
		if after == current {
			fmt.Printf("Database is up to date (version %d)\n", after)
		} else {
			fmt.Printf("Migrated from version %d to %d (%d migrations applied)\n", current, after, after-current)
		}
	}
}

func printUsage() {
	fmt.Printf(`nano-brain %s — persistent memory for AI coding agents

Usage: nano-brain [command] [flags]

Commands:
  (no command)       Start the server (foreground)
  serve              Start the server (foreground)
  serve -d           Start the server (background/daemon)
  stop               Stop background server
  restart            Restart background server
  init               Interactive setup wizard (or --root <path> to register workspace)
  doctor             Check prerequisites (PostgreSQL, pgvector, embedding provider)
  status             Show server status
  version            Show version
  config show        Show current configuration
  config check       Validate configuration
  query              Hybrid search (BM25 + vector)
  search             BM25 keyword search
  vsearch            Vector similarity search
  workspaces         List registered workspaces (alias: ls)
  write              Write a document
  collection         Manage collections (add/remove/list)
  harvest            Trigger session harvesting
  logs               View log file (-f to follow, -n <count>)
  docker             Manage Docker Compose (start/stop/status)
  db:migrate         Run database migrations
  bench              Benchmarking suite (generate/run/compare/stress)
  help               Show this help

Global flags:
  --config <path>    Config file path (default: ~/.nano-brain/config.yml)

`, Version)
}

func runVersionCmd(args []string) {
	jsonFlag := false
	for _, a := range args {
		if a == "--json" {
			jsonFlag = true
		}
	}
	if jsonFlag {
		j, _ := json.Marshal(map[string]string{"version": Version})
		fmt.Println(string(j))
		return
	}
	fmt.Printf("nano-brain %s\n", Version)
}
