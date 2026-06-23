package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithSystemContext_IsSystemContext(t *testing.T) {
	ctx := context.Background()
	assert.False(t, IsSystemContext(ctx), "bare context must not be a system context")

	sysCtx := WithSystemContext(ctx)
	assert.True(t, IsSystemContext(sysCtx), "WithSystemContext must mark the context")

	// Original context is unchanged.
	assert.False(t, IsSystemContext(ctx))
}

func TestWithSystemContext_UserIDStillReadable(t *testing.T) {
	ctx := WithSystemContext(WithUserID(context.Background(), "u-1"))
	id, ok := UserIDFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, "u-1", id)
	assert.True(t, IsSystemContext(ctx))
}

func TestWarnIfMissingPrincipal_NoWarningWithUserID(t *testing.T) {
	// Should not panic or error when a user ID is present.
	ctx := WithUserID(context.Background(), "u-1")
	WarnIfMissingPrincipal(ctx) // no panic == pass
}

func TestWarnIfMissingPrincipal_NoWarningWithSystemContext(t *testing.T) {
	// Should not panic or error when the system context marker is present.
	ctx := WithSystemContext(context.Background())
	WarnIfMissingPrincipal(ctx) // no panic == pass
}

func TestWarnIfMissingPrincipal_LogsWhenNeitherPresent(t *testing.T) {
	// Bare context has neither principal; WarnIfMissingPrincipal should not
	// panic. The warning is emitted via slog (not capturable without a custom
	// handler), so we simply assert the call completes without error.
	WarnIfMissingPrincipal(context.Background()) // no panic == pass
}
