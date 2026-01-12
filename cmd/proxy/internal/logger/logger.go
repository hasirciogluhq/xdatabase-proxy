package logger

import (
	"context"
	"log/slog"
	"os"
	"sync"
)

var (
	defaultLogger *slog.Logger
	once          sync.Once
)

// Init initializes the global logger based on environment variables.
// DEBUG=true enables debug level logging.
func Init() {
	once.Do(func() {
		level := slog.LevelInfo
		if os.Getenv("DEBUG") == "true" {
			level = slog.LevelDebug
		}

		opts := &slog.HandlerOptions{
			Level: level,
			// Add source file information if in debug mode
			AddSource: level == slog.LevelDebug,
		}

		handler := slog.NewTextHandler(os.Stdout, opts)
		defaultLogger = slog.New(handler)
		slog.SetDefault(defaultLogger)
	})
}

// Debug logs at Debug level.
func Debug(msg string, args ...any) {
	if defaultLogger == nil {
		Init()
	}
	defaultLogger.Debug(msg, args...)
}

// Info logs at Info level.
func Info(msg string, args ...any) {
	if defaultLogger == nil {
		Init()
	}
	defaultLogger.Info(msg, args...)
}

// Warn logs at Warn level.
func Warn(msg string, args ...any) {
	if defaultLogger == nil {
		Init()
	}
	defaultLogger.Warn(msg, args...)
}

// Error logs at Error level.
func Error(msg string, args ...any) {
	if defaultLogger == nil {
		Init()
	}
	defaultLogger.Error(msg, args...)
}

// Fatal logs at Error level and then exits.
func Fatal(msg string, args ...any) {
	if defaultLogger == nil {
		Init()
	}
	defaultLogger.Error(msg, args...)
	os.Exit(1)
}

// With returns a new logger with the given attributes.
func With(args ...any) *slog.Logger {
	if defaultLogger == nil {
		Init()
	}
	return defaultLogger.With(args...)
}

// DebugContext logs at Debug level with context.
func DebugContext(ctx context.Context, msg string, args ...any) {
	if defaultLogger == nil {
		Init()
	}
	defaultLogger.DebugContext(ctx, msg, args...)
}

// InfoContext logs at Info level with context.
func InfoContext(ctx context.Context, msg string, args ...any) {
	if defaultLogger == nil {
		Init()
	}
	defaultLogger.InfoContext(ctx, msg, args...)
}

// WarnContext logs at Warn level with context.
func WarnContext(ctx context.Context, msg string, args ...any) {
	if defaultLogger == nil {
		Init()
	}
	defaultLogger.WarnContext(ctx, msg, args...)
}

// ErrorContext logs at Error level with context.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	if defaultLogger == nil {
		Init()
	}
	defaultLogger.ErrorContext(ctx, msg, args...)
}
