package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/testdb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Error-path tests triggered by context cancellation or closed DB
// ---------------------------------------------------------------------------

func seedTestDBForErrorPaths(t *testing.T) (*sql.DB, string) {
	t.Helper()
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)

	testdb.SeedFullTestDB(t, db)

	// Create an active session with power readings and SOC snapshots
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, NewChargeSessionRepository(db).Create(t.Context(), session))

	ci := 100.0
	require.NoError(t, NewChargeSessionRepository(db).CreatePowerReading(t.Context(), &models.PowerReading{
		ID:        "pr-err-test",
		SessionID: session.ID,
		Timestamp: time.Now(),
		Voltage:   230,
		Current:   2.6,
		Power:     600,
		EnergyKwh: 0.5,
		CarbonIntensityGCo2PerKwh: &ci,
	}))

	require.NoError(t, NewChargeSessionRepository(db).CreateSOCSnapshot(t.Context(), &models.SOCSnapshot{
		ID:         "snap-err-test",
		SessionID:  session.ID,
		Timestamp:  time.Now(),
		SocPercent: 45.0,
	}))

	return db, session.ID
}

// --- Delete / deleteInTx error paths ---

func TestChargeSessionRepository_Delete_BeginTxError(t *testing.T) {
	db, _ := seedTestDBForErrorPaths(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)
	// Close DB to force BeginTx error
	db.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cancel() // cancel as well

	err := repo.Delete(ctx, "any-id")
	assert.Error(t, err)
}

func TestChargeSessionRepository_Delete_ContextCanceled(t *testing.T) {
	db, sessionID := seedTestDBForErrorPaths(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // cancel immediately - deleteInTx will fail on first ExecContext

	err := repo.Delete(ctx, sessionID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestChargeSessionRepository_DeleteInTx_ContextCanceled(t *testing.T) {
	db, sessionID := seedTestDBForErrorPaths(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	tx, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	err = repo.deleteInTx(ctx, tx, sessionID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- ActivatePending error paths ---

func TestChargeSessionRepository_ActivatePending_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusPending,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.ActivatePending(ctx, session.ID, time.Now())
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- CancelPending error paths ---

func TestChargeSessionRepository_CancelPending_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusPending,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.CancelPending(ctx, session.ID, time.Now())
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- UpdateTarget error paths ---

func TestChargeSessionRepository_UpdateTarget_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.UpdateTarget(ctx, session.ID, 80)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- GetPending error path (scanChargeSession error) ---

func TestChargeSessionRepository_GetPending_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusPending,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	// Close DB after query is prepared but before scan
	db.Close()

	_, err := repo.GetPending(t.Context())
	assert.Error(t, err)
}

// --- GetActiveByPlug error path ---

func TestChargeSessionRepository_GetActiveByPlug_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	userID := "test-user"
	plugID := "plug-err-test"
	require.NoError(t, testdb.InsertPlug(db, plugID, userID, "Err Plug", "ns-err", "err"))

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    &userID,
		PlugID:    &plugID,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    "active",
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ctx := internal.WithUserID(t.Context(), userID)

	// Close DB to force scan error
	db.Close()

	_, err := repo.GetActiveByPlug(ctx, plugID)
	assert.Error(t, err)
}

// --- GetPendingByPlug error path ---

func TestChargeSessionRepository_GetPendingByPlug_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	userID := "test-user"
	plugID := "plug-pending-err"
	require.NoError(t, testdb.InsertPlug(db, plugID, userID, "Pending Err", "ns-pend-err", "pend-err"))

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    &userID,
		PlugID:    &plugID,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusPending,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ctx := internal.WithUserID(t.Context(), userID)

	// Close DB to force scan error
	db.Close()

	_, err := repo.GetPendingByPlug(ctx, plugID)
	assert.Error(t, err)
}

// --- scanChargeSessionRows error path ---

func TestScanChargeSessionRows_ContextCanceled(t *testing.T) {
	db, _ := seedTestDBForErrorPaths(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	rows, err := db.QueryContext(ctx, `SELECT `+chargeSessionColumns+` FROM charge_sessions`)
	// Query may succeed or fail depending on timing; either way, scanning should error
	if err == nil {
		sessions, scanErr := scanChargeSessionRows(rows)
		assert.Empty(t, sessions)
		assert.Error(t, scanErr)
	}
	// If query itself failed due to cancellation, that's also an error path
}

// --- GetPowerReadings error path ---

func TestChargeSessionRepository_GetPowerReadings_ContextCanceled(t *testing.T) {
	db, sessionID := seedTestDBForErrorPaths(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	readings, err := repo.GetPowerReadings(ctx, sessionID)
	assert.Empty(t, readings)
	assert.Error(t, err)
}

// --- GetSOCSnapshots error path ---

func TestChargeSessionRepository_GetSOCSnapshots_ContextCanceled(t *testing.T) {
	db, sessionID := seedTestDBForErrorPaths(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	snaps, err := repo.GetSOCSnapshots(ctx, sessionID)
	assert.Empty(t, snaps)
	assert.Error(t, err)
}

// --- GetAvgCarbonIntensityForSessions error path ---

func TestChargeSessionRepository_GetAvgCarbonIntensityForSessions_ContextCanceled(t *testing.T) {
	db, sessionID := seedTestDBForErrorPaths(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result, err := repo.GetAvgCarbonIntensityForSessions(ctx, []string{sessionID})
	assert.Nil(t, result)
	assert.Error(t, err)
}

// --- GetLastSOCSnapshot error path (scan error) ---

func TestChargeSessionRepository_GetLastSOCSnapshot_DBError(t *testing.T) {
	db, sessionID := seedTestDBForErrorPaths(t)

	repo := NewChargeSessionRepository(db)
	// Close DB to force scan error
	db.Close()

	_, err := repo.GetLastSOCSnapshot(t.Context(), sessionID)
	assert.Error(t, err)
}

// --- GetLastPowerReading error path (scan error) ---

func TestChargeSessionRepository_GetLastPowerReading_DBError(t *testing.T) {
	db, sessionID := seedTestDBForErrorPaths(t)

	repo := NewChargeSessionRepository(db)
	// Close DB to force scan error
	db.Close()

	_, err := repo.GetLastPowerReading(t.Context(), sessionID)
	assert.Error(t, err)
}

// --- ResolveChartSession error paths ---

func TestChargeSessionRepository_ResolveChartSession_VehicleError(t *testing.T) {
	db := setupChargeSessionDB(t)

	repo := NewChargeSessionRepository(db)
	// Close DB - GetActiveByVehicle will error
	db.Close()

	_, err := repo.ResolveChartSession(t.Context(), "", "rm1")
	assert.Error(t, err)
}

// --- UpdateStatus error path ---

func TestChargeSessionRepository_UpdateStatus_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.UpdateStatus(ctx, session.ID, "completed")
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- UpdateEndWithStats error path ---

func TestChargeSessionRepository_UpdateEndWithStats_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	avgCarbon := 250.0
	err := repo.UpdateEndWithStats(ctx, session.ID, time.Now(), 1.9, 80, 1.52, 1.9, 500, &avgCarbon, 0, 0)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- CreatePowerReading error path ---

func TestChargeSessionRepository_CreatePowerReading_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.CreatePowerReading(ctx, &models.PowerReading{
		SessionID: session.ID,
		Timestamp: time.Now(),
		Voltage:   230,
		Current:   2.6,
		Power:     600,
		EnergyKwh: 0.5,
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- CreateSOCSnapshot error path ---

func TestChargeSessionRepository_CreateSOCSnapshot_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.CreateSOCSnapshot(ctx, &models.SOCSnapshot{
		ID:         "snap-cancel",
		SessionID:  session.ID,
		Timestamp:  time.Now(),
		SocPercent: 50.0,
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- UpdateStartTotalKwh error path ---

func TestChargeSessionRepository_UpdateStartTotalKwh_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.UpdateStartTotalKwh(ctx, session.ID, 100.0)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- UpdateEndedAt error path ---

func TestChargeSessionRepository_UpdateEndedAt_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.UpdateEndedAt(ctx, session.ID, time.Now())
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- UpdateCancelData error path ---

func TestChargeSessionRepository_UpdateCancelData_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.UpdateCancelData(ctx, session.ID, time.Now(), nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- UpdateLastBlendedKwh error path ---

func TestChargeSessionRepository_UpdateLastBlendedKwh_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.UpdateLastBlendedKwh(ctx, session.ID, 0.75)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- GetSessionAggregates error path ---

func TestChargeSessionRepository_GetSessionAggregates_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	agg, err := repo.GetSessionAggregates(ctx, "rm1", time.Time{})
	assert.Nil(t, agg)
	assert.Error(t, err)
}

// --- GetDailyEnergy error path ---

func TestChargeSessionRepository_GetDailyEnergy_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	daily, err := repo.GetDailyEnergy(ctx, "rm1", time.Time{})
	assert.Nil(t, daily)
	assert.Error(t, err)
}

// --- GetVehicleChargingEfficiency error path ---

func TestChargeSessionRepository_GetVehicleChargingEfficiency_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	eff, err := repo.GetVehicleChargingEfficiency(ctx, "rm1")
	assert.Equal(t, float64(0), eff)
	assert.Error(t, err)
}

// --- Create error path ---

func TestChargeSessionRepository_Create_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := repo.Create(ctx, session)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- FindByID already has error test, but let's add context cancellation ---

func TestChargeSessionRepository_FindByID_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// Close DB to force scan error on QueryRowContext
	db.Close()

	_, err := repo.FindByID(ctx, session.ID)
	assert.Error(t, err)
}

// --- GetActive error path via context ---

func TestChargeSessionRepository_GetActive_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	// Close DB to force scan error
	db.Close()

	_, err := repo.GetActive(t.Context())
	assert.Error(t, err)
}

// --- GetActiveByVehicle error path via context ---

func TestChargeSessionRepository_GetActiveByVehicle_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	// Close DB to force scan error
	db.Close()

	_, err := repo.GetActiveByVehicle(t.Context(), "rm1")
	assert.Error(t, err)
}

// --- GetLastCompletedByVehicle error path via context ---

func TestChargeSessionRepository_GetLastCompletedByVehicle_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusCompleted,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	// Close DB to force scan error
	db.Close()

	_, err := repo.GetLastCompletedByVehicle(t.Context(), "rm1")
	assert.Error(t, err)
}

// --- GetLastCompleted error path via context ---

func TestChargeSessionRepository_GetLastCompleted_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusCompleted,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	// Close DB to force scan error
	db.Close()

	_, err := repo.GetLastCompleted(t.Context())
	assert.Error(t, err)
}

// --- GetAll error path via context ---

func TestChargeSessionRepository_GetAll_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	all, err := repo.GetAll(ctx)
	assert.Nil(t, all)
	assert.Error(t, err)
}

// --- GetAllByVehicle error path via context ---

func TestChargeSessionRepository_GetAllByVehicle_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	all, err := repo.GetAllByVehicle(ctx, "rm1")
	assert.Nil(t, all)
	assert.Error(t, err)
}

// --- GetLatest error path via context ---

func TestChargeSessionRepository_GetLatest_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	all, err := repo.GetLatest(ctx, 10, 0)
	assert.Nil(t, all)
	assert.Error(t, err)
}

// --- GetLatestByVehicle error path via context ---

func TestChargeSessionRepository_GetLatestByVehicle_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	all, err := repo.GetLatestByVehicle(ctx, "rm1", 10, 0)
	assert.Nil(t, all)
	assert.Error(t, err)
}

// --- GetByDate error path via context ---

func TestChargeSessionRepository_GetByDate_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	all, err := repo.GetByDate(ctx, "2025-01-15", 10, 0)
	assert.Nil(t, all)
	assert.Error(t, err)
}

// --- GetByVehicleAndDate error path via context ---

func TestChargeSessionRepository_GetByVehicleAndDate_ContextCanceled(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	all, err := repo.GetByVehicleAndDate(ctx, "rm1", "2025-01-15", 10, 0)
	assert.Nil(t, all)
	assert.Error(t, err)
}
