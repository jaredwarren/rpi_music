package log

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Config mirrors WallCalendar's logger.Config.
type Config struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // "json" or "console"
	File   string `yaml:"file"`   // optional log file path
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Level:  "info",
		Format: "json",
	}
}

var logFile *os.File
var globalLogger = NewNoOpLogger()

// Init initialises the global slog logger. Call once at startup after
// loading config. Subsequent calls re-initialise (useful in tests).
func Init(cfg Config) {
	level := parseLevel(cfg.Level)

	writer := io.Writer(os.Stdout)

	if cfg.File != "" {
		if dir := filepath.Dir(cfg.File); dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		f, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err != nil {
			os.Stderr.WriteString("warning: failed to open log file: " + err.Error() + "\n")
		} else {
			logFile = f
			writer = io.MultiWriter(writer, f)
		}
	}

	// Logs are always emitted as JSON for stable machine parsing and piping.
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: level})
	globalLogger = slog.New(handler)
}

// Get returns the global logger.
func Get() *slog.Logger {
	return globalLogger
}

// Component returns the global logger with a "component" field pre-set.
func Component(name string) *slog.Logger {
	return globalLogger.With("component", name)
}

// NewNoOpLogger returns a logger that discards all output.
func NewNoOpLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// CloseLogFile closes the file log sink if one was opened by Init.
func CloseLogFile() error {
	if logFile != nil {
		if err := logFile.Close(); err != nil {
			return err
		}
		logFile = nil
	}
	return nil
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

