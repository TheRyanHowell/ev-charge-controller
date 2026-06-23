package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	var id string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id = GetRequestID(r.Context())
	})

	handler := RequestIDMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.NotEmpty(t, id)
	assert.True(t, strings.HasPrefix(id, "req-"))
	assert.NotEmpty(t, rr.Header().Get(RequestIDHeader))
}

func TestRequestIDMiddleware_PreservesExistingID(t *testing.T) {
	var receivedID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedID = GetRequestID(r.Context())
	})

	handler := RequestIDMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, "external-123")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "external-123", receivedID)
	assert.Equal(t, "external-123", rr.Header().Get(RequestIDHeader))
}

func TestRequestIDMiddleware_RejectsMaliciousID(t *testing.T) {
	malicious := []string{
		"inj\nFAKE LOG LINE",            // newline log injection
		"abc\rdef",                      // carriage return
		"id with spaces",                // space
		strings.Repeat("a", 65),         // too long
		"semi;colon",                    // disallowed punctuation
		"<script>",                      // angle brackets
	}
	for _, bad := range malicious {
		t.Run(bad, func(t *testing.T) {
			var receivedID string
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				receivedID = GetRequestID(r.Context())
			})
			handler := RequestIDMiddleware(next)
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set(RequestIDHeader, bad)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.NotEqual(t, bad, receivedID, "malicious ID must be replaced")
			assert.True(t, strings.HasPrefix(receivedID, "req-"))
			assert.Equal(t, receivedID, rr.Header().Get(RequestIDHeader))
		})
	}
}

func TestRequestIDMiddleware_UniqueIDs(t *testing.T) {
	var ids []string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids = append(ids, GetRequestID(r.Context()))
	})

	handler := RequestIDMiddleware(next)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}

	assert.Len(t, ids, 10)
	for i := 1; i < len(ids); i++ {
		assert.NotEqual(t, ids[i-1], ids[i])
	}
}

func TestGetRequestID_EmptyContext(t *testing.T) {
	assert.Empty(t, GetRequestID(context.Background()))
}
