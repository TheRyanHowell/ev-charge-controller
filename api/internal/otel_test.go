package internal

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testSlogHandler struct {
	records []slog.Record
	attrs   []slog.Attr
	groups  []string
}

func newTestSlogHandler() *testSlogHandler {
	return &testSlogHandler{records: make([]slog.Record, 0)}
}

func (h *testSlogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *testSlogHandler) Handle(ctx context.Context, record slog.Record) error {
	h.records = append(h.records, record)
	return nil
}

func (h *testSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.attrs = append(h.attrs, attrs...)
	return h
}

func (h *testSlogHandler) WithGroup(name string) slog.Handler {
	h.groups = append(h.groups, name)
	return h
}

func TestNewTraceHandler(t *testing.T) {
	handler := newTestSlogHandler()
	traceHandler := NewTraceHandler(handler)
	assert.NotNil(t, traceHandler)
	assert.Same(t, handler, traceHandler.handler)
}

func TestTraceHandler_Enabled(t *testing.T) {
	handler := newTestSlogHandler()
	traceHandler := NewTraceHandler(handler)

	assert.True(t, traceHandler.Enabled(context.Background(), slog.LevelInfo))
}

func TestTraceHandler_Handle_WithTraceID(t *testing.T) {
	handler := newTestSlogHandler()
	traceHandler := NewTraceHandler(handler)

	ctx := context.WithValue(context.Background(), traceContextKey{}, "trace-123")
	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test message", 0)

	err := traceHandler.Handle(ctx, record)
	require.NoError(t, err)
	assert.Len(t, handler.records, 1)
}

func TestTraceHandler_Handle_WithoutTraceID(t *testing.T) {
	handler := newTestSlogHandler()
	traceHandler := NewTraceHandler(handler)

	ctx := context.Background()
	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test message", 0)

	err := traceHandler.Handle(ctx, record)
	require.NoError(t, err)
	assert.Len(t, handler.records, 1)
}

func TestTraceHandler_WithAttrs(t *testing.T) {
	handler := newTestSlogHandler()
	traceHandler := NewTraceHandler(handler)

	newHandler := traceHandler.WithAttrs([]slog.Attr{slog.String("key", "value")})
	assert.NotNil(t, newHandler)
	_, ok := newHandler.(*TraceHandler)
	assert.True(t, ok)
}

func TestTraceHandler_WithGroup(t *testing.T) {
	handler := newTestSlogHandler()
	traceHandler := NewTraceHandler(handler)

	newHandler := traceHandler.WithGroup("test-group")
	assert.NotNil(t, newHandler)
	_, ok := newHandler.(*TraceHandler)
	assert.True(t, ok)
}

func TestTraceIDFromContext(t *testing.T) {
	ctx := context.Background()
	assert.Empty(t, TraceIDFromContext(ctx))

	ctx = context.WithValue(ctx, traceContextKey{}, "trace-456")
	assert.Equal(t, "trace-456", TraceIDFromContext(ctx))
}

func TestTraceIDFromContext_NonString(t *testing.T) {
	ctx := context.WithValue(context.Background(), traceContextKey{}, 123)
	assert.Empty(t, TraceIDFromContext(ctx))
}
