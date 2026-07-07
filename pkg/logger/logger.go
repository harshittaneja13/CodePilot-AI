// Package logger provides structured logging for the CodePilot AI application
// using zerolog. It supports pretty console output in development mode and
// JSON-formatted output in production.
package logger

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Log is the global logger instance used throughout the application.
var Log zerolog.Logger

// contextKey is an unexported type used for context keys to avoid collisions.
type contextKey string

const requestIDKey contextKey = "request_id"

// Init initializes the global logger based on the environment.
// In "development" mode, it uses a pretty console writer.
// In all other modes (production), it uses structured JSON output.
func Init(environment string, level string) {
	var output io.Writer

	if environment == "development" {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	} else {
		output = os.Stdout
	}

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().UTC()
	}

	Log = zerolog.New(output).
		Level(lvl).
		With().
		Timestamp().
		Caller().
		Str("service", "codepilot-ai").
		Logger()
}

// WithContext returns a logger enriched with the request_id from the given context.
// If no request_id is found, it returns the base logger.
func WithContext(ctx context.Context) zerolog.Logger {
	if reqID, ok := ctx.Value(requestIDKey).(string); ok {
		return Log.With().Str("request_id", reqID).Logger()
	}
	return Log
}

// SetRequestID stores a request ID in the context for later retrieval by WithContext.
func SetRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID retrieves the request ID from context.
// Returns an empty string if no request ID is set.
func GetRequestID(ctx context.Context) string {
	if reqID, ok := ctx.Value(requestIDKey).(string); ok {
		return reqID
	}
	return ""
}
