package log

import (
	"context"
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
		Format: "console",
	}
}

var logFile *os.File
var globalLogger = NewNoOpLogger()

// Init initialises the global slog logger. Call once at startup after
// loading config. Subsequent calls re-initialise (useful in tests).
func Init(cfg Config) {
	level := parseLevel(cfg.Level)

	var writer io.Writer
	if cfg.Format == "json" {
		writer = os.Stdout
	} else {
		writer = os.Stdout
	}

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

	var handler slog.Handler
	if strings.EqualFold(cfg.Format, "json") {
		handler = slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: level})
	} else {
		handler = newColorHandler(writer, &slog.HandlerOptions{Level: level})
	}
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

type colorHandler struct {
	delegate slog.Handler
}

func newColorHandler(out io.Writer, opts *slog.HandlerOptions) slog.Handler {
	localOpts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if opts != nil {
		localOpts.Level = opts.Level
	}
	localOpts.ReplaceAttr = func(_ []string, a slog.Attr) slog.Attr {
		if a.Key != slog.LevelKey {
			return a
		}
		level, ok := a.Value.Any().(slog.Level)
		if !ok {
			return a
		}
		label := level.String()
		switch {
		case level <= slog.LevelDebug:
			label = "\x1b[36m" + label + "\x1b[0m"
		case level >= slog.LevelError:
			label = "\x1b[31m" + label + "\x1b[0m"
		case level >= slog.LevelWarn:
			label = "\x1b[33m" + label + "\x1b[0m"
		default:
			label = "\x1b[32m" + label + "\x1b[0m"
		}
		return slog.String(a.Key, label)
	}
	return &colorHandler{delegate: slog.NewTextHandler(out, localOpts)}
}

func (h *colorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.delegate.Enabled(ctx, level)
}

func (h *colorHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.delegate.Handle(ctx, r)
}

func (h *colorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &colorHandler{delegate: h.delegate.WithAttrs(attrs)}
}

func (h *colorHandler) WithGroup(name string) slog.Handler {
	return &colorHandler{delegate: h.delegate.WithGroup(name)}
}
