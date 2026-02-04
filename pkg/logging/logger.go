// Package logging provides structured logging for G2 using Go's slog.
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

var logger *slog.Logger

// Level represents log level.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Config controls logging behavior.
type Config struct {
	Level      Level
	JSONFormat bool
	Output     io.Writer
}

// DefaultConfig returns default logging configuration.
func DefaultConfig() Config {
	return Config{
		Level:      LevelWarn,
		JSONFormat: false,
		Output:     os.Stderr,
	}
}

// Init initializes the logger with the given configuration.
func Init(cfg Config) {
	level := parseLevel(cfg.Level)
	output := cfg.Output
	if output == nil {
		output = os.Stderr
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if cfg.JSONFormat {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	logger = slog.New(handler)
}

// parseLevel converts a Level string to slog.Level.
func parseLevel(l Level) slog.Level {
	switch strings.ToLower(string(l)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}

// ParseLevel parses a level string and returns the Level.
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelWarn
	}
}

// ensureInit initializes the logger with defaults if not already initialized.
func ensureInit() {
	if logger == nil {
		Init(DefaultConfig())
	}
}

// Debug logs a debug message with optional key-value pairs.
func Debug(msg string, args ...any) {
	ensureInit()
	logger.Debug(msg, args...)
}

// Info logs an info message with optional key-value pairs.
func Info(msg string, args ...any) {
	ensureInit()
	logger.Info(msg, args...)
}

// Warn logs a warning message with optional key-value pairs.
func Warn(msg string, args ...any) {
	ensureInit()
	logger.Warn(msg, args...)
}

// Error logs an error message with optional key-value pairs.
func Error(msg string, args ...any) {
	ensureInit()
	logger.Error(msg, args...)
}

// With returns a logger with the given attributes.
func With(args ...any) *slog.Logger {
	ensureInit()
	return logger.With(args...)
}

// SetLogger sets a custom logger (useful for testing).
func SetLogger(l *slog.Logger) {
	logger = l
}

// GetLogger returns the current logger.
func GetLogger() *slog.Logger {
	ensureInit()
	return logger
}
