package log_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jaredwarren/rpi_music/log"
	"github.com/rs/zerolog"
)

// captureLogger returns a zerolog.Logger whose output is written to buf at the given level.
func captureLogger(buf *bytes.Buffer, level zerolog.Level) zerolog.Logger {
	return zerolog.New(buf).Level(level)
}

func TestInfo(t *testing.T) {
	var buf bytes.Buffer
	l := captureLogger(&buf, zerolog.InfoLevel)
	l.Info().Msg("hello world")
	if !strings.Contains(buf.String(), "hello world") {
		t.Fatalf("expected 'hello world' in output, got: %s", buf.String())
	}
}

func TestDebugSuppressedAtInfoLevel(t *testing.T) {
	var buf bytes.Buffer
	l := captureLogger(&buf, zerolog.InfoLevel)
	l.Debug().Msg("should not appear")
	if buf.Len() != 0 {
		t.Fatalf("expected no output at Info level for Debug call, got: %s", buf.String())
	}
}

func TestDebugAppearsAtDebugLevel(t *testing.T) {
	var buf bytes.Buffer
	l := captureLogger(&buf, zerolog.DebugLevel)
	l.Debug().Msg("debug message")
	if !strings.Contains(buf.String(), "debug message") {
		t.Fatalf("expected debug output, got: %s", buf.String())
	}
}

func TestWithFields(t *testing.T) {
	var buf bytes.Buffer
	l := captureLogger(&buf, zerolog.InfoLevel).With().Str("key", "value").Logger()
	l.Info().Msg("with fields")
	out := buf.String()
	if !strings.Contains(out, "key") || !strings.Contains(out, "value") {
		t.Fatalf("expected field key/value in output, got: %s", out)
	}
}

func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	l := captureLogger(&buf, zerolog.InfoLevel)
	l.Debug().Msg("before") // suppressed

	l = l.Level(zerolog.DebugLevel)
	l.Debug().Msg("after") // should appear

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
	l.Debug().Msg("x")
	l.Info().Msg("x")
	l.Warn().Msg("x")
	l.Error().Msg("x")
}

func TestInit(t *testing.T) {
	log.Init(log.Config{Level: "debug", Format: "json"})
	logger := log.Get()
	// Should be a valid logger without panic
	var buf bytes.Buffer
	logger = logger.Output(&buf)
	logger.Info().Msg("init test")
	if !strings.Contains(buf.String(), "init test") {
		t.Fatalf("expected 'init test' in output, got: %s", buf.String())
	}
}
