package internal

import (
	"log/slog"
	"os"
	"strings"
)

type LoggerConfig struct {
	JSON  bool
	Level slog.Level
}

// NewLogger creates a new slog.Logger with the given configuration.
// The logger is not set as the default, allowing it to be injected via constructor.
// Automatically enriches logs with trace context when available.
func NewLogger(cfg LoggerConfig) *slog.Logger {
	handler := slogHandler(cfg.JSON, resolveLevel(cfg.Level))
	traceHandler := NewTraceHandler(handler)
	return slog.New(traceHandler)
}

// InitLogger creates a new logger and sets it as the default slog logger.
// The LOG_FORMAT environment variable overrides cfg.JSON: set it to "json"
// for structured JSON output (e.g. in production / log aggregators).
func InitLogger(cfg LoggerConfig) {
	if f := resolveLogFormat(); f == "json" {
		cfg.JSON = true
	}
	slog.SetDefault(NewLogger(cfg))
}

// resolveLogFormat reads LOG_FORMAT from the environment.
// Returns "json" when set to that value (case-insensitive); empty string otherwise.
func resolveLogFormat() string {
	switch strings.ToLower(os.Getenv("LOG_FORMAT")) {
	case "json":
		return "json"
	default:
		return ""
	}
}

func resolveLevel(defaultLevel slog.Level) slog.Level {
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		switch strings.ToLower(envLevel) {
		case "debug":
			return slog.LevelDebug
		case "info":
			return slog.LevelInfo
		case "warn":
			return slog.LevelWarn
		case "error":
			return slog.LevelError
		}
	}
	return defaultLevel
}

func slogHandler(json bool, level slog.Level) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}
	if json {
		return slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.NewTextHandler(os.Stdout, opts)
}
