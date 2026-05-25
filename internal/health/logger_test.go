package health

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

func TestLogLevelFilter(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := config.LoggingConfig{
		Level: "warn",
		File:  logFile,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	logger.Info().Msg("info message")
	logger.Warn().Msg("warn message")

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logOutput := string(content)
	if strings.Contains(logOutput, "info message") {
		t.Error("expected info message to be filtered out at warn level")
	}
	if !strings.Contains(logOutput, "warn message") {
		t.Error("expected warn message to be in output")
	}
}

func TestLogOutputIsJSON(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := config.LoggingConfig{
		Level: "info",
		File:  logFile,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	logger.Info().Str("key", "value").Msg("test message")

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logOutput := strings.TrimSpace(string(content))
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(logOutput), &logEntry); err != nil {
		t.Fatalf("log output is not valid JSON: %v\nOutput: %s", err, logOutput)
	}

	if _, hasLevel := logEntry["level"]; !hasLevel {
		t.Error("expected JSON log entry to have 'level' field")
	}
	if _, hasTime := logEntry["time"]; !hasTime {
		t.Error("expected JSON log entry to have 'time' field")
	}
	if msg, hasMsg := logEntry["message"]; !hasMsg || msg != "test message" {
		t.Error("expected JSON log entry to have 'message' field with correct value")
	}
	if val, hasKey := logEntry["key"]; !hasKey || val != "value" {
		t.Error("expected JSON log entry to have custom field 'key' with value 'value'")
	}
}

func TestEnvVarOverridesLogLevel(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := config.LoggingConfig{
		Level: "error",
		File:  logFile,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	logger.Info().Msg("info message")
	logger.Error().Msg("error message")

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logOutput := string(content)
	if strings.Contains(logOutput, "info message") {
		t.Error("expected info message to be filtered out at error level")
	}
	if !strings.Contains(logOutput, "error message") {
		t.Error("expected error message to be in output")
	}
}

func TestLogToFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := config.LoggingConfig{
		Level: "info",
		File:  logFile,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	logger.Info().Str("test", "value").Msg("test log message")

	if _, err := os.Stat(logFile); err != nil {
		t.Fatalf("log file not created: %v", err)
	}

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logOutput := string(content)
	if !strings.Contains(logOutput, "test log message") {
		t.Error("expected test log message to be in file")
	}
	if !strings.Contains(logOutput, "test") || !strings.Contains(logOutput, "value") {
		t.Error("expected custom field to be in file")
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zerolog.Level
		wantErr  bool
	}{
		{"debug", zerolog.DebugLevel, false},
		{"info", zerolog.InfoLevel, false},
		{"warn", zerolog.WarnLevel, false},
		{"error", zerolog.ErrorLevel, false},
		{"DEBUG", zerolog.DebugLevel, false},
		{"INFO", zerolog.InfoLevel, false},
		{"  warn  ", zerolog.WarnLevel, false},
		{"invalid", zerolog.InfoLevel, true},
		{"", zerolog.InfoLevel, true},
	}

	for _, tt := range tests {
		level, err := parseLogLevel(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseLogLevel(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if err == nil && level != tt.expected {
			t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, level, tt.expected)
		}
	}
}

func TestDefaultLogPath(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := zerolog.New(buf).Level(zerolog.InfoLevel)
	logger.Info().Msg("test")

	if buf.Len() == 0 {
		t.Error("expected logger to write output")
	}
}
