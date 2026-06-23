package repository

import (
	"context"
	"strings"

	"ev-charge-controller/api/internal"
)

// buildPlaceholders returns a comma-separated list of n SQL bind placeholders
// ("?, ?, ?") for use in an IN (...) clause. Returns "" when n <= 0.
func buildPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// toAnySlice converts a []string to []any for variadic query argument passing.
func toAnySlice(values []string) []any {
	args := make([]any, len(values))
	for i, v := range values {
		args[i] = v
	}
	return args
}

// andUserClause appends " AND user_id = ?" when a user ID is in ctx.
// When the context carries a system marker (WithSystemContext), the clause is
// omitted intentionally - background workers need cross-user access.
// When neither a user ID nor a system marker is present, a warning is logged
// (see WarnIfMissingPrincipal) and no filter is applied. This keeps existing
// behaviour but makes the security gap visible in logs.
//
// TODO: once all tests use either WithUserID or WithSystemContext, replace the
// no-filter fallback with a deny-all clause (" AND 1=0") so that
// unauthenticated handler contexts fail closed.
func andUserClause(ctx context.Context) (string, []any) {
	userID, ok := internal.UserIDFromContext(ctx)
	if ok {
		return " AND user_id = ?", []any{userID}
	}
	internal.WarnIfMissingPrincipal(ctx)
	return "", nil
}

// whereUserClause returns " WHERE user_id = ?" when a user ID is in ctx.
// See andUserClause for the system-context and missing-principal behaviour.
func whereUserClause(ctx context.Context) (string, []any) {
	userID, ok := internal.UserIDFromContext(ctx)
	if ok {
		return " WHERE user_id = ?", []any{userID}
	}
	internal.WarnIfMissingPrincipal(ctx)
	return "", nil
}
