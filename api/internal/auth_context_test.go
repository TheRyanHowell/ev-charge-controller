package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithUserID_And_UserIDFromContext(t *testing.T) {
	ctx := context.Background()

	_, ok := UserIDFromContext(ctx)
	assert.False(t, ok)

	ctx = WithUserID(ctx, "user-123")
	id, ok := UserIDFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, "user-123", id)
}

func TestUserIDFromContext_EmptyString(t *testing.T) {
	ctx := context.Background()
	ctx = WithUserID(ctx, "")

	_, ok := UserIDFromContext(ctx)
	assert.False(t, ok)
}

func TestUserIDFromContext_NonStringValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), userIDKey{}, 123)

	_, ok := UserIDFromContext(ctx)
	assert.False(t, ok)
}

func TestWithUserID_Chained(t *testing.T) {
	ctx := context.Background()
	ctx1 := WithUserID(ctx, "user-1")
	ctx2 := WithUserID(ctx1, "user-2")

	id1, ok1 := UserIDFromContext(ctx1)
	id2, ok2 := UserIDFromContext(ctx2)

	assert.True(t, ok1)
	assert.Equal(t, "user-1", id1)
	assert.True(t, ok2)
	assert.Equal(t, "user-2", id2)

	// Original context unchanged
	_, okOrig := UserIDFromContext(ctx)
	assert.False(t, okOrig)
}
