package internal

import "context"

type userIDKey struct{}

// WithUserID returns a new context with userID attached.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// UserIDFromContext extracts the authenticated user ID. Returns ("", false) if not set.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey{}).(string)
	return id, ok && id != ""
}
