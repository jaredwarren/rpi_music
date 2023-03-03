package log

import "context"

type ContextKey string

const ContextUserKey ContextKey = "logger"

// WithLogger adds Logger to context
func WithLogger(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, ContextUserKey, l)
}

// GetLogger extract Logger from context
// return global so never return nil
func GetLogger(ctx context.Context) Logger {
	cl := ctx.Value(ContextUserKey)
	if cl != nil {
		if l, ok := cl.(Logger); ok {
			if l != nil {
				return l
			}
		}
	}
	return Get()
}
