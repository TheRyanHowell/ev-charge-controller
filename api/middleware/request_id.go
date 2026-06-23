package middleware

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sync/atomic"
)

type ctxKey struct{}

// RequestIDHeader is the header name for the request ID.
const RequestIDHeader = "X-Request-ID"

// validRequestID constrains an inbound, client-supplied request ID to a safe
// character set and length. The ID is echoed in a response header and written
// to structured logs, so an unvalidated value is a log-injection / header-
// injection vector. Anything not matching is replaced with a generated ID.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// GetRequestID returns the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(ctxKey{}); v != nil {
		s, ok := v.(string)
		if !ok {
			return ""
		}
		return s
	}
	return ""
}

// requestIDCounter is a monotonically increasing counter for request IDs.
var requestIDCounter atomic.Uint64

// RequestIDMiddleware generates a unique request ID for each request,
// sets it on the context, and includes it in the response header.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if !validRequestID.MatchString(id) {
			id = genRequestID()
		}

		ctx := context.WithValue(r.Context(), ctxKey{}, id)
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func genRequestID() string {
	n := requestIDCounter.Add(1)
	return fmt.Sprintf("req-%016x", n)
}
