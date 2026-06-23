package repository

import (
	"context"
	"testing"
	"time"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ctxWithUser injects a user ID into context the same way auth middleware does.
func ctxWithUser(userID string) context.Context {
	return internal.WithUserID(context.Background(), userID)
}

func setupVehicleTestDB(t *testing.T) *VehicleRepository {
	t.Helper()
	db, err := database.SetupTestDB(true) // seed populates vehicle_models
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	// Pre-seed test users so vehicles.user_id FK constraints pass.
	_, err = db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash) VALUES ('user-1', 'u1@repo.com', ''), ('user-2', 'u2@repo.com', '')`)
	require.NoError(t, err)
	return NewVehicleRepository(db)
}

func TestVehicleRepository_CreateAndFind(t *testing.T) {
	repo := setupVehicleTestDB(t)
	userID := "user-1"

	v := &models.Vehicle{
		ModelID:        "rm1",
		UserID:         &userID,
		Name:           "My RM1",
		CurrentPercent: 25.0,
		TargetPercent:  80.0,
	}
	require.NoError(t, repo.CreateInstance(t.Context(), v))
	assert.NotEmpty(t, v.ID)

	found, err := repo.FindByID(t.Context(), v.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "My RM1", found.Name)
	assert.Equal(t, "rm1", found.ModelID)
	assert.InDelta(t, 2.026, found.CapacityKwh, 0.001)
	assert.Equal(t, 0.8, found.ChargingEfficiency)
	assert.Equal(t, 25.0, found.CurrentPercent)
	assert.Equal(t, 80.0, found.TargetPercent)
}

func TestVehicleRepository_FindByID_Missing(t *testing.T) {
	repo := setupVehicleTestDB(t)

	found, err := repo.FindByID(t.Context(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestVehicleRepository_List_UserScoped(t *testing.T) {
	repo := setupVehicleTestDB(t)
	user1 := "user-1"
	user2 := "user-2"

	require.NoError(t, repo.CreateInstance(t.Context(), &models.Vehicle{ModelID: "rm1", UserID: &user1, Name: "U1 RM1"}))
	require.NoError(t, repo.CreateInstance(t.Context(), &models.Vehicle{ModelID: "rm2", UserID: &user1, Name: "U1 RM2"}))
	require.NoError(t, repo.CreateInstance(t.Context(), &models.Vehicle{ModelID: "rm1s", UserID: &user2, Name: "U2 RM1S"}))

	list, err := repo.List(ctxWithUser(user1))
	require.NoError(t, err)
	assert.Len(t, list, 2)

	list2, err := repo.List(ctxWithUser(user2))
	require.NoError(t, err)
	assert.Len(t, list2, 1)
	assert.Equal(t, "U2 RM1S", list2[0].Name)
}

func TestVehicleRepository_List_CatalogMerged(t *testing.T) {
	repo := setupVehicleTestDB(t)
	userID := "user-1"
	require.NoError(t, repo.CreateInstance(t.Context(), &models.Vehicle{
		ModelID: "rm2", UserID: &userID, Name: "My RM2",
	}))

	list, err := repo.List(ctxWithUser(userID))
	require.NoError(t, err)
	require.Len(t, list, 1)
	v := list[0]
	assert.InDelta(t, 5.46, v.CapacityKwh, 0.01)
	assert.Equal(t, 1200.0, v.ChargerOutputW)
}

func TestVehicleRepository_UpdatePercents(t *testing.T) {
	repo := setupVehicleTestDB(t)
	userID := "user-1"
	v := &models.Vehicle{ModelID: "rm1", UserID: &userID, Name: "test", CurrentPercent: 20, TargetPercent: 80}
	require.NoError(t, repo.CreateInstance(t.Context(), v))

	require.NoError(t, repo.UpdatePercents(t.Context(), v.ID, 50.0, 75.0))

	found, err := repo.FindByID(t.Context(), v.ID)
	require.NoError(t, err)
	assert.Equal(t, 50.0, found.CurrentPercent)
	assert.Equal(t, 75.0, found.TargetPercent)
}

func TestVehicleRepository_DeleteInstance(t *testing.T) {
	repo := setupVehicleTestDB(t)
	userID := "user-1"
	wrongUser := "user-2"

	v := &models.Vehicle{ModelID: "rm1", UserID: &userID, Name: "to delete", CreatedAt: time.Now()}
	require.NoError(t, repo.CreateInstance(t.Context(), v))

	// Wrong user - should silently do nothing
	require.NoError(t, repo.DeleteInstance(t.Context(), v.ID, wrongUser))
	found, err := repo.FindByID(t.Context(), v.ID)
	require.NoError(t, err)
	assert.NotNil(t, found)

	// Correct user - removes the row
	require.NoError(t, repo.DeleteInstance(t.Context(), v.ID, userID))
	found, err = repo.FindByID(t.Context(), v.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestVehicleRepository_FindByIDs(t *testing.T) {
	repo := setupVehicleTestDB(t)
	userID := "user-1"

	v1 := &models.Vehicle{ModelID: "rm1", UserID: &userID, Name: "A"}
	v2 := &models.Vehicle{ModelID: "rm1s", UserID: &userID, Name: "B"}
	require.NoError(t, repo.CreateInstance(t.Context(), v1))
	require.NoError(t, repo.CreateInstance(t.Context(), v2))

	result, err := repo.FindByIDs(t.Context(), []string{v1.ID, v2.ID})
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "A", result[v1.ID].Name)
	assert.Equal(t, "B", result[v2.ID].Name)

	empty, err := repo.FindByIDs(t.Context(), []string{})
	require.NoError(t, err)
	assert.Len(t, empty, 0)
}

func TestVehicleRepository_PackSpecs(t *testing.T) {
	repo := setupVehicleTestDB(t)
	userID := "user-1"
	require.NoError(t, repo.CreateInstance(t.Context(), &models.Vehicle{
		ModelID: "rm1", UserID: &userID, Name: "RM1 specs",
	}))

	list, err := repo.List(ctxWithUser(userID))
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NotNil(t, list[0].PackVoltageMaxV)
	require.NotNil(t, list[0].PackCutoffCurrentMa)
	assert.InDelta(t, 58.8, *list[0].PackVoltageMaxV, 0.01)
	assert.InDelta(t, 600.0, *list[0].PackCutoffCurrentMa, 0.1)
}

func TestVehicleRepository_DuplicateModel(t *testing.T) {
	repo := setupVehicleTestDB(t)
	userID := "user-1"

	v1 := &models.Vehicle{ModelID: "rm1", UserID: &userID, Name: "My first RM1"}
	v2 := &models.Vehicle{ModelID: "rm1", UserID: &userID, Name: "My second RM1"}
	require.NoError(t, repo.CreateInstance(t.Context(), v1))
	require.NoError(t, repo.CreateInstance(t.Context(), v2))

	list, err := repo.List(ctxWithUser(userID))
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestVehicleRepository_UpdateName(t *testing.T) {
	repo := setupVehicleTestDB(t)
	userID := "user-1"

	v := &models.Vehicle{ModelID: "rm1", UserID: &userID, Name: "Old Name"}
	require.NoError(t, repo.CreateInstance(t.Context(), v))

	require.NoError(t, repo.UpdateName(t.Context(), v.ID, "New Name", userID))

	found, err := repo.FindByID(t.Context(), v.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Name", found.Name)
}

func TestVehicleRepository_UpdateName_WrongUser(t *testing.T) {
	repo := setupVehicleTestDB(t)
	userID := "user-1"
	wrongUser := "user-2"

	v := &models.Vehicle{ModelID: "rm1", UserID: &userID, Name: "Original"}
	require.NoError(t, repo.CreateInstance(t.Context(), v))

	require.NoError(t, repo.UpdateName(t.Context(), v.ID, "Hacked", wrongUser))

	found, err := repo.FindByID(t.Context(), v.ID)
	require.NoError(t, err)
	assert.Equal(t, "Original", found.Name)
}

func TestVehicleRepository_IncrementLifetimeStats(t *testing.T) {
	repo := setupVehicleTestDB(t)
	userID := "user-1"
	sessionAt := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	v := &models.Vehicle{ModelID: "rm1", UserID: &userID}
	require.NoError(t, repo.CreateInstance(t.Context(), v))

	batteryKwh := 5.5
	wallKwh := 6.875
	co2Grams := 1234.5

	require.NoError(t, repo.IncrementLifetimeStats(t.Context(), v.ID, batteryKwh, wallKwh, co2Grams, 0, sessionAt))

	found, err := repo.FindByID(t.Context(), v.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, found.TotalSessions)
	assert.InDelta(t, batteryKwh, found.TotalBatteryKwh, 0.001)
	assert.InDelta(t, wallKwh, found.TotalWallKwh, 0.001)
	assert.InDelta(t, co2Grams, found.TotalCo2Grams, 0.001)
	assert.InDelta(t, batteryKwh, found.MinSessionBatteryKwh, 0.001)
	assert.InDelta(t, batteryKwh, found.MaxSessionBatteryKwh, 0.001)
	require.NotNil(t, found.LastSessionAt)
	assert.WithinDuration(t, sessionAt, *found.LastSessionAt, time.Second)
}

func TestVehicleRepository_IncrementLifetimeStats_Accumulates(t *testing.T) {
	repo := setupVehicleTestDB(t)
	userID := "user-1"
	sessionAt := time.Now()

	v := &models.Vehicle{ModelID: "rm1", UserID: &userID}
	require.NoError(t, repo.CreateInstance(t.Context(), v))

	require.NoError(t, repo.IncrementLifetimeStats(t.Context(), v.ID, 5.0, 6.25, 1000, 0, sessionAt))
	require.NoError(t, repo.IncrementLifetimeStats(t.Context(), v.ID, 3.0, 3.75, 600, 0, sessionAt))

	found, err := repo.FindByID(t.Context(), v.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, found.TotalSessions)
	assert.InDelta(t, 8.0, found.TotalBatteryKwh, 0.001)
	assert.InDelta(t, 10.0, found.TotalWallKwh, 0.001)
	assert.InDelta(t, 1600.0, found.TotalCo2Grams, 0.001)
	assert.InDelta(t, 3.0, found.MinSessionBatteryKwh, 0.001)
	assert.InDelta(t, 5.0, found.MaxSessionBatteryKwh, 0.001)
}

func TestVehicleRepository_DecrementLifetimeStats(t *testing.T) {
	repo := setupVehicleTestDB(t)
	userID := "user-1"
	sessionAt := time.Now()

	v := &models.Vehicle{ModelID: "rm1", UserID: &userID}
	require.NoError(t, repo.CreateInstance(t.Context(), v))

	// Simulate 2 sessions
	require.NoError(t, repo.IncrementLifetimeStats(t.Context(), v.ID, 10.0, 12.5, 2000, 0, sessionAt))
	require.NoError(t, repo.IncrementLifetimeStats(t.Context(), v.ID, 5.0, 6.25, 1000, 0, sessionAt))

	// Delete one session (subtract its stats); min/max are unchanged (stale but acceptable)
	require.NoError(t, repo.DecrementLifetimeStats(t.Context(), v.ID, 5.0, 6.25, 1000, 0))

	found, err := repo.FindByID(t.Context(), v.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, found.TotalSessions)
	assert.InDelta(t, 10.0, found.TotalBatteryKwh, 0.001)
	assert.InDelta(t, 12.5, found.TotalWallKwh, 0.001)
	assert.InDelta(t, 2000.0, found.TotalCo2Grams, 0.001)
	// min/max remain at their pre-decrement values since N>1 sessions remain
	assert.InDelta(t, 5.0, found.MinSessionBatteryKwh, 0.001)
	assert.InDelta(t, 10.0, found.MaxSessionBatteryKwh, 0.001)
}

func TestVehicleRepository_DecrementLifetimeStats_ClampsToZero(t *testing.T) {
	repo := setupVehicleTestDB(t)
	userID := "user-1"

	v := &models.Vehicle{ModelID: "rm1", UserID: &userID}
	require.NoError(t, repo.CreateInstance(t.Context(), v))

	// Start at 0, decrement - should clamp to 0, not go negative
	require.NoError(t, repo.DecrementLifetimeStats(t.Context(), v.ID, 5.0, 6.25, 1000, 0))

	found, err := repo.FindByID(t.Context(), v.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, found.TotalSessions)
	assert.Equal(t, 0.0, found.TotalBatteryKwh)
	assert.Equal(t, 0.0, found.TotalWallKwh)
	assert.Equal(t, 0.0, found.TotalCo2Grams)
	assert.Equal(t, 0.0, found.MinSessionBatteryKwh)
	assert.Equal(t, 0.0, found.MaxSessionBatteryKwh)
}
