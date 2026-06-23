package services

import (
	"database/sql"
	"sync"
	"testing"
	"time"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/testdb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupVehicleServiceTestDB(t *testing.T) *sql.DB {
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	testdb.SeedFullTestDB(t, db)
	require.NoError(t, testdb.InsertUser(db, "u1", "u1@test.com", ""))
	require.NoError(t, testdb.InsertUser(db, "u2", "u2@test.com", ""))
	return db
}

func newVehicleService(db *sql.DB) *VehicleService {
	return NewVehicleService(
		repository.NewVehicleRepository(db),
		repository.NewVehicleModelRepository(db),
		repository.NewChargeSessionRepository(db),
		newSessionLock(),
	)
}

func insertTestVehicle(t *testing.T, db *sql.DB, id, userID, modelID string, curPct, tgtPct float64) {
	t.Helper()
	require.NoError(t, testdb.InsertVehicle(db, id, userID, modelID, modelID+"-"+id, curPct, tgtPct))
}

func ptr(f float64) *float64 { return &f }

func TestVehicleService_FindByID_EmptyDB(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	_, err := service.FindByID(t.Context(), "any-id")
	assert.ErrorIs(t, err, ErrVehicleNotFound)
}

func TestVehicleService_List_MultipleVehicles(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	userID := "u1"
	insertTestVehicle(t, db, "v1", userID, "rm1", 20, 80)
	insertTestVehicle(t, db, "v2", userID, "rm1s", 20, 80)
	insertTestVehicle(t, db, "v3", userID, "rm2", 20, 80)

	ctx := internal.WithUserID(t.Context(), userID)
	vehicles, err := service.List(ctx)
	require.NoError(t, err)
	assert.Len(t, vehicles, 3)
}

func TestVehicleService_List_IgnoresChargeSessions(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	userID := "u1"
	insertTestVehicle(t, db, "v1", userID, "rm1", 20, 80)
	createdAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endedAt := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "cs1",
		VehicleID: "v1",
		UserID:    "test-user",
		PlugID:    "test-plug",
		Status:    "completed",
		CreatedAt: createdAt,
		EndedAt:   &endedAt,
		StartKwh:  0.4,
		EndKwh:    ptr(1.6),
		TargetKwh: 1.6,
		StartPct:  20,
		EndPct:    ptr(80),
		TargetPct: 80,
	}))

	ctx := internal.WithUserID(t.Context(), userID)
	vehicles, err := service.List(ctx)
	require.NoError(t, err)
	assert.Len(t, vehicles, 1)
	assert.Equal(t, "v1", vehicles[0].ID)
}

func TestVehicleService_FindByID_ByTimestamp(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	now := time.Now().UTC().Format(time.DateTime)
	_, err := db.Exec(
		`INSERT INTO vehicles (id, user_id, model_id, name, current_percent, target_percent, created_at)
		 VALUES ('vid-ts', 'u1', 'rm1', 'RM1', 20, 80, ?)`, now)
	require.NoError(t, err)

	vehicle, err := service.FindByID(t.Context(), "vid-ts")
	require.NoError(t, err)
	require.NotNil(t, vehicle)
	assert.Equal(t, now, vehicle.CreatedAt.Format(time.DateTime))
}

func TestVehicleService_NullTimeFields(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	insertTestVehicle(t, db, "v1", "u1", "rm1", 20, 80)
	vehicle, err := service.FindByID(t.Context(), "v1")
	require.NoError(t, err)
	require.NotNil(t, vehicle)
	assert.Equal(t, "v1", vehicle.ID)
	assert.Equal(t, "rm1", vehicle.ModelID)
}

func TestVehicleService_UpdatePercents_BothFields(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	insertTestVehicle(t, db, "v1", "u1", "rm1", 20, 80)
	require.NoError(t, service.UpdatePercents(t.Context(), "v1", ptr(50.0), ptr(75.0)))

	v, err := service.FindByID(t.Context(), "v1")
	require.NoError(t, err)
	assert.Equal(t, 50.0, v.CurrentPercent)
	assert.Equal(t, 75.0, v.TargetPercent)
}

func TestVehicleService_UpdatePercents_TargetOnly(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	insertTestVehicle(t, db, "v1", "u1", "rm1", 40, 80)
	require.NoError(t, service.UpdatePercents(t.Context(), "v1", nil, ptr(90.0)))

	v, err := service.FindByID(t.Context(), "v1")
	require.NoError(t, err)
	assert.Equal(t, 40.0, v.CurrentPercent, "current should be unchanged")
	assert.Equal(t, 90.0, v.TargetPercent)
}

func TestVehicleService_UpdatePercents_CurrentOnly(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	insertTestVehicle(t, db, "v1", "u1", "rm1", 20, 80)
	require.NoError(t, service.UpdatePercents(t.Context(), "v1", ptr(35.0), nil))

	v, err := service.FindByID(t.Context(), "v1")
	require.NoError(t, err)
	assert.Equal(t, 35.0, v.CurrentPercent)
	assert.Equal(t, 80.0, v.TargetPercent, "target should be unchanged")
}

func TestVehicleService_UpdatePercents_CurrentRejectedDuringSession(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	insertTestVehicle(t, db, "v1", "u1", "rm1", 20, 80)
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "cs1",
		VehicleID: "v1",
		UserID:    "test-user",
		PlugID:    "test-plug",
		Status:    "active",
		CreatedAt: time.Now(),
		StartKwh:  0.4,
		TargetKwh: 1.6,
		StartPct:  20,
		TargetPct: 80,
	}))

	assert.ErrorIs(t, service.UpdatePercents(t.Context(), "v1", ptr(50.0), nil), ErrCurrentLockedDuringSession)
}

func TestVehicleService_UpdatePercents_TargetAllowedDuringSession(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	insertTestVehicle(t, db, "v1", "u1", "rm1", 20, 80)
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "cs1",
		VehicleID: "v1",
		UserID:    "test-user",
		PlugID:    "test-plug",
		Status:    "active",
		CreatedAt: time.Now(),
		StartKwh:  0.4,
		TargetKwh: 1.6,
		StartPct:  20,
		TargetPct: 80,
	}))

	require.NoError(t, service.UpdatePercents(t.Context(), "v1", nil, ptr(90.0)))
}

func TestVehicleService_UpdatePercents_Validation(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	insertTestVehicle(t, db, "v1", "u1", "rm1", 20, 80)

	cases := []struct{ cur, tgt *float64 }{
		{ptr(-1), ptr(50)},
		{ptr(101), ptr(50)},
		{ptr(50), ptr(-1)},
		{ptr(50), ptr(101)},
		{ptr(80), ptr(50)},
	}
	for _, c := range cases {
		assert.Error(t, service.UpdatePercents(t.Context(), "v1", c.cur, c.tgt))
	}
	assert.NoError(t, service.UpdatePercents(t.Context(), "v1", ptr(50), ptr(50)))
}

// TestVehicleService_UpdatePercents_ConcurrentWrites verifies that the session
// lock serialises concurrent percent writes: every call succeeds (no torn write
// or "database is locked"), the stored value is one of the written values, and
// the current<=target invariant always holds. Run with -race to catch data races.
func TestVehicleService_UpdatePercents_ConcurrentWrites(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)
	insertTestVehicle(t, db, "v1", "u1", "rm1", 20, 80)

	const writers = 16
	targets := make([]float64, writers)
	var wg sync.WaitGroup
	errs := make([]error, writers)
	wg.Add(writers)
	for i := range writers {
		targets[i] = float64(50 + i) // 50..65, all >= current (20)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = service.UpdatePercents(t.Context(), "v1", nil, ptr(targets[idx]))
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "writer %d", i)
	}

	v, err := service.FindByID(t.Context(), "v1")
	require.NoError(t, err)
	assert.Contains(t, targets, v.TargetPercent, "final target must be one of the writes")
	assert.LessOrEqual(t, v.CurrentPercent, v.TargetPercent)
}

// TestVehicleService_UpdatePercents_UsesInjectedLock proves the write runs under
// the injected lock by holding that lock from the test and asserting the call
// cannot proceed until it is released.
func TestVehicleService_UpdatePercents_UsesInjectedLock(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	lock := &sync.Mutex{}
	service := NewVehicleService(
		repository.NewVehicleRepository(db),
		repository.NewVehicleModelRepository(db),
		repository.NewChargeSessionRepository(db),
		lock,
	)
	insertTestVehicle(t, db, "v1", "u1", "rm1", 20, 80)

	lock.Lock()
	done := make(chan error, 1)
	go func() {
		done <- service.UpdatePercents(t.Context(), "v1", nil, ptr(90.0))
	}()

	select {
	case <-done:
		t.Fatal("UpdatePercents proceeded while the lock was held")
	case <-time.After(50 * time.Millisecond):
		// Expected: blocked on the lock.
	}

	lock.Unlock()
	require.NoError(t, <-done)

	v, err := service.FindByID(t.Context(), "v1")
	require.NoError(t, err)
	assert.Equal(t, 90.0, v.TargetPercent)
}

func TestVehicleService_resolvePercents_ActiveSession(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	insertTestVehicle(t, db, "v1", "u1", "rm1", 20, 80)
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:           "cs1",
		VehicleID:    "v1",
		UserID:       "test-user",
		PlugID:       "test-plug",
		Status:       "active",
		CreatedAt:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		StartKwh:     0.6,
		TargetKwh:    1.4,
		StartPct:     30,
		TargetPct:    70,
		StartTotalKwh: ptr(0.6),
	}))

	current, target := service.resolvePercents(t.Context(), "v1")
	assert.Equal(t, 30.0, current)
	assert.Equal(t, 70.0, target)
}

func TestVehicleService_resolvePercents_NoActiveSession(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	insertTestVehicle(t, db, "v1", "u1", "rm1", 35.0, 75.0)
	createdAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endedAt := time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "cs1",
		VehicleID: "v1",
		UserID:    "test-user",
		PlugID:    "test-plug",
		Status:    "completed",
		CreatedAt: createdAt,
		EndedAt:   &endedAt,
		StartKwh:  0.4,
		TargetKwh: 1.6,
		StartPct:  20,
		EndPct:    ptr(80),
		TargetPct: 80,
	}))

	current, target := service.resolvePercents(t.Context(), "v1")
	assert.Equal(t, 35.0, current)
	assert.Equal(t, 75.0, target)
}

func TestVehicleService_resolvePercents_VehicleNotFound(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	current, target := service.resolvePercents(t.Context(), "nonexistent")
	assert.Equal(t, float64(models.DefaultCurrentPercent), current)
	assert.Equal(t, float64(models.DefaultTargetPercent), target)
}

func TestVehicleService_AddVehicle(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	v, err := service.AddVehicle(t.Context(), "u1", "rm1", "My Custom RM1")
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, "My Custom RM1", v.Name)
	assert.Equal(t, "rm1", v.ModelID)
	assert.InDelta(t, 2.026, v.CapacityKwh, 0.001)
}

func TestVehicleService_AddVehicle_DefaultName(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	v, err := service.AddVehicle(t.Context(), "u1", "rm2", "")
	require.NoError(t, err)
	assert.Equal(t, "Maeving RM2", v.Name)
}

func TestVehicleService_AddVehicle_ModelNotFound(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	_, err := service.AddVehicle(t.Context(), "u1", "nonexistent", "")
	assert.ErrorIs(t, err, ErrVehicleModelNotFound)
}

func TestVehicleService_AddVehicle_DuplicateModelAllowed(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	v1, err := service.AddVehicle(t.Context(), "u1", "rm1", "First")
	require.NoError(t, err)
	v2, err := service.AddVehicle(t.Context(), "u1", "rm1", "Second")
	require.NoError(t, err)
	assert.NotEqual(t, v1.ID, v2.ID)
}

func TestVehicleService_DeleteVehicle(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	v, err := service.AddVehicle(t.Context(), "u1", "rm1", "To Delete")
	require.NoError(t, err)
	require.NoError(t, service.DeleteVehicle(t.Context(), "u1", v.ID))
	_, err = service.FindByID(t.Context(), v.ID)
	assert.ErrorIs(t, err, ErrVehicleNotFound)
}

func TestVehicleService_DeleteVehicle_WrongUser(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	v, err := service.AddVehicle(t.Context(), "u1", "rm1", "Mine")
	require.NoError(t, err)
	assert.ErrorIs(t, service.DeleteVehicle(t.Context(), "u2", v.ID), ErrVehicleNotFound)
}

func TestVehicleService_ListModels(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	mods, err := service.ListModels(t.Context())
	require.NoError(t, err)
	assert.Len(t, mods, 3)
}

func TestVehicleService_UpdateName_Success(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	insertTestVehicle(t, db, "v1", "u1", "rm1", 20, 80)
	require.NoError(t, service.UpdateName(t.Context(), "v1", "New Name"))

	v, err := service.FindByID(t.Context(), "v1")
	require.NoError(t, err)
	assert.Equal(t, "New Name", v.Name)
}

func TestVehicleService_UpdateName_Duplicate(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	insertTestVehicle(t, db, "v1", "u1", "rm1", 20, 80)
	insertTestVehicle(t, db, "v2", "u1", "rm2", 20, 80)

	assert.ErrorIs(t, service.UpdateName(t.Context(), "v1", "rm2-v2"), ErrDuplicateVehicleName)
}

func TestVehicleService_UpdateName_NotFound(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	assert.ErrorIs(t, service.UpdateName(t.Context(), "nonexistent", "Name"), ErrVehicleNotFound)
}

func TestVehicleService_UpdateName_Empty(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	insertTestVehicle(t, db, "v1", "u1", "rm1", 20, 80)
	assert.ErrorIs(t, service.UpdateName(t.Context(), "v1", ""), ErrNameRequired)
}

func TestVehicleService_AddVehicle_DuplicateName(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	_, err := service.AddVehicle(t.Context(), "u1", "rm1", "Same Name")
	require.NoError(t, err)

	_, err = service.AddVehicle(t.Context(), "u1", "rm2", "Same Name")
	assert.ErrorIs(t, err, ErrDuplicateVehicleName)
}

func TestVehicleService_AddVehicle_DifferentUsersSameName(t *testing.T) {
	db := setupVehicleServiceTestDB(t)
	service := newVehicleService(db)

	v1, err := service.AddVehicle(t.Context(), "u1", "rm1", "Same Name")
	require.NoError(t, err)
	v2, err := service.AddVehicle(t.Context(), "u2", "rm2", "Same Name")
	require.NoError(t, err)
	assert.NotEqual(t, v1.ID, v2.ID)
}
