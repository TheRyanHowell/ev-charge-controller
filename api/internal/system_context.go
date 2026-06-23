package internal

import (
	"context"
	"log/slog"
)

type systemContextKey struct{}

// WithSystemContext marks ctx as a system-initiated call (background workers,
// MQTT handlers) that legitimately needs cross-user repository access.
// Repository helpers (andUserClause, whereUserClause) use this marker to
// distinguish authorised system access from a handler that accidentally bypassed
// auth middleware.
//
// All background goroutines (energy poller, auto-stop checker, schedule
// activator, MQTT dispatcher) must receive a system context so that their
// repository queries can skip the per-user filter without triggering the
// missing-principal warning.
func WithSystemContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, systemContextKey{}, true)
}

// IsSystemContext reports whether ctx was created by WithSystemContext.
func IsSystemContext(ctx context.Context) bool {
	v, _ := ctx.Value(systemContextKey{}).(bool)
	return v
}

// WarnIfMissingPrincipal logs an error-level warning when a repository clause
// is evaluated with a context that has neither a user ID nor a system marker.
// This catches handlers that were accidentally registered without the auth
// middleware - data is not filtered out today (to avoid breaking existing
// tests), but the log message is actionable evidence of a routing mistake.
//
// TODO: Once all tests set either WithUserID or WithSystemContext on their
// contexts, replace this warning with a deny-all clause ("AND 1=0") so that
// missing-principal contexts fail closed instead of silently returning
// unfiltered rows.
func WarnIfMissingPrincipal(ctx context.Context) {
	if UserIDFromContext2(ctx) || IsSystemContext(ctx) {
		return
	}
	slog.Error("[security] repository query has neither user nor system context - possible missing auth middleware",
		"callerHint", "check registerRoutes for a missing protect() wrapper")
}

// UserIDFromContext2 is like UserIDFromContext but returns a plain bool
// (used internally to avoid importing the full two-value form).
func UserIDFromContext2(ctx context.Context) bool {
	_, ok := UserIDFromContext(ctx)
	return ok
}
