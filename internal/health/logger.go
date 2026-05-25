package health

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
)

// NewLogger creates a zerolog.Logger with multi-writer (stdout + rotating file).
// It takes a LoggingConfig and returns a configured logger and any error.
func NewLogger(cfg config.LoggingConfig) (zerolog.Logger, error) {
	level, err := parseLogLevel(cfg.Level)
	if err != nil {
		return zerolog.Logger{}, fmt.Errorf("invalid log level: %w", err)
	}

	logPath := cfg.File
	if logPath == "" {
		logPath = "~/.nano-brain/logs/nano-brain.log"
	}

	if strings.HasPrefix(logPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return zerolog.Logger{}, fmt.Errorf("failed to get home directory: %w", err)
		}
		logPath = filepath.Join(home, logPath[1:])
	}

	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return zerolog.Logger{}, fmt.Errorf("failed to create log directory: %w", err)
	}

	fileWriter := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    50,
		MaxBackups: 5,
		MaxAge:     0,
		Compress:   true,
	}

	// TTY-aware stdout: use ConsoleWriter for human-readable output on terminals,
	// raw JSON when piped/redirected. Honors NO_COLOR env var.
	var stdoutWriter io.Writer = os.Stdout
	if stat, statErr := os.Stdout.Stat(); statErr == nil && stat.Mode()&os.ModeCharDevice != 0 {
		stdoutWriter = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			NoColor:    os.Getenv("NO_COLOR") != "",
		}
	}

	multiWriter := io.MultiWriter(stdoutWriter, fileWriter)

	logger := zerolog.New(multiWriter).
		With().
		Timestamp().
		Logger().
		Level(level)

	return logger, nil
}

func parseLogLevel(levelStr string) (zerolog.Level, error) {
	levelStr = strings.ToLower(strings.TrimSpace(levelStr))
	switch levelStr {
	case "trace":
		return zerolog.TraceLevel, nil
	case "debug":
		return zerolog.DebugLevel, nil
	case "info":
		return zerolog.InfoLevel, nil
	case "warn":
		return zerolog.WarnLevel, nil
	case "error":
		return zerolog.ErrorLevel, nil
	default:
		return zerolog.InfoLevel, fmt.Errorf("unknown log level: %s", levelStr)
	}
}
