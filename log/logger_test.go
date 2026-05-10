package log_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/jaredwarren/rpi_music/log"
)

// captureLogger returns a slog.Logger whose output is written to buf at the given level.
func captureLogger(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: level}))
}

func TestInfo(t *testing.T) {
	var buf bytes.Buffer
	l := captureLogger(&buf, slog.LevelInfo)
	l.Info("hello world")
	if !strings.Contains(buf.String(), "hello world") {
		t.Fatalf("expected 'hello world' in output, got: %s", buf.String())
	}
}

func TestDebugSuppressedAtInfoLevel(t *testing.T) {
	var buf bytes.Buffer
	l := captureLogger(&buf, slog.LevelInfo)
	l.Debug("should not appear")
	if buf.Len() != 0 {
		t.Fatalf("expected no output at Info level for Debug call, got: %s", buf.String())
	}
}

func TestDebugAppearsAtDebugLevel(t *testing.T) {
	var buf bytes.Buffer
	l := captureLogger(&buf, slog.LevelDebug)
	l.Debug("debug message")
	if !strings.Contains(buf.String(), "debug message") {
		t.Fatalf("expected debug output, got: %s", buf.String())
	}
}

func TestWithFields(t *testing.T) {
	var buf bytes.Buffer
	l := captureLogger(&buf, slog.LevelInfo).With("key", "value")
	l.Info("with fields")
	out := buf.String()
	if !strings.Contains(out, "key") || !strings.Contains(out, "value") {
		t.Fatalf("expected field key/value in output, got: %s", out)
	}
}

func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	l := captureLogger(&buf, slog.LevelInfo)
	l.Debug("before") // suppressed

	l = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	l.Debug("after") // should appear

	if !strings.Contains(buf.String(), "after") {
		t.Fatalf("expected 'after' in output after Level(Debug), got: %s", buf.String())
	}
	if strings.Contains(buf.String(), "before") {
		t.Fatalf("'before' should have been suppressed at Info level")
	}
}

func TestNoOpLogger(t *testing.T) {
	l := log.NewNoOpLogger()
	// Should not panic
	l.Debug("x")
	l.Info("x")
	l.Warn("x")
	l.Error("x")
}

func TestInit(t *testing.T) {
	log.Init(log.Config{Level: "debug", Format: "json"})
	logger := log.Get()
	// Should be a valid logger without panic
	var buf bytes.Buffer
	logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger.Info("init test")
	if !strings.Contains(buf.String(), "init test") {
		t.Fatalf("expected 'init test' in output, got: %s", buf.String())
	}
}
