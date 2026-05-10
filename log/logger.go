package log

import (
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
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

// Init initialises the global zerolog logger. Call once at startup after
// loading config. Subsequent calls re-initialise (useful in tests).
func Init(cfg Config) {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	var writer io.Writer
	if cfg.Format == "json" {
		writer = os.Stdout
	} else {
		writer = zerolog.ConsoleWriter{Out: os.Stdout}
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

	zlog.Logger = zerolog.New(writer).With().Timestamp().Logger()
}

// Get returns the global logger.
func Get() zerolog.Logger {
	return zlog.Logger
}

// Component returns the global logger with a "component" field pre-set.
func Component(name string) zerolog.Logger {
	return zlog.With().Str("component", name).Logger()
}

// NewNoOpLogger returns a logger that discards all output.
func NewNoOpLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
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
