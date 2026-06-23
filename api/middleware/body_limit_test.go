package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBodyLimitMiddleware_PassThrough(t *testing.T) {
	handler := BodyLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))

	body := strings.NewReader(`{"key": "value"}`)
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, `{"key": "value"}`, rec.Body.String())
}

func TestBodyLimitMiddleware_NilBody(t *testing.T) {
	called := false
	handler := BodyLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBodyLimitMiddleware_ExceedsLimit(t *testing.T) {
	handler := BodyLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Create body larger than MaxBodyBytes (1 MiB)
	largeBody := strings.NewReader(strings.Repeat("x", MaxBodyBytes+1))
	req := httptest.NewRequest(http.MethodPost, "/test", largeBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestMaxBodyBytes_Value(t *testing.T) {
	assert.Equal(t, 1<<20, MaxBodyBytes)
}

func TestMaxHeaderBytes_Value(t *testing.T) {
	assert.Equal(t, 1<<20, MaxHeaderBytes)
}
