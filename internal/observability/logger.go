package observability

import (
	"context"
	"log/slog"
	"os"
)

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
)

var defaultLogger *slog.Logger

func init() {
	// Default to JSON handler in production, or text handler if LOG_FORMAT=text
	handlerOptions := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	if os.Getenv("LOG_LEVEL") == "debug" {
		handlerOptions.Level = slog.LevelDebug
	}

	var handler slog.Handler
	if os.Getenv("LOG_FORMAT") == "text" {
		handler = slog.NewTextHandler(os.Stdout, handlerOptions)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, handlerOptions)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

// Logger returns the global structured logger.
func Logger() *slog.Logger {
	return defaultLogger
}

// FromContext extracts contextual fields (request_id, user_id) and returns a logger enriched with them.
func FromContext(ctx context.Context) *slog.Logger {
	l := defaultLogger
	if ctx == nil {
		return l
	}
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
		l = l.With(slog.String("request_id", reqID))
	}
	if userID, ok := ctx.Value(UserIDKey).(uint); ok && userID != 0 {
		l = l.With(slog.Uint64("user_id", uint64(userID)))
	}
	return l
}
