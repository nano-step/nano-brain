//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func pidFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "nano-brain.pid"
	}
	return filepath.Join(home, ".nano-brain", "nano-brain.pid")
}

func readPID() (int, error) {
	data, err := os.ReadFile(pidFilePath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func isRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func runServeCmd(args []string, configPath string) {
	cliLog.Debug().Str("cmd", "serve").Msg("cli command started")
	daemon := false
	for _, a := range args {
		switch a {
		case "-d", "--detach":
			daemon = true
		case "--unsafe-no-auth":
			unsafeNoAuth = true
		case "--serve-only":
			serveOnlyFlag = true
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", a)
			fmt.Fprintln(os.Stderr, "Usage: nano-brain serve [-d] [--unsafe-no-auth] [--serve-only]")
			os.Exit(1)
		}
	}

	if daemon {
		runServeDaemon(configPath)
		return
	}

	startServer(configPath)
}

func runServeDaemon(configPath string) {
	if err := guardBeforeStart(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	if pid, err := readPID(); err == nil && isRunning(pid) {
		fmt.Fprintf(os.Stderr, "nano-brain is already running (PID: %d)\n", pid)
		os.Exit(1)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot resolve binary path: %v\n", err)
		os.Exit(1)
	}

	logPath := defaultLogPath()
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		fmt.Fprintf(os.Stderr, "cannot create log directory: %v\n", err)
		os.Exit(1)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open log file %s: %v\n", logPath, err)
		os.Exit(1)
	}

	childArgs := []string{exe, "--daemon-child"}
	if configPath != "" {
		childArgs = append(childArgs, "--config", configPath)
	}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open /dev/null: %v\n", err)
		os.Exit(1)
	}

	attr := &os.ProcAttr{
		Dir:   "/",
		Files: []*os.File{devNull, logFile, logFile},
		Sys:   &syscall.SysProcAttr{Setsid: true},
	}

	proc, err := os.StartProcess(exe, childArgs, attr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start daemon: %v\n", err)
		os.Exit(1)
	}

	childPID := proc.Pid

	if err := os.MkdirAll(filepath.Dir(pidFilePath()), 0700); err != nil {
		fmt.Fprintf(os.Stderr, "cannot create PID directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(pidFilePath(), []byte(strconv.Itoa(childPID)), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "cannot write PID file: %v\n", err)
		os.Exit(1)
	}

	_ = proc.Release()
	_ = logFile.Close()
	_ = devNull.Close()

	fmt.Printf("nano-brain started (PID: %d)\n", childPID)
	fmt.Printf("Logs: %s\n", logPath)
	cliLog.Debug().Str("cmd", "serve").Int("pid", childPID).Str("pid_file", pidFilePath()).Msg("daemon started")
}

func runStopCmd() {
	cliLog.Debug().Str("cmd", "stop").Msg("cli command started")
	pid, err := readPID()
	if err != nil {
		fmt.Fprintln(os.Stderr, "nano-brain is not running")
		cliLog.Error().Err(err).Str("cmd", "stop").Msg("read pid file failed")
		os.Exit(1)
	}

	if !isRunning(pid) {
		_ = os.Remove(pidFilePath())
		fmt.Fprintln(os.Stderr, "nano-brain is not running (cleaned stale PID file)")
		os.Exit(1)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot find process %d: %v\n", pid, err)
		os.Exit(1)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "failed to send SIGTERM to PID %d: %v\n", pid, err)
		os.Exit(1)
	}

	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if !isRunning(pid) {
			_ = os.Remove(pidFilePath())
			fmt.Println("nano-brain stopped")
			cliLog.Debug().Str("cmd", "stop").Int("pid", pid).Msg("cli command completed")
			return
		}
	}

	_ = proc.Signal(syscall.SIGKILL)
	_ = os.Remove(pidFilePath())
	fmt.Println("nano-brain stopped (forced)")
	cliLog.Debug().Str("cmd", "stop").Int("pid", pid).Bool("forced", true).Msg("cli command completed")
}

func runRestartCmd(args []string, configPath string) {
	cliLog.Debug().Str("cmd", "restart").Msg("cli command started")
	if pid, err := readPID(); err == nil && isRunning(pid) {
		proc, _ := os.FindProcess(pid)
		_ = proc.Signal(syscall.SIGTERM)
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			if !isRunning(pid) {
				break
			}
		}
		if isRunning(pid) {
			_ = proc.Signal(syscall.SIGKILL)
			time.Sleep(200 * time.Millisecond)
		}
		_ = os.Remove(pidFilePath())
	}

	runServeDaemon(configPath)
}
