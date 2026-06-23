package handlers

import (
	"context"
	"database/sql"
	"testing"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/services"
	"ev-charge-controller/api/testdb"

	"github.com/stretchr/testify/require"
)

func setupHandlerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	// Seed default user, plug, and vehicles for test-user.
	testdb.SeedFullTestDB(t, db)
	// Also seed additional users (u1, u2) that some tests reference.
	require.NoError(t, testdb.InsertUser(db, testdb.UserID2, "u1@test.com", ""))
	require.NoError(t, testdb.InsertUser(db, testdb.UserID3, "u2@test.com", ""))
	return db
}

func newTestChargeSessionHandler(t *testing.T, db *sql.DB) *ChargeSessionHandler {
	service := services.NewChargeSessionService(
		context.Background(),
		repository.NewChargeSessionRepository(db),
		repository.NewVehicleRepository(db),
		repository.NewPlugRepository(db),
		nil,
		nil,
		nil,
	)
	return NewChargeSessionHandler(service)
}
