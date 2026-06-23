package internal

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitLogger_JSON(t *testing.T) {
	var buf bytes.Buffer
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	InitLogger(LoggerConfig{JSON: true, Level: slog.LevelInfo})

	slog.Info("test message", "key", "value")

	w.Close()
	os.Stdout = orig
	_, _ = buf.ReadFrom(r)

	var entry map[string]interface{}
	require.NoError(t, json.NewDecoder(&buf).Decode(&entry))
	assert.Equal(t, "test message", entry["msg"])
	assert.Equal(t, "value", entry["key"])
}

func TestInitLogger_Text(t *testing.T) {
	var buf bytes.Buffer
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	InitLogger(LoggerConfig{JSON: false, Level: slog.LevelInfo})

	slog.Info("test message", "key", "value")

	w.Close()
	os.Stdout = orig
	_, _ = buf.ReadFrom(r)

	output := buf.String()
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "key=value")
}

func TestInitLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	InitLogger(LoggerConfig{JSON: true, Level: slog.LevelInfo})

	slog.Warn("warning message", "reason", "something")

	w.Close()
	os.Stdout = orig
	_, _ = buf.ReadFrom(r)

	var entry map[string]interface{}
	require.NoError(t, json.NewDecoder(&buf).Decode(&entry))
	assert.Equal(t, "warning message", entry["msg"])
	assert.Equal(t, "something", entry["reason"])
	assert.Equal(t, "WARN", entry["level"])
}

func TestInitLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	InitLogger(LoggerConfig{JSON: true, Level: slog.LevelInfo})

	slog.Error("error message", "err", "something failed")

	w.Close()
	os.Stdout = orig
	_, _ = buf.ReadFrom(r)

	var entry map[string]interface{}
	require.NoError(t, json.NewDecoder(&buf).Decode(&entry))
	assert.Equal(t, "error message", entry["msg"])
	assert.Equal(t, "something failed", entry["err"])
	assert.Equal(t, "ERROR", entry["level"])
}

func TestInitLogger_DebugLevel(t *testing.T) {
	var buf bytes.Buffer
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	InitLogger(LoggerConfig{JSON: true, Level: slog.LevelDebug})

	slog.Debug("debug message", "key", "value")
	slog.Info("info message")

	w.Close()
	os.Stdout = orig
	_, _ = buf.ReadFrom(r)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2)

	var debugEntry map[string]interface{}
	require.NoError(t, json.NewDecoder(strings.NewReader(lines[0])).Decode(&debugEntry))
	assert.Equal(t, "debug message", debugEntry["msg"])

	var infoEntry map[string]interface{}
	require.NoError(t, json.NewDecoder(strings.NewReader(lines[1])).Decode(&infoEntry))
	assert.Equal(t, "info message", infoEntry["msg"])
}

func TestInitLogger_Config(t *testing.T) {
	t.Run("JSON with warn level filters info", func(t *testing.T) {
		t.Setenv("LOG_LEVEL", "")

		var buf bytes.Buffer
		orig := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		InitLogger(LoggerConfig{JSON: true, Level: slog.LevelWarn})

		slog.Info("info msg")
		slog.Warn("warn msg")

		w.Close()
		os.Stdout = orig
		_, _ = buf.ReadFrom(r)

		output := buf.String()
		assert.NotContains(t, output, "info msg")
		assert.Contains(t, output, "warn msg")
	})

	t.Run("slogHandler JSON", func(t *testing.T) {
		handler := slogHandler(true, slog.LevelInfo)
		require.IsType(t, &slog.JSONHandler{}, handler)
	})

	t.Run("slogHandler Text", func(t *testing.T) {
		handler := slogHandler(false, slog.LevelInfo)
		require.IsType(t, &slog.TextHandler{}, handler)
	})
}

func TestResolveLevel(t *testing.T) {
	t.Run("LOG_LEVEL=debug", func(t *testing.T) {
		_ = os.Setenv("LOG_LEVEL", "debug")
		defer os.Unsetenv("LOG_LEVEL")
		assert.Equal(t, slog.LevelDebug, resolveLevel(slog.LevelWarn))
	})

	t.Run("LOG_LEVEL=info", func(t *testing.T) {
		_ = os.Setenv("LOG_LEVEL", "info")
		defer os.Unsetenv("LOG_LEVEL")
		assert.Equal(t, slog.LevelInfo, resolveLevel(slog.LevelWarn))
	})

	t.Run("LOG_LEVEL=warn", func(t *testing.T) {
		_ = os.Setenv("LOG_LEVEL", "warn")
		defer os.Unsetenv("LOG_LEVEL")
		assert.Equal(t, slog.LevelWarn, resolveLevel(slog.LevelWarn))
	})

	t.Run("LOG_LEVEL=error", func(t *testing.T) {
		_ = os.Setenv("LOG_LEVEL", "error")
		defer os.Unsetenv("LOG_LEVEL")
		assert.Equal(t, slog.LevelError, resolveLevel(slog.LevelWarn))
	})

	t.Run("LOG_LEVEL invalid falls back to default", func(t *testing.T) {
		_ = os.Setenv("LOG_LEVEL", "invalid")
		defer os.Unsetenv("LOG_LEVEL")
		assert.Equal(t, slog.LevelWarn, resolveLevel(slog.LevelWarn))
	})

	t.Run("no LOG_LEVEL returns default", func(t *testing.T) {
		_ = os.Unsetenv("LOG_LEVEL")
		assert.Equal(t, slog.LevelWarn, resolveLevel(slog.LevelWarn))
	})
}

func TestResolveLogFormat(t *testing.T) {
	t.Run("LOG_FORMAT=json returns json", func(t *testing.T) {
		t.Setenv("LOG_FORMAT", "json")
		assert.Equal(t, "json", resolveLogFormat())
	})

	t.Run("LOG_FORMAT=JSON (uppercase) returns json", func(t *testing.T) {
		t.Setenv("LOG_FORMAT", "JSON")
		assert.Equal(t, "json", resolveLogFormat())
	})

	t.Run("LOG_FORMAT unset returns empty string", func(t *testing.T) {
		t.Setenv("LOG_FORMAT", "")
		assert.Empty(t, resolveLogFormat())
	})

	t.Run("LOG_FORMAT=text returns empty string", func(t *testing.T) {
		t.Setenv("LOG_FORMAT", "text")
		assert.Empty(t, resolveLogFormat())
	})
}

func TestInitLogger_LogFormatEnv_OverridesToJSON(t *testing.T) {
	t.Setenv("LOG_FORMAT", "json")

	var buf bytes.Buffer
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Pass JSON: false to confirm the env var overrides it.
	InitLogger(LoggerConfig{JSON: false, Level: slog.LevelInfo})

	slog.Info("env override test")

	w.Close()
	os.Stdout = orig
	_, _ = buf.ReadFrom(r)

	// If LOG_FORMAT=json took effect, the output must be parseable as JSON.
	var entry map[string]interface{}
	require.NoError(t, json.NewDecoder(&buf).Decode(&entry), "output should be JSON when LOG_FORMAT=json")
	assert.Equal(t, "env override test", entry["msg"])
}

func TestInitLogger_Concurrent(t *testing.T) {
	var buf bytes.Buffer
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	InitLogger(LoggerConfig{JSON: true, Level: slog.LevelInfo})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			slog.Info("concurrent message", "index", n)
		}(i)
	}
	wg.Wait()

	w.Close()
	os.Stdout = orig
	_, _ = buf.ReadFrom(r)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Len(t, lines, 10, "should have 10 log entries")
}
