package internal

import (
	"context"
	"log/slog"
)

// traceContextKey is used to store/retrieve trace IDs in context.
type traceContextKey struct{}

// TraceIDFromContext retrieves the trace ID from context if present
func TraceIDFromContext(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceContextKey{}).(string); ok {
		return traceID
	}
	return ""
}

// TraceHandler wraps an slog.Handler to add trace context to log records.
// If a trace ID is present in the context, it's automatically added to the log record.
type TraceHandler struct {
	handler slog.Handler
}

// NewTraceHandler creates a new handler that enriches logs with trace context.
func NewTraceHandler(handler slog.Handler) *TraceHandler {
	return &TraceHandler{handler: handler}
}

// Enabled delegates to the wrapped handler.
func (h *TraceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle enriches the record with trace context if present.
func (h *TraceHandler) Handle(ctx context.Context, record slog.Record) error {
	if traceID := TraceIDFromContext(ctx); traceID != "" {
		record.AddAttrs(slog.String("trace_id", traceID))
	}
	return h.handler.Handle(ctx, record)
}

// WithAttrs delegates to the wrapped handler.
func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return NewTraceHandler(h.handler.WithAttrs(attrs))
}

// WithGroup delegates to the wrapped handler.
func (h *TraceHandler) WithGroup(name string) slog.Handler {
	return NewTraceHandler(h.handler.WithGroup(name))
}
