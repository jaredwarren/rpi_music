package log

import (
	"context"

	"github.com/rs/zerolog"
)

type ContextKey string

const ContextUserKey ContextKey = "logger"

// WithLogger stores a zerolog.Logger in ctx.
func WithLogger(ctx context.Context, l zerolog.Logger) context.Context {
	return context.WithValue(ctx, ContextUserKey, l)
}

// GetLogger retrieves a zerolog.Logger from ctx, falling back to the global logger.
func GetLogger(ctx context.Context) zerolog.Logger {
	if l, ok := ctx.Value(ContextUserKey).(zerolog.Logger); ok {
		return l
	}
	return Get()
}
