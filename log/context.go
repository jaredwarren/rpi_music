package log

import (
	"context"
	"log/slog"
)

type ContextKey string

const ContextUserKey ContextKey = "logger"

// WithLogger stores a slog.Logger in ctx.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ContextUserKey, l)
}

// FromContext retrieves a slog.Logger from ctx, falling back to the global logger.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ContextUserKey).(*slog.Logger); ok {
		return l
	}
	return Get()
}

// GetLogger is kept as a compatibility alias for FromContext.
func GetLogger(ctx context.Context) *slog.Logger { return FromContext(ctx) }
