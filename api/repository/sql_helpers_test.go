package repository

import (
	"context"
	"testing"

	"ev-charge-controller/api/internal"

	"github.com/stretchr/testify/assert"
)

func TestAndUserClause_WithUserID(t *testing.T) {
	ctx := internal.WithUserID(context.Background(), "user-abc")
	clause, args := andUserClause(ctx)
	assert.Equal(t, " AND user_id = ?", clause)
	assert.Equal(t, []any{"user-abc"}, args)
}

func TestAndUserClause_WithSystemContext(t *testing.T) {
	ctx := internal.WithSystemContext(context.Background())
	clause, args := andUserClause(ctx)
	assert.Empty(t, clause, "system context must produce no filter clause")
	assert.Empty(t, args)
}

func TestAndUserClause_NeitherPrincipal(t *testing.T) {
	// A bare context (no user ID, no system marker) currently produces no
	// filter and logs a warning. Behaviour is tested here to lock in the
	// contract: we do NOT deny-all yet, but the clause is empty (see TODO in
	// andUserClause for the planned upgrade path).
	clause, args := andUserClause(context.Background())
	assert.Empty(t, clause)
	assert.Empty(t, args)
}

func TestWhereUserClause_WithUserID(t *testing.T) {
	ctx := internal.WithUserID(context.Background(), "user-xyz")
	clause, args := whereUserClause(ctx)
	assert.Equal(t, " WHERE user_id = ?", clause)
	assert.Equal(t, []any{"user-xyz"}, args)
}

func TestWhereUserClause_WithSystemContext(t *testing.T) {
	ctx := internal.WithSystemContext(context.Background())
	clause, args := whereUserClause(ctx)
	assert.Empty(t, clause)
	assert.Empty(t, args)
}

