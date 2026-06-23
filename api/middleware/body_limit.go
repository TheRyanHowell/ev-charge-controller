package middleware

import "net/http"

// MaxBodyBytes caps the size of an accepted request body (1 MiB). All payloads
// in this API (session/target/push/schedule) are tiny; the cap bounds memory
// use and rejects oversized or abusive bodies before a handler reads them.
const MaxBodyBytes = 1 << 20

// MaxHeaderBytes caps the size of accepted request headers (1 MiB) on the
// http.Server. Exported so server construction and tests share one value.
const MaxHeaderBytes = 1 << 20

// BodyLimitMiddleware wraps each request body in an http.MaxBytesReader so a
// handler that reads it gets an error (and the connection is closed) once the
// limit is exceeded, rather than buffering an unbounded body.
func BodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}
