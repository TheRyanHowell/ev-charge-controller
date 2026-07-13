package repository

import (
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

const (
	repoTestUserID = "test-user"
	repoTestPlugID = "test-plug"
)

var (
	repoTestUserIDStr = repoTestUserID
	repoTestPlugIDStr = repoTestPlugID
	repoTestUserIDPtr = &repoTestUserIDStr
	repoTestPlugIDPtr = &repoTestPlugIDStr
)

func setupChargeSessionDB(t *testing.T) *sql.DB {
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	testdb.SeedFullTestDB(t, db)
	return db
}

func setupChargeSessionDBFull(t *testing.T) *sql.DB {
	return setupChargeSessionDB(t)
}

func TestChargeSessionRepository_Create(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    "active",
	}

	err := repo.Create(t.Context(), session)
	assert.NoError(t, err)
	assert.NotEmpty(t, session.ID)

	found, err := repo.FindByID(t.Context(), session.ID)
	assert.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, "rm1", found.VehicleID)
}

func TestChargeSessionRepository_Create_TwoStageFields(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)
	holdPercent := 64.0
	readyByTime := "07:00"
	session := &models.ChargeSession{
		VehicleID:       "rm1",
		UserID:          repoTestUserIDPtr,
		PlugID:          repoTestPlugIDPtr,
		StartKwh:        0.38,
		TargetKwh:       1.9,
		Status:          "active",
		HoldPercent:     &holdPercent,
		ReadyByTime:     &readyByTime,
		TargetPercent:   80,
		CarbonAwareHold: true,
	}

	require.NoError(t, repo.Create(t.Context(), session))

	found, err := repo.FindByID(t.Context(), session.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.NotNil(t, found.HoldPercent)
	assert.Equal(t, 64.0, *found.HoldPercent)
	require.NotNil(t, found.ReadyByTime)
	assert.Equal(t, "07:00", *found.ReadyByTime)
	assert.True(t, found.CarbonAwareHold)
}

func TestChargeSessionRepository_Create_TwoStageFieldsNil(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    "active",
	}

	require.NoError(t, repo.Create(t.Context(), session))

	found, err := repo.FindByID(t.Context(), session.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Nil(t, found.HoldPercent)
	assert.Nil(t, found.ReadyByTime)
	assert.False(t, found.CarbonAwareHold)
}

// TestChargeSessionRepository_ReadsPopulateUserID guards a regression where
// reads omitted the user_id column, leaving ChargeSession.UserID nil. That made
// ownership checks (verifySessionOwnership) treat every session as not-owned and
// reject target updates with 404 for authenticated callers.
func TestChargeSessionRepository_ReadsPopulateUserID(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    "active",
	}
	require.NoError(t, repo.Create(t.Context(), session))

	byID, err := repo.FindByID(t.Context(), session.ID)
	require.NoError(t, err)
	require.NotNil(t, byID.UserID)
	assert.Equal(t, repoTestUserID, *byID.UserID)

	active, err := repo.GetActiveByVehicle(t.Context(), "rm1")
	require.NoError(t, err)
	require.NotNil(t, active.UserID)
	assert.Equal(t, repoTestUserID, *active.UserID)
}

func TestChargeSessionRepository_GetActive(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Create first active session
	session1 := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Now().Add(-time.Hour), StartKwh: 0.38,
		TargetKwh: 1.9, Status: "active",
	}
	_ = repo.Create(t.Context(), session1)

	// Create second active session
	session2 := &models.ChargeSession{
		VehicleID: "rm1s",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Now(), StartKwh: 0.2,
		TargetKwh: 3.8, Status: "active",
	}
	_ = repo.Create(t.Context(), session2)

	// Get active should return the most recent
	active, err := repo.GetActive(t.Context(), )
	assert.NoError(t, err)
	assert.NotNil(t, active)
	assert.Equal(t, session2.ID, active.ID)
}

func TestChargeSessionRepository_UpdateStatus(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 1.9, Status: "active",
	}
	_ = repo.Create(t.Context(), session)

	err := repo.UpdateStatus(t.Context(), session.ID, "completed")
	assert.NoError(t, err)

	found, err := repo.FindByID(t.Context(), session.ID)
	assert.NoError(t, err)
	assert.Equal(t, "completed", found.Status)
}

func TestChargeSessionRepository_UpdateEndWithStats(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 1.9, Status: "active",
	}
	_ = repo.Create(t.Context(), session)

	endTime := time.Now()
	batteryKwh := 1.52
	wallKwh := 1.9
	co2Grams := 500.0
	avgCarbon := 263.16
	err := repo.UpdateEndWithStats(t.Context(), session.ID, endTime, 1.9, 80, batteryKwh, wallKwh, co2Grams, &avgCarbon, 0, 0)
	assert.NoError(t, err)

	found, err := repo.FindByID(t.Context(), session.ID)
	assert.NoError(t, err)
	assert.Equal(t, "completed", found.Status)
	assert.NotNil(t, found.EndKwh)
	assert.Equal(t, 1.9, *found.EndKwh)
	assert.NotNil(t, found.EndedAt)
	assert.Equal(t, endTime.Unix(), found.EndedAt.Unix())
	assert.Equal(t, &batteryKwh, found.BatteryKwh)
	assert.Equal(t, &wallKwh, found.WallKwh)
	assert.Equal(t, &co2Grams, found.Co2Grams)
	assert.Equal(t, &avgCarbon, found.AvgCarbonIntensity)
}

func TestChargeSessionRepository_PowerReadings(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 1.9, Status: "active",
	}
	_ = repo.Create(t.Context(), session)

	reading := &models.PowerReading{
		SessionID: session.ID, Timestamp: time.Now(), Voltage: 230,
		Current: 2.6, Power: 600, EnergyKwh: 0.38,
	}
	err := repo.CreatePowerReading(t.Context(), reading)
	assert.NoError(t, err)

	readings, err := repo.GetPowerReadings(t.Context(), session.ID)
	assert.NoError(t, err)
	assert.Len(t, readings, 1)
	assert.Equal(t, 600.0, readings[0].Power)
}

func TestChargeSessionRepository_GetActiveByVehicle(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// No active session
	active, err := repo.GetActiveByVehicle(t.Context(), "rm1")
	assert.NoError(t, err)
	assert.Nil(t, active)

	// Create an active session for rm1
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 1.9, Status: "active",
	}
	_ = repo.Create(t.Context(), session)

	active, err = repo.GetActiveByVehicle(t.Context(), "rm1")
	assert.NoError(t, err)
	assert.NotNil(t, active)
	assert.Equal(t, session.ID, active.ID)

	// Different vehicle should return nil
	active, err = repo.GetActiveByVehicle(t.Context(), "rm2")
	assert.NoError(t, err)
	assert.Nil(t, active)
}

// GetActiveByVehicle must report pending and conditioning sessions as active -
// matching GetActive - so a vehicle mid-conditioning is never read as "idle".
func TestChargeSessionRepository_GetActiveByVehicle_PendingAndConditioning(t *testing.T) {
	for _, status := range []string{models.SessionStatusPending, models.SessionStatusConditioning} {
		t.Run(status, func(t *testing.T) {
			db := setupChargeSessionDB(t)
			defer db.Close()
			repo := NewChargeSessionRepository(db)

			session := &models.ChargeSession{
				VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
				TargetKwh: 1.9, Status: status,
			}
			require.NoError(t, repo.Create(t.Context(), session))

			active, err := repo.GetActiveByVehicle(t.Context(), "rm1")
			assert.NoError(t, err)
			require.NotNil(t, active, "%s session must be reported as active", status)
			assert.Equal(t, session.ID, active.ID)
		})
	}
}

func TestChargeSessionRepository_GetLastCompletedByVehicle(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// No completed session
	completed, err := repo.GetLastCompletedByVehicle(t.Context(), "rm1")
	assert.NoError(t, err)
	assert.Nil(t, completed)

	// Create a completed session
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 1.9, Status: "active",
	}
	_ = repo.Create(t.Context(), session)
	_ = repo.UpdateStatus(t.Context(), session.ID, "completed")

	completed, err = repo.GetLastCompletedByVehicle(t.Context(), "rm1")
	assert.NoError(t, err)
	assert.NotNil(t, completed)
	assert.Equal(t, session.ID, completed.ID)
}

func TestChargeSessionRepository_GetLastCompleted(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// No completed session
	completed, err := repo.GetLastCompleted(t.Context(), )
	assert.NoError(t, err)
	assert.Nil(t, completed)

	// Create a completed session
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 1.9, Status: "active",
	}
	_ = repo.Create(t.Context(), session)
	_ = repo.UpdateStatus(t.Context(), session.ID, "completed")

	completed, err = repo.GetLastCompleted(t.Context(), )
	assert.NoError(t, err)
	assert.NotNil(t, completed)
	assert.Equal(t, session.ID, completed.ID)
}

func TestChargeSessionRepository_GetAll(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Empty list
	all, err := repo.GetAll(t.Context(), )
	assert.NoError(t, err)
	assert.Empty(t, all)

	// Create two sessions with distinct timestamps for deterministic ordering
	s1 := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 1.9, Status: "active",
		CreatedAt: time.Now().Add(-time.Hour),
	}
	s2 := &models.ChargeSession{
		VehicleID: "rm2",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.5,
		TargetKwh: 2.0, Status: "completed",
		CreatedAt: time.Now(),
	}
	_ = repo.Create(t.Context(), s1)
	_ = repo.Create(t.Context(), s2)

	all, err = repo.GetAll(t.Context(), )
	assert.NoError(t, err)
	assert.Len(t, all, 2)
	// Should be ordered by created_at DESC
	assert.Equal(t, s2.ID, all[0].ID)
	assert.Equal(t, s1.ID, all[1].ID)
}

func TestChargeSessionRepository_GetAllByVehicle(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Create sessions for two vehicles with distinct timestamps
	s1 := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 1.9, Status: "active",
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	s2 := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.5,
		TargetKwh: 2.0, Status: "completed",
		CreatedAt: time.Now().Add(-time.Hour),
	}
	s3 := &models.ChargeSession{
		VehicleID: "rm2",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.4,
		TargetKwh: 1.8, Status: "active",
		CreatedAt: time.Now(),
	}
	_ = repo.Create(t.Context(), s1)
	_ = repo.Create(t.Context(), s2)
	_ = repo.Create(t.Context(), s3)

	all, err := repo.GetAllByVehicle(t.Context(), "rm1")
	assert.NoError(t, err)
	assert.Len(t, all, 2)
	assert.Equal(t, s2.ID, all[0].ID)

	all, err = repo.GetAllByVehicle(t.Context(), "rm2")
	assert.NoError(t, err)
	assert.Len(t, all, 1)
	assert.Equal(t, s3.ID, all[0].ID)
}

func TestChargeSessionRepository_UpdateTarget(t *testing.T) {
	db := setupChargeSessionDBFull(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Create a session for rm1 (seed data: capacity_kwh=2.026)
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 2.026, StartPercent: 20, TargetPercent: 100,
		Status: "active",
	}
	_ = repo.Create(t.Context(), session)

	// Update target to 80%
	err := repo.UpdateTarget(t.Context(), session.ID, 80)
	assert.NoError(t, err)

	found, err := repo.FindByID(t.Context(), session.ID)
	assert.NoError(t, err)
	assert.Equal(t, 80.0, found.TargetPercent)
	assert.InDelta(t, 1.6208, found.TargetKwh, 0.001) // 2.026 * 80 / 100
}

func TestChargeSessionRepository_UpdateTarget_CalculatesTargetKwh(t *testing.T) {
	tests := []struct {
		name              string
		capacityKwh       float64
		startKwh          float64
		targetPercent     float64
		expectedTargetKwh float64
	}{
		{
			name:              "80 percent of 2.026 kWh battery",
			capacityKwh:       2.026,
			startKwh:          0.6078,
			targetPercent:     80,
			expectedTargetKwh: 1.6208,
		},
		{
			name:              "50 percent of 1.9 kWh battery",
			capacityKwh:       1.9,
			startKwh:          0.38,
			targetPercent:     50,
			expectedTargetKwh: 0.95,
		},
		{
			name:              "100 percent equals full capacity",
			capacityKwh:       2.026,
			startKwh:          0.6078,
			targetPercent:     100,
			expectedTargetKwh: 2.026,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupChargeSessionDBFull(t)
			defer db.Close()

			repo := NewChargeSessionRepository(db)

			// Insert a custom vehicle_model with the test-case capacity, then an instance.
			_, err := db.Exec(
				`INSERT OR IGNORE INTO vehicle_models (id, name, capacity_kwh, charger_output_w, charging_efficiency, range_min_mi, range_max_mi) VALUES ('test-model', 'Test Model', ?, 600, 0.8, 0, 0)`,
				tt.capacityKwh,
			)
			require.NoError(t, err)
			_, err = db.Exec(
				`INSERT OR IGNORE INTO vehicles (id, user_id, model_id, name, current_percent, target_percent, created_at) VALUES ('v1', 'test-user', 'test-model', 'Test', 20, 80, CURRENT_TIMESTAMP)`,
			)
			require.NoError(t, err)

			session := &models.ChargeSession{
				VehicleID: "v1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: tt.startKwh,
				TargetKwh: tt.capacityKwh, StartPercent: 30, TargetPercent: 100,
				Status: "active",
			}
			require.NoError(t, repo.Create(t.Context(), session))

			err = repo.UpdateTarget(t.Context(), session.ID, tt.targetPercent)
			require.NoError(t, err)

			found, err := repo.FindByID(t.Context(), session.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.targetPercent, found.TargetPercent)
			assert.InDelta(t, tt.expectedTargetKwh, found.TargetKwh, 0.001,
				"targetKwh must be capacity * percent / 100, NOT startKwh + capacity * percent / 100")
		})
	}
}

func TestChargeSessionRepository_SOCSnapshots(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// No snapshots
	snaps, err := repo.GetSOCSnapshots(t.Context(), "nonexistent")
	assert.NoError(t, err)
	assert.Empty(t, snaps)

	// Create a session
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 1.9, Status: "active",
	}
	_ = repo.Create(t.Context(), session)

	// Create snapshots
	snap1 := &models.SOCSnapshot{
		ID: "snap1", SessionID: session.ID, SocPercent: 30,
		Timestamp: time.Now(),
	}
	snap2 := &models.SOCSnapshot{
		ID: "snap2", SessionID: session.ID, SocPercent: 50,
		Timestamp: time.Now().Add(time.Minute),
	}
	err = repo.CreateSOCSnapshot(t.Context(), snap1)
	assert.NoError(t, err)
	err = repo.CreateSOCSnapshot(t.Context(), snap2)
	assert.NoError(t, err)

	snaps, err = repo.GetSOCSnapshots(t.Context(), session.ID)
	assert.NoError(t, err)
	assert.Len(t, snaps, 2)
	assert.Equal(t, 30.0, snaps[0].SocPercent)
	assert.Equal(t, 50.0, snaps[1].SocPercent)
}

func TestChargeSessionRepository_ResolveChartSession(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Case 1: explicit sessionID
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 1.9, Status: "active",
	}
	_ = repo.Create(t.Context(), session)

	resolved, err := repo.ResolveChartSession(t.Context(), session.ID, "")
	assert.NoError(t, err)
	assert.NotNil(t, resolved)
	assert.Equal(t, session.ID, resolved.ID)

	// Case 2: active session fallback (by vehicle)
	resolved, err = repo.ResolveChartSession(t.Context(), "", "rm1")
	assert.NoError(t, err)
	assert.NotNil(t, resolved)
	assert.Equal(t, session.ID, resolved.ID)

	// Case 3: falls back to completed when no active
	_ = repo.UpdateStatus(t.Context(), session.ID, "completed")
	resolved, err = repo.ResolveChartSession(t.Context(), "", "rm1")
	assert.NoError(t, err)
	assert.NotNil(t, resolved)
	assert.Equal(t, session.ID, resolved.ID)

	// Case 4: no session anywhere
	resolved, err = repo.ResolveChartSession(t.Context(), "", "unknown")
	assert.NoError(t, err)
	assert.Nil(t, resolved)
}

func TestChargeSessionRepository_ResolveChartSession_NoParamsCompletedFallback(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Insert a completed session (no active sessions)
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 1.9, StartPercent: 20, TargetPercent: 80,
		Status: "completed",
	}
	require.NoError(t, repo.Create(t.Context(), session))

	// No sessionID, no vehicleID - should fall back to GetLastCompleted
	resolved, err := repo.ResolveChartSession(t.Context(), "", "")
	assert.NoError(t, err)
	assert.NotNil(t, resolved)
	assert.Equal(t, session.ID, resolved.ID)
}

func TestChargeSessionRepository_ResolveChartSession_NoParamsNoSessions(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Empty database, no params - should return nil
	resolved, err := repo.ResolveChartSession(t.Context(), "", "")
	assert.NoError(t, err)
	assert.Nil(t, resolved)
}

func TestChargeSessionRepository_ResolveChartSession_NonexistentSessionID(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Explicit nonexistent sessionID - should return nil, no error
	resolved, err := repo.ResolveChartSession(t.Context(), "cs_nonexistent", "")
	assert.NoError(t, err)
	assert.Nil(t, resolved)
}

func TestChargeSessionRepository_ResolveChartSession_ActiveByVehicle(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Create an active session for rm1
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 1.9, Status: "active",
	}
	_ = repo.Create(t.Context(), session)

	resolved, err := repo.ResolveChartSession(t.Context(), "", "rm1")
	assert.NoError(t, err)
	assert.NotNil(t, resolved)
	assert.Equal(t, session.ID, resolved.ID)
}

func TestChargeSessionRepository_ResolveChartSession_ActiveByVehicle_NoActive(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Create a completed session for rm1 (no active session)
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 1.9, Status: "completed",
	}
	_ = repo.Create(t.Context(), session)

	resolved, err := repo.ResolveChartSession(t.Context(), "", "rm1")
	assert.NoError(t, err)
	assert.NotNil(t, resolved)
	assert.Equal(t, session.ID, resolved.ID)
}

func TestNullableTimeScan_NilValue(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Insert a completed session with NULL ended_at
	startTime := time.Now().Add(-time.Hour)
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "session-null-time",
		VehicleID: "rm1",
		UserID:    "test-user",
		PlugID:    "test-plug",
		Status:    "completed",
		CreatedAt: startTime,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		StartPct:  20,
		TargetPct: 80,
	}))

	found, err := repo.FindByID(t.Context(), "session-null-time")
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Nil(t, found.EndedAt) // should be nil, not zero time
}

func TestNullableFloatScan_NilValue(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Insert a session with NULL end_kwh
	startTime := time.Now().Add(-time.Hour)
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "session-null-kwh",
		VehicleID: "rm1",
		UserID:    "test-user",
		PlugID:    "test-plug",
		Status:    "completed",
		CreatedAt: startTime,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		StartPct:  20,
		TargetPct: 80,
	}))

	found, err := repo.FindByID(t.Context(), "session-null-kwh")
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Nil(t, found.EndKwh) // should be nil, not zero value
}

func TestFindByID_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)

	// Close the DB to force an error
	db.Close()

	_, err := repo.FindByID(t.Context(), "any-id")
	assert.Error(t, err)
}

func TestGetActive_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)

	// Close the DB to force an error
	db.Close()

	_, err := repo.GetActive(t.Context(), )
	assert.Error(t, err)
}

func TestChargeSessionRepository_UpdateTarget_SessionNotFound(t *testing.T) {
	db := setupChargeSessionDBFull(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Call UpdateTarget with a non-existent session ID (no vehicle needed)
	err := repo.UpdateTarget(t.Context(), "nonexistent-session-id", 80)
	assert.ErrorIs(t, err, ErrSessionWrongState)
}

func TestChargeSessionRepository_GetLatest_WithOffset(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Create 5 sessions at different times
	sessions := make([]*models.ChargeSession, 5)
	for i := 0; i < 5; i++ {
		sessions[i] = &models.ChargeSession{
			VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
			CreatedAt: time.Now().Add(-time.Duration(4-i) * time.Hour),
			StartKwh:  0.38,
			TargetKwh: 1.9,
			Status:    "completed",
		}
		require.NoError(t, repo.Create(t.Context(), sessions[i]))
	}

	// GetLatest(2, 0) returns the 2 most recent (s[4], s[3])
	latest, err := repo.GetLatest(t.Context(), 2, 0)
	assert.NoError(t, err)
	assert.Len(t, latest, 2)
	assert.Equal(t, sessions[4].ID, latest[0].ID)
	assert.Equal(t, sessions[3].ID, latest[1].ID)

	// GetLatest(2, 2) returns next page (s[2], s[1])
	latest, err = repo.GetLatest(t.Context(), 2, 2)
	assert.NoError(t, err)
	assert.Len(t, latest, 2)
	assert.Equal(t, sessions[2].ID, latest[0].ID)
	assert.Equal(t, sessions[1].ID, latest[1].ID)

	// GetLatest(2, 4) returns last session
	latest, err = repo.GetLatest(t.Context(), 2, 4)
	assert.NoError(t, err)
	assert.Len(t, latest, 1)
	assert.Equal(t, sessions[0].ID, latest[0].ID)

	// GetLatest(2, 5) returns empty (beyond data)
	latest, err = repo.GetLatest(t.Context(), 2, 5)
	assert.NoError(t, err)
	assert.Empty(t, latest)
}

func TestChargeSessionRepository_GetLatest(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Empty database returns empty slice
	all, err := repo.GetLatest(t.Context(), 10, 0)
	assert.NoError(t, err)
	assert.Empty(t, all)

	// Create 3 sessions at different times
	s1 := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Now().Add(-2 * time.Hour), StartKwh: 0.38,
		TargetKwh: 1.9, Status: "completed",
	}
	s2 := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Now().Add(-1 * time.Hour), StartKwh: 0.4,
		TargetKwh: 1.9, Status: "completed",
	}
	s3 := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Now(), StartKwh: 0.5,
		TargetKwh: 1.9, Status: "active",
	}
	require.NoError(t, repo.Create(t.Context(), s1))
	require.NoError(t, repo.Create(t.Context(), s2))
	require.NoError(t, repo.Create(t.Context(), s3))

	// GetLatest(1, 0) returns the most recent
	latest, err := repo.GetLatest(t.Context(), 1, 0)
	assert.NoError(t, err)
	assert.Len(t, latest, 1)
	assert.Equal(t, s3.ID, latest[0].ID)

	// GetLatest(2, 0) returns 2 most recent in order
	latest, err = repo.GetLatest(t.Context(), 2, 0)
	assert.NoError(t, err)
	assert.Len(t, latest, 2)
	assert.Equal(t, s3.ID, latest[0].ID)
	assert.Equal(t, s2.ID, latest[1].ID)

	// GetLatest(0, 0) returns empty
	latest, err = repo.GetLatest(t.Context(), 0, 0)
	assert.NoError(t, err)
	assert.Empty(t, latest)
}

func TestChargeSessionRepository_GetLatestByVehicle_WithOffset(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Create 4 sessions for rm1, 2 for rm2
	rm1Sessions := make([]*models.ChargeSession, 4)
	for i := 0; i < 4; i++ {
		rm1Sessions[i] = &models.ChargeSession{
			VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
			CreatedAt: time.Now().Add(-time.Duration(3-i) * time.Hour),
			StartKwh:  0.38,
			TargetKwh: 1.9,
			Status:    "completed",
		}
		require.NoError(t, repo.Create(t.Context(), rm1Sessions[i]))
	}
	rm2Session := &models.ChargeSession{
		VehicleID: "rm2",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Now().Add(-time.Hour),
		StartKwh:  1.0,
		TargetKwh: 3.8,
		Status:    "completed",
	}
	require.NoError(t, repo.Create(t.Context(), rm2Session))

	// GetLatestByVehicle("rm1", 2, 0) returns 2 most recent rm1 sessions
	latest, err := repo.GetLatestByVehicle(t.Context(), "rm1", 2, 0)
	assert.NoError(t, err)
	assert.Len(t, latest, 2)
	assert.Equal(t, rm1Sessions[3].ID, latest[0].ID)
	assert.Equal(t, rm1Sessions[2].ID, latest[1].ID)

	// GetLatestByVehicle("rm1", 2, 2) returns next page
	latest, err = repo.GetLatestByVehicle(t.Context(), "rm1", 2, 2)
	assert.NoError(t, err)
	assert.Len(t, latest, 2)
	assert.Equal(t, rm1Sessions[1].ID, latest[0].ID)
	assert.Equal(t, rm1Sessions[0].ID, latest[1].ID)

	// GetLatestByVehicle("rm2", 2, 0) returns rm2's single session
	latest, err = repo.GetLatestByVehicle(t.Context(), "rm2", 2, 0)
	assert.NoError(t, err)
	assert.Len(t, latest, 1)
	assert.Equal(t, rm2Session.ID, latest[0].ID)

	// GetLatestByVehicle("rm1", 2, 4) returns empty (beyond data)
	latest, err = repo.GetLatestByVehicle(t.Context(), "rm1", 2, 4)
	assert.NoError(t, err)
	assert.Empty(t, latest)
}

func TestChargeSessionRepository_GetLatestByVehicle(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Nonexistent vehicle returns empty
	all, err := repo.GetLatestByVehicle(t.Context(), "nonexistent", 1, 0)
	assert.NoError(t, err)
	assert.Empty(t, all)

	// Create sessions for two vehicles
	rm1Old := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Now().Add(-2 * time.Hour), StartKwh: 0.38,
		TargetKwh: 1.9, Status: "completed",
	}
	rm1New := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Now(), StartKwh: 0.5,
		TargetKwh: 1.9, Status: "active",
	}
	rm2 := &models.ChargeSession{
		VehicleID: "rm2",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Now().Add(-1 * time.Hour), StartKwh: 1.0,
		TargetKwh: 3.8, Status: "completed",
	}
	require.NoError(t, repo.Create(t.Context(), rm1Old))
	require.NoError(t, repo.Create(t.Context(), rm1New))
	require.NoError(t, repo.Create(t.Context(), rm2))

	// GetLatestByVehicle("rm1", 1, 0) returns rm1's most recent
	latest, err := repo.GetLatestByVehicle(t.Context(), "rm1", 1, 0)
	assert.NoError(t, err)
	assert.Len(t, latest, 1)
	assert.Equal(t, rm1New.ID, latest[0].ID)

	// GetLatestByVehicle("rm1", 2, 0) returns both rm1 sessions
	latest, err = repo.GetLatestByVehicle(t.Context(), "rm1", 2, 0)
	assert.NoError(t, err)
	assert.Len(t, latest, 2)
	assert.Equal(t, rm1New.ID, latest[0].ID)
	assert.Equal(t, rm1Old.ID, latest[1].ID)

	// GetLatestByVehicle("rm2", 2, 0) returns rm2's single session
	latest, err = repo.GetLatestByVehicle(t.Context(), "rm2", 2, 0)
	assert.NoError(t, err)
	assert.Len(t, latest, 1)
	assert.Equal(t, rm2.ID, latest[0].ID)
}

func TestChargeSessionRepository_GetByDate_WithLimitAndOffset(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Create 5 sessions on the same date
	sessions := make([]*models.ChargeSession, 5)
	for i := 0; i < 5; i++ {
		sessions[i] = &models.ChargeSession{
			VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
			CreatedAt: time.Date(2025, 1, 15, 8+i, 0, 0, 0, time.UTC),
			StartKwh:  0.38,
			TargetKwh: 1.9,
			Status:    "completed",
		}
		require.NoError(t, repo.Create(t.Context(), sessions[i]))
	}
	// Create 1 session on a different date
	otherDate := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC),
		StartKwh:  0.3,
		TargetKwh: 1.5,
		Status:    "completed",
	}
	require.NoError(t, repo.Create(t.Context(), otherDate))

	// GetByDate("2025-01-15", 2, 0) returns 2 most recent on that date
	all, err := repo.GetByDate(t.Context(), "2025-01-15", 2, 0)
	assert.NoError(t, err)
	assert.Len(t, all, 2)
	assert.Equal(t, sessions[4].ID, all[0].ID)
	assert.Equal(t, sessions[3].ID, all[1].ID)

	// GetByDate("2025-01-15", 2, 2) returns next page
	all, err = repo.GetByDate(t.Context(), "2025-01-15", 2, 2)
	assert.NoError(t, err)
	assert.Len(t, all, 2)
	assert.Equal(t, sessions[2].ID, all[0].ID)
	assert.Equal(t, sessions[1].ID, all[1].ID)

	// GetByDate("2025-01-15", 2, 4) returns last session
	all, err = repo.GetByDate(t.Context(), "2025-01-15", 2, 4)
	assert.NoError(t, err)
	assert.Len(t, all, 1)
	assert.Equal(t, sessions[0].ID, all[0].ID)

	// GetByDate("2025-01-15", 2, 5) returns empty
	all, err = repo.GetByDate(t.Context(), "2025-01-15", 2, 5)
	assert.NoError(t, err)
	assert.Empty(t, all)

	// GetByDate("2026-01-15", 10, 0) returns the other date's session
	all, err = repo.GetByDate(t.Context(), "2026-01-15", 10, 0)
	assert.NoError(t, err)
	assert.Len(t, all, 1)
	assert.Equal(t, otherDate.ID, all[0].ID)
}

func TestChargeSessionRepository_GetByDate(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Date with no sessions returns empty
	all, err := repo.GetByDate(t.Context(), "2024-06-01", 100, 0)
	assert.NoError(t, err)
	assert.Empty(t, all)

	// Create sessions on two different dates
	s1 := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Date(2025, 1, 15, 8, 0, 0, 0, time.UTC), StartKwh: 0.4,
		TargetKwh: 1.9, Status: "completed",
	}
	s2 := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC), StartKwh: 0.5,
		TargetKwh: 1.9, Status: "completed",
	}
	s3 := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC), StartKwh: 0.3,
		TargetKwh: 1.5, Status: "completed",
	}
	require.NoError(t, repo.Create(t.Context(), s1))
	require.NoError(t, repo.Create(t.Context(), s2))
	require.NoError(t, repo.Create(t.Context(), s3))

	// GetByDate("2025-01-15", 100, 0) returns 2 sessions
	all, err = repo.GetByDate(t.Context(), "2025-01-15", 100, 0)
	assert.NoError(t, err)
	assert.Len(t, all, 2)
	// Ordered by created_at DESC - afternoon session first
	assert.Equal(t, s2.ID, all[0].ID)
	assert.Equal(t, s1.ID, all[1].ID)

	// GetByDate("2026-01-15", 100, 0) returns 1 session
	all, err = repo.GetByDate(t.Context(), "2026-01-15", 100, 0)
	assert.NoError(t, err)
	assert.Len(t, all, 1)
	assert.Equal(t, s3.ID, all[0].ID)
}

func TestChargeSessionRepository_GetByVehicleAndDate_WithLimitAndOffset(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Create 4 sessions for rm1 on the same date
	rm1Sessions := make([]*models.ChargeSession, 4)
	for i := 0; i < 4; i++ {
		rm1Sessions[i] = &models.ChargeSession{
			VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
			CreatedAt: time.Date(2025, 1, 15, 8+i, 0, 0, 0, time.UTC),
			StartKwh:  0.38,
			TargetKwh: 1.9,
			Status:    "completed",
		}
		require.NoError(t, repo.Create(t.Context(), rm1Sessions[i]))
	}
	// Create 2 sessions for rm2 on the same date
	rm2Sessions := make([]*models.ChargeSession, 2)
	for i := 0; i < 2; i++ {
		rm2Sessions[i] = &models.ChargeSession{
			VehicleID: "rm2",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
			CreatedAt: time.Date(2025, 1, 15, 9+i, 0, 0, 0, time.UTC),
			StartKwh:  1.0,
			TargetKwh: 3.8,
			Status:    "completed",
		}
		require.NoError(t, repo.Create(t.Context(), rm2Sessions[i]))
	}

	// GetByVehicleAndDate("rm1", "2025-01-15", 2, 0) returns 2 most recent for rm1
	all, err := repo.GetByVehicleAndDate(t.Context(), "rm1", "2025-01-15", 2, 0)
	assert.NoError(t, err)
	assert.Len(t, all, 2)
	assert.Equal(t, rm1Sessions[3].ID, all[0].ID)
	assert.Equal(t, rm1Sessions[2].ID, all[1].ID)

	// GetByVehicleAndDate("rm1", "2025-01-15", 2, 2) returns next page for rm1
	all, err = repo.GetByVehicleAndDate(t.Context(), "rm1", "2025-01-15", 2, 2)
	assert.NoError(t, err)
	assert.Len(t, all, 2)
	assert.Equal(t, rm1Sessions[1].ID, all[0].ID)
	assert.Equal(t, rm1Sessions[0].ID, all[1].ID)

	// GetByVehicleAndDate("rm2", "2025-01-15", 1, 0) returns 1 most recent for rm2
	all, err = repo.GetByVehicleAndDate(t.Context(), "rm2", "2025-01-15", 1, 0)
	assert.NoError(t, err)
	assert.Len(t, all, 1)
	assert.Equal(t, rm2Sessions[1].ID, all[0].ID)

	// GetByVehicleAndDate("rm1", "2025-01-15", 2, 4) returns empty for rm1
	all, err = repo.GetByVehicleAndDate(t.Context(), "rm1", "2025-01-15", 2, 4)
	assert.NoError(t, err)
	assert.Empty(t, all)
}

func TestChargeSessionRepository_GetByVehicleAndDate(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Nonexistent vehicle returns empty
	all, err := repo.GetByVehicleAndDate(t.Context(), "nonexistent", "2025-01-15", 100, 0)
	assert.NoError(t, err)
	assert.Empty(t, all)

	// Create sessions for two vehicles on the same date
	rm1Day := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Date(2025, 1, 15, 8, 0, 0, 0, time.UTC), StartKwh: 0.4,
		TargetKwh: 1.9, Status: "completed",
	}
	rm2Day := &models.ChargeSession{
		VehicleID: "rm2",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC), StartKwh: 1.0,
		TargetKwh: 3.8, Status: "completed",
	}
	rm1Other := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC), StartKwh: 0.5,
		TargetKwh: 1.9, Status: "completed",
	}
	require.NoError(t, repo.Create(t.Context(), rm1Day))
	require.NoError(t, repo.Create(t.Context(), rm2Day))
	require.NoError(t, repo.Create(t.Context(), rm1Other))

	// GetByVehicleAndDate("rm1", "2025-01-15", 100, 0) returns only rm1's session on that date
	all, err = repo.GetByVehicleAndDate(t.Context(), "rm1", "2025-01-15", 100, 0)
	assert.NoError(t, err)
	assert.Len(t, all, 1)
	assert.Equal(t, rm1Day.ID, all[0].ID)

	// GetByVehicleAndDate("rm2", "2025-01-15", 100, 0) returns only rm2's session
	all, err = repo.GetByVehicleAndDate(t.Context(), "rm2", "2025-01-15", 100, 0)
	assert.NoError(t, err)
	assert.Len(t, all, 1)
	assert.Equal(t, rm2Day.ID, all[0].ID)

	// GetByVehicleAndDate("rm1", "2026-01-15", 100, 0) returns rm1's session on the other date
	all, err = repo.GetByVehicleAndDate(t.Context(), "rm1", "2026-01-15", 100, 0)
	assert.NoError(t, err)
	assert.Len(t, all, 1)
	assert.Equal(t, rm1Other.ID, all[0].ID)
}

func TestGetActiveByVehicle_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)
	db.Close()
	_, err := repo.GetActiveByVehicle(t.Context(), "rm1")
	assert.Error(t, err)
}

func TestGetLastCompletedByVehicle_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)
	db.Close()
	_, err := repo.GetLastCompletedByVehicle(t.Context(), "rm1")
	assert.Error(t, err)
}

func TestGetLastCompleted_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)
	db.Close()
	_, err := repo.GetLastCompleted(t.Context(), )
	assert.Error(t, err)
}

func TestGetAll_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)
	db.Close()
	_, err := repo.GetAll(t.Context(), )
	assert.Error(t, err)
}

func TestGetAllByVehicle_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)
	db.Close()
	_, err := repo.GetAllByVehicle(t.Context(), "rm1")
	assert.Error(t, err)
}

func TestGetLatest_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)
	db.Close()
	_, err := repo.GetLatest(t.Context(), 10, 0)
	assert.Error(t, err)
}

func TestGetLatestByVehicle_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)
	db.Close()
	_, err := repo.GetLatestByVehicle(t.Context(), "rm1", 10, 0)
	assert.Error(t, err)
}

func TestGetByDate_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)
	db.Close()
	_, err := repo.GetByDate(t.Context(), "2025-01-15", 10, 0)
	assert.Error(t, err)
}

func TestGetByVehicleAndDate_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)
	db.Close()
	_, err := repo.GetByVehicleAndDate(t.Context(), "rm1", "2025-01-15", 10, 0)
	assert.Error(t, err)
}

func TestGetPowerReadings_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)
	db.Close()
	_, err := repo.GetPowerReadings(t.Context(), "nonexistent")
	assert.Error(t, err)
}

func TestGetSOCSnapshots_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)
	db.Close()
	_, err := repo.GetSOCSnapshots(t.Context(), "nonexistent")
	assert.Error(t, err)
}

func TestResolveChartSession_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)
	db.Close()
	_, err := repo.ResolveChartSession(t.Context(), "", "")
	assert.Error(t, err)
}

func TestChargeSessionRepository_Delete(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	// Use a transaction to keep setup atomic and release the connection quickly
	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO charge_sessions (id, vehicle_id, created_at, start_kwh, target_kwh, start_percent, target_percent, status, user_id, plug_id)
		VALUES ('test-session', 'rm1', datetime('now'), 0.38, 1.9, 20, 80, 'completed', 'test-user', 'test-plug')`)
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO power_readings (id, session_id, timestamp, voltage, current, power, energy_kwh)
		VALUES ('pr-1', 'test-session', datetime('now'), 230, 2.6, 600, 1.5)`)
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO soc_snapshots (id, session_id, timestamp, soc_percent)
		VALUES ('snap-1', 'test-session', datetime('now'), 45.0)`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	repo := NewChargeSessionRepository(db)

	err = repo.Delete(t.Context(), "test-session")
	assert.NoError(t, err)

	// Verify session is gone
	err = db.QueryRow(`SELECT id FROM charge_sessions WHERE id = ?`, "test-session").Scan(new(string))
	assert.ErrorIs(t, err, sql.ErrNoRows)

	// Verify power readings are cascaded
	err = db.QueryRow(`SELECT id FROM power_readings WHERE session_id = ?`, "test-session").Scan(new(string))
	assert.ErrorIs(t, err, sql.ErrNoRows)

	// Verify SOC snapshots are cascaded
	err = db.QueryRow(`SELECT id FROM soc_snapshots WHERE session_id = ?`, "test-session").Scan(new(string))
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestChargeSessionRepository_Delete_NotFound(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)
	err := repo.Delete(t.Context(), "nonexistent-session")
	assert.NoError(t, err)
}

func TestChargeSessionRepository_Delete_Error(t *testing.T) {
	db := setupChargeSessionDB(t)
	repo := NewChargeSessionRepository(db)
	db.Close()
	err := repo.Delete(t.Context(), "any-id")
	assert.Error(t, err)
}

func TestScanChargeSession_FromRow(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38,
		TargetKwh: 1.9, StartPercent: 20, TargetPercent: 80,
		Status: "active",
	}
	require.NoError(t, repo.Create(t.Context(), session))

	endedAt := time.Now()
	endKwh := 1.5
	endPercent := 75.0
	require.NoError(t, repo.UpdateEndWithStats(t.Context(), session.ID, endedAt, endKwh, endPercent, 1.12, 1.4, 300, nil, 0, 0))

	row := db.QueryRow(`SELECT `+chargeSessionColumns+`
		FROM charge_sessions WHERE id = ?`, session.ID)

	var s models.ChargeSession
	err := scanChargeSession(&s, row)
	assert.NoError(t, err)
	assert.Equal(t, session.ID, s.ID)
	assert.Equal(t, "rm1", s.VehicleID)
	assert.Equal(t, 0.38, s.StartKwh)
	assert.Equal(t, 1.9, s.TargetKwh)
	assert.Equal(t, 20.0, s.StartPercent)
	assert.Equal(t, 80.0, s.TargetPercent)
	assert.NotNil(t, s.EndKwh)
	assert.Equal(t, endKwh, *s.EndKwh)
	assert.NotNil(t, s.EndPercent)
	assert.Equal(t, endPercent, *s.EndPercent)
	assert.NotNil(t, s.EndedAt)
}

func TestScanChargeSession_FromRows(t *testing.T) {
	db := setupChargeSessionDBFull(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Create two sessions with distinct timestamps (rm1 exists from seed data)
	s1 := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Now().Add(-2 * time.Hour), StartKwh: 0.38,
		TargetKwh: 2.026, StartPercent: 20, TargetPercent: 80,
		Status: "completed",
	}
	s2 := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		CreatedAt: time.Now().Add(-1 * time.Hour), StartKwh: 0.5,
		TargetKwh: 2.026, StartPercent: 25, TargetPercent: 90,
		Status: "active",
	}
	require.NoError(t, repo.Create(t.Context(), s1))
	require.NoError(t, repo.Create(t.Context(), s2))

	rows, err := db.Query(`SELECT ` + chargeSessionColumns + `
		FROM charge_sessions ORDER BY created_at DESC`)
	require.NoError(t, err)
	defer rows.Close()

	var sessions []models.ChargeSession
	for rows.Next() {
		var s models.ChargeSession
		err := scanChargeSession(&s, rows)
		require.NoError(t, err)
		sessions = append(sessions, s)
	}
	require.NoError(t, rows.Err())
	assert.Len(t, sessions, 2)
	assert.Equal(t, s2.ID, sessions[0].ID)
	assert.Equal(t, s1.ID, sessions[1].ID)
	assert.Equal(t, 25.0, sessions[0].StartPercent)
	assert.Equal(t, 20.0, sessions[1].StartPercent)
}

func TestScanChargeSession_NullFields(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	// Insert with NULL nullable fields
	startTime := time.Now().Add(-time.Hour)
	require.NoError(t, testdb.InsertChargeSession(db, &testdb.ChargeSessionOpts{
		ID:        "null-test",
		VehicleID: "rm1",
		UserID:    "test-user",
		PlugID:    "test-plug",
		Status:    "active",
		CreatedAt: startTime,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		StartPct:  20,
		TargetPct: 80,
	}))

	row := db.QueryRow(`SELECT `+chargeSessionColumns+`
		FROM charge_sessions WHERE id = ?`, "null-test")

	var s models.ChargeSession
	err := scanChargeSession(&s, row)
	assert.NoError(t, err)
	assert.Nil(t, s.EndedAt)
	assert.Nil(t, s.EndKwh)
	assert.Nil(t, s.EndPercent)
	assert.Nil(t, s.StartTotalKwh)
	assert.Nil(t, s.StartedAt)
}

func TestChargeSessionRepository_GetPending(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// No pending session
	pending, err := repo.GetPending(t.Context())
	assert.NoError(t, err)
	assert.Nil(t, pending)

	// Create pending session
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusPending,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	// Get pending should return the session
	pending, err = repo.GetPending(t.Context())
	assert.NoError(t, err)
	assert.NotNil(t, pending)
	assert.Equal(t, session.ID, pending.ID)
	assert.Equal(t, models.SessionStatusPending, pending.Status)
}

func TestChargeSessionRepository_ActivatePending(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Create pending session
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusPending,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	// Activate the pending session
	startedAt := time.Now()
	err := repo.ActivatePending(t.Context(), session.ID, startedAt)
	assert.NoError(t, err)

	// Verify session is now active
	updated, err := repo.FindByID(t.Context(), session.ID)
	assert.NoError(t, err)
	assert.NotNil(t, updated)
	assert.Equal(t, models.SessionStatusActive, updated.Status)
	assert.NotNil(t, updated.StartedAt)
}

func TestChargeSessionRepository_ActivatePending_WrongState(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Create active session (not pending)
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	// Try to activate - should fail with ErrSessionWrongState
	err := repo.ActivatePending(t.Context(), session.ID, time.Now())
	assert.Error(t, err)
	assert.Equal(t, ErrSessionWrongState, err)
}

func TestChargeSessionRepository_ResumeHolding(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	holdPercent := 64.0
	session := &models.ChargeSession{
		VehicleID:   "rm1",
		UserID:      repoTestUserIDPtr,
		PlugID:      repoTestPlugIDPtr,
		StartKwh:    0.38,
		TargetKwh:   1.9,
		Status:      models.SessionStatusHolding,
		HoldPercent: &holdPercent,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	err := repo.ResumeHolding(t.Context(), session.ID)
	assert.NoError(t, err)

	updated, err := repo.FindByID(t.Context(), session.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, models.SessionStatusActive, updated.Status)
	assert.Nil(t, updated.HoldPercent)
}

func TestChargeSessionRepository_ResumeHolding_WrongState(t *testing.T) {
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

	err := repo.ResumeHolding(t.Context(), session.ID)
	assert.Equal(t, ErrSessionWrongState, err)
}

func TestChargeSessionRepository_UpdateStartTotalKwh(t *testing.T) {
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

	err := repo.UpdateStartTotalKwh(t.Context(), session.ID, 123.45)
	assert.NoError(t, err)

	updated, err := repo.FindByID(t.Context(), session.ID)
	assert.NoError(t, err)
	assert.NotNil(t, updated)
	assert.NotNil(t, updated.StartTotalKwh)
	assert.Equal(t, 123.45, *updated.StartTotalKwh)
}

func TestChargeSessionRepository_UpdateEndedAt(t *testing.T) {
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

	endedAt := time.Now()
	err := repo.UpdateEndedAt(t.Context(), session.ID, endedAt)
	assert.NoError(t, err)

	updated, err := repo.FindByID(t.Context(), session.ID)
	assert.NoError(t, err)
	assert.NotNil(t, updated)
	assert.NotNil(t, updated.EndedAt)
}

func TestChargeSessionRepository_UpdateCancelData(t *testing.T) {
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

	endedAt := time.Now()
	err := repo.UpdateCancelData(t.Context(), session.ID, endedAt, nil)
	assert.NoError(t, err)

	updated, err := repo.FindByID(t.Context(), session.ID)
	assert.NoError(t, err)
	assert.NotNil(t, updated)
	assert.Equal(t, models.SessionStatusCancelled, updated.Status)
	assert.NotNil(t, updated.EndedAt)
}

func TestChargeSessionRepository_CancelPending(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Create pending session
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusPending,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	endedAt := time.Now()
	err := repo.CancelPending(t.Context(), session.ID, endedAt)
	assert.NoError(t, err)

	// Verify session is now cancelled
	updated, err := repo.FindByID(t.Context(), session.ID)
	assert.NoError(t, err)
	assert.NotNil(t, updated)
	assert.Equal(t, models.SessionStatusCancelled, updated.Status)
	assert.NotNil(t, updated.EndedAt)
}

func TestChargeSessionRepository_CancelPending_WrongState(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Create active session (not pending)
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	// Try to cancel - should fail with ErrSessionWrongState
	err := repo.CancelPending(t.Context(), session.ID, time.Now())
	assert.Error(t, err)
	assert.Equal(t, ErrSessionWrongState, err)
}

func TestChargeSessionRepository_UpdateLastBlendedKwh(t *testing.T) {
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

	err := repo.UpdateLastBlendedKwh(t.Context(), session.ID, 0.75)
	assert.NoError(t, err)

	updated, err := repo.FindByID(t.Context(), session.ID)
	assert.NoError(t, err)
	assert.NotNil(t, updated)
	assert.NotNil(t, updated.LastBlendedKwh)
	assert.Equal(t, 0.75, *updated.LastBlendedKwh)
}

func TestChargeSessionRepository_UpdateStatus_Conditioning(t *testing.T) {
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

	err := repo.UpdateStatus(t.Context(), session.ID, models.SessionStatusConditioning)
	assert.NoError(t, err)

	updated, err := repo.FindByID(t.Context(), session.ID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusConditioning, updated.Status)
}

func TestChargeSessionRepository_GetActive_IncludesConditioning(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    models.SessionStatusConditioning,
	}
	require.NoError(t, repo.Create(t.Context(), session))

	active, err := repo.GetActive(t.Context())
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Equal(t, session.ID, active.ID)
	assert.Equal(t, models.SessionStatusConditioning, active.Status)
}

func TestChargeSessionRepository_CreatePowerReading_WithCarbonIntensity(t *testing.T) {
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

	ci := 123.5
	reading := &models.PowerReading{
		ID:                        "pr_ci_test",
		SessionID:                 session.ID,
		Timestamp:                 time.Now(),
		Voltage:                   230,
		Current:                   2.6,
		Power:                     600,
		EnergyKwh:                 0.5,
		CarbonIntensityGCo2PerKwh: &ci,
	}
	require.NoError(t, repo.CreatePowerReading(t.Context(), reading))

	readings, err := repo.GetPowerReadings(t.Context(), session.ID)
	require.NoError(t, err)
	require.Len(t, readings, 1)
	require.NotNil(t, readings[0].CarbonIntensityGCo2PerKwh)
	assert.InDelta(t, 123.5, *readings[0].CarbonIntensityGCo2PerKwh, 0.001)
}

func TestChargeSessionRepository_GetPowerReadings_NullCarbonIntensity(t *testing.T) {
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

	reading := &models.PowerReading{
		ID:        "pr_no_ci",
		SessionID: session.ID,
		Timestamp: time.Now(),
		Voltage:   230,
		Current:   2.6,
		Power:     600,
		EnergyKwh: 0.5,
		// CarbonIntensityGCo2PerKwh intentionally nil
	}
	require.NoError(t, repo.CreatePowerReading(t.Context(), reading))

	readings, err := repo.GetPowerReadings(t.Context(), session.ID)
	require.NoError(t, err)
	require.Len(t, readings, 1)
	assert.Nil(t, readings[0].CarbonIntensityGCo2PerKwh)
}

func TestChargeSessionRepository_GetAvgCarbonIntensityForSessions(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	sess1 := &models.ChargeSession{VehicleID: "rm1", UserID: repoTestUserIDPtr, PlugID: repoTestPlugIDPtr, StartKwh: 0.38, TargetKwh: 1.9, Status: models.SessionStatusCompleted}
	sess2 := &models.ChargeSession{VehicleID: "rm1", UserID: repoTestUserIDPtr, PlugID: repoTestPlugIDPtr, StartKwh: 0.38, TargetKwh: 1.9, Status: models.SessionStatusCompleted}
	sess3 := &models.ChargeSession{VehicleID: "rm1", UserID: repoTestUserIDPtr, PlugID: repoTestPlugIDPtr, StartKwh: 0.38, TargetKwh: 1.9, Status: models.SessionStatusCompleted}
	require.NoError(t, repo.Create(t.Context(), sess1))
	require.NoError(t, repo.Create(t.Context(), sess2))
	require.NoError(t, repo.Create(t.Context(), sess3))

	ci100 := 100.0
	ci200 := 200.0
	ts := time.Now()

	// sess1: two readings with CI → avg = 150
	require.NoError(t, repo.CreatePowerReading(t.Context(), &models.PowerReading{
		ID: "r1a", SessionID: sess1.ID, Timestamp: ts, Power: 600, EnergyKwh: 0.5,
		CarbonIntensityGCo2PerKwh: &ci100,
	}))
	require.NoError(t, repo.CreatePowerReading(t.Context(), &models.PowerReading{
		ID: "r1b", SessionID: sess1.ID, Timestamp: ts, Power: 600, EnergyKwh: 0.5,
		CarbonIntensityGCo2PerKwh: &ci200,
	}))

	// sess2: no readings → not in result map
	// sess3: one reading with nil CI → not in result map
	require.NoError(t, repo.CreatePowerReading(t.Context(), &models.PowerReading{
		ID: "r3a", SessionID: sess3.ID, Timestamp: ts, Power: 600, EnergyKwh: 0.5,
	}))

	result, err := repo.GetAvgCarbonIntensityForSessions(t.Context(), []string{sess1.ID, sess2.ID, sess3.ID})
	require.NoError(t, err)

	require.Contains(t, result, sess1.ID)
	assert.InDelta(t, 150.0, *result[sess1.ID], 0.001)
	assert.NotContains(t, result, sess2.ID)
	assert.NotContains(t, result, sess3.ID)
}

func TestChargeSessionRepository_GetAvgCarbonIntensityForSessions_Empty(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	result, err := repo.GetAvgCarbonIntensityForSessions(t.Context(), []string{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestChargeSessionRepository_GetActiveByPlug(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	userID := "test-user"
	plugID := "plug-test-123"
	require.NoError(t, testdb.InsertPlug(db, plugID, userID, "Test Plug", "ns-test", "test"))

	ctx := internal.WithUserID(t.Context(), userID)
	active, err := repo.GetActiveByPlug(ctx, plugID)
	assert.NoError(t, err)
	assert.Nil(t, active)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    &userID,
		PlugID:    &plugID,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    "active",
	}
	require.NoError(t, repo.Create(t.Context(), session))

	active, err = repo.GetActiveByPlug(ctx, plugID)
	assert.NoError(t, err)
	assert.NotNil(t, active)
	assert.Equal(t, session.ID, active.ID)

	active, err = repo.GetActiveByPlug(ctx, "other-plug")
	assert.NoError(t, err)
	assert.Nil(t, active)
}

func TestChargeSessionRepository_GetPendingByPlug(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	userID := "test-user"
	plugID := "plug-pending-123"
	require.NoError(t, testdb.InsertPlug(db, plugID, userID, "Pending Plug", "ns-pending", "pending"))

	ctx := internal.WithUserID(t.Context(), userID)

	pending, err := repo.GetPendingByPlug(ctx, plugID)
	assert.NoError(t, err)
	assert.Nil(t, pending)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    &userID,
		PlugID:    &plugID,
		StartKwh: 0.38, TargetKwh: 1.9, Status: models.SessionStatusPending,

	}
	require.NoError(t, repo.Create(t.Context(), session))

	pending, err = repo.GetPendingByPlug(ctx, plugID)
	assert.NoError(t, err)
	assert.NotNil(t, pending)
	assert.Equal(t, session.ID, pending.ID)
	assert.Equal(t, models.SessionStatusPending, pending.Status)
}

func TestChargeSessionRepository_GetLastSOCSnapshot(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	snap, err := repo.GetLastSOCSnapshot(t.Context(), "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, snap)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38, TargetKwh: 1.9, Status: "active",
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ts := time.Now()
	require.NoError(t, repo.CreateSOCSnapshot(t.Context(), &models.SOCSnapshot{
		ID: "snap1", SessionID: session.ID, SocPercent: 30, Timestamp: ts,
	}))
	require.NoError(t, repo.CreateSOCSnapshot(t.Context(), &models.SOCSnapshot{
		ID: "snap2", SessionID: session.ID, SocPercent: 50, Timestamp: ts.Add(time.Minute),
	}))

	snap, err = repo.GetLastSOCSnapshot(t.Context(), session.ID)
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Equal(t, "snap2", snap.ID)
	assert.Equal(t, 50.0, snap.SocPercent)
}

func TestChargeSessionRepository_GetLastPowerReading(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	reading, err := repo.GetLastPowerReading(t.Context(), "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, reading)

	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh: 0.38, TargetKwh: 1.9, Status: "active",
	}
	require.NoError(t, repo.Create(t.Context(), session))

	ts := time.Now()
	require.NoError(t, repo.CreatePowerReading(t.Context(), &models.PowerReading{
		ID: "pr1", SessionID: session.ID, Timestamp: ts, Voltage: 230, Current: 2.6, Power: 600, EnergyKwh: 0.5,
	}))
	require.NoError(t, repo.CreatePowerReading(t.Context(), &models.PowerReading{
		ID: "pr2", SessionID: session.ID, Timestamp: ts.Add(time.Minute), Voltage: 231, Current: 2.7, Power: 620, EnergyKwh: 0.6,
	}))

	reading, err = repo.GetLastPowerReading(t.Context(), session.ID)
	assert.NoError(t, err)
	assert.NotNil(t, reading)
	assert.Equal(t, "pr2", reading.ID)
	assert.Equal(t, 620.0, reading.Power)
}

func TestChargeSessionRepository_GetSessionAggregates_Lifetime(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Insert 3 completed sessions with stats
	now := time.Now()
	for i, batteryKwh := range []float64{1.5, 2.0, 1.0} {
		wallKwh := batteryKwh / 0.8
		co2Grams := wallKwh * 250
		s := &models.ChargeSession{
			VehicleID: "rm1",
			UserID:    repoTestUserIDPtr,
			PlugID:    repoTestPlugIDPtr,
			StartKwh: 0.38, Status: "completed",
			CreatedAt: now.AddDate(0, 0, -i),
		}
		require.NoError(t, repo.Create(t.Context(), s))
		endKwh := s.StartKwh + batteryKwh
		costPence := wallKwh * 24
		require.NoError(t, repo.UpdateEndWithStats(t.Context(), s.ID, s.CreatedAt, endKwh, 80, batteryKwh, wallKwh, co2Grams, nil, costPence, 0))
	}

	agg, err := repo.GetSessionAggregates(t.Context(), "rm1", time.Time{})
	require.NoError(t, err)
	require.NotNil(t, agg)
	assert.Equal(t, 3, agg.TotalSessions)
	assert.InDelta(t, 4.5, agg.TotalBatteryKwh, 0.01)
	assert.InDelta(t, 4.5/0.8, agg.TotalWallKwh, 0.01)
	assert.InDelta(t, (4.5/0.8)*250, agg.TotalCo2Grams, 0.01)
	assert.InDelta(t, (4.5/0.8)*24, agg.TotalCostPence, 0.01)
	assert.NotNil(t, agg.AvgCarbonGCo2Kwh)
	assert.InDelta(t, 250.0, *agg.AvgCarbonGCo2Kwh, 0.01)
}

func TestChargeSessionRepository_GetSessionAggregates_TimeRange(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	now := time.Now()
	// Session 1 day ago (in range)
	s1 := &models.ChargeSession{
		VehicleID: "rm1", UserID: repoTestUserIDPtr, PlugID: repoTestPlugIDPtr,
		StartKwh: 0.38, Status: "completed", CreatedAt: now.Add(-24 * time.Hour),
	}
	require.NoError(t, repo.Create(t.Context(), s1))
	require.NoError(t, repo.UpdateEndWithStats(t.Context(), s1.ID, s1.CreatedAt, 1.88, 80, 1.5, 1.875, 468.75, nil, 0, 0))

	// Session 10 days ago (out of 7-day range)
	s2 := &models.ChargeSession{
		VehicleID: "rm1", UserID: repoTestUserIDPtr, PlugID: repoTestPlugIDPtr,
		StartKwh: 0.5, Status: "completed", CreatedAt: now.AddDate(0, 0, -10),
	}
	require.NoError(t, repo.Create(t.Context(), s2))
	require.NoError(t, repo.UpdateEndWithStats(t.Context(), s2.ID, s2.CreatedAt, 2.5, 80, 2.0, 2.5, 625, nil, 0, 0))

	cutoff := now.Add(-7 * 24 * time.Hour)
	agg, err := repo.GetSessionAggregates(t.Context(), "rm1", cutoff)
	require.NoError(t, err)
	require.NotNil(t, agg)
	assert.Equal(t, 1, agg.TotalSessions)
	assert.InDelta(t, 1.5, agg.TotalBatteryKwh, 0.01)
}

func TestChargeSessionRepository_GetSessionAggregates_ExcludesUnfinished(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	now := time.Now()
	// Completed session with stats
	s1 := &models.ChargeSession{
		VehicleID: "rm1", UserID: repoTestUserIDPtr, PlugID: repoTestPlugIDPtr,
		StartKwh: 0.38, Status: "completed", CreatedAt: now,
	}
	require.NoError(t, repo.Create(t.Context(), s1))
	require.NoError(t, repo.UpdateEndWithStats(t.Context(), s1.ID, s1.CreatedAt, 1.88, 80, 1.5, 1.875, 468.75, nil, 0, 0))

	// Active session (no stats)
	s2 := &models.ChargeSession{
		VehicleID: "rm1", UserID: repoTestUserIDPtr, PlugID: repoTestPlugIDPtr,
		StartKwh: 0.5, Status: "active", CreatedAt: now,
	}
	require.NoError(t, repo.Create(t.Context(), s2))

	// Completed session without stats (old session, NULL battery_kwh)
	s3 := &models.ChargeSession{
		VehicleID: "rm1", UserID: repoTestUserIDPtr, PlugID: repoTestPlugIDPtr,
		StartKwh: 0.3, Status: "completed", CreatedAt: now.Add(-48 * time.Hour),
	}
	require.NoError(t, repo.Create(t.Context(), s3))

	agg, err := repo.GetSessionAggregates(t.Context(), "rm1", time.Time{})
	require.NoError(t, err)
	require.NotNil(t, agg)
	assert.Equal(t, 1, agg.TotalSessions)
	assert.InDelta(t, 1.5, agg.TotalBatteryKwh, 0.01)
}

func TestChargeSessionRepository_GetSessionAggregates_NoSessions(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)
	agg, err := repo.GetSessionAggregates(t.Context(), "rm1", time.Time{})
	require.NoError(t, err)
	require.NotNil(t, agg)
	assert.Equal(t, 0, agg.TotalSessions)
	assert.Equal(t, float64(0), agg.TotalBatteryKwh)
	assert.Nil(t, agg.AvgCarbonGCo2Kwh)
}

func TestChargeSessionRepository_GetDailyEnergy_GroupsByDate(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	// Use UTC midnight anchors so SQLite's DATE() and Go's Format("2006-01-02")
	// always agree regardless of the host/container timezone.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	day1Time := today.AddDate(0, 0, -2).Add(10 * time.Hour) // 10:00 UTC, 2 days ago
	day2Time := today.AddDate(0, 0, -1).Add(10 * time.Hour) // 10:00 UTC, 1 day ago

	// Day 1: 2 sessions
	for _, batteryKwh := range []float64{1.0, 0.5} {
		wallKwh := batteryKwh / 0.8
		co2Grams := wallKwh * 200
		s := &models.ChargeSession{
			VehicleID: "rm1", UserID: repoTestUserIDPtr, PlugID: repoTestPlugIDPtr,
			StartKwh: 0.38, Status: "completed", CreatedAt: day1Time,
		}
		require.NoError(t, repo.Create(t.Context(), s))
		endKwh := s.StartKwh + batteryKwh
		require.NoError(t, repo.UpdateEndWithStats(t.Context(), s.ID, s.CreatedAt, endKwh, 80, batteryKwh, wallKwh, co2Grams, nil, 0, 0))
	}

	// Day 2: 1 session
	{
		batteryKwh := 2.0
		wallKwh := batteryKwh / 0.8
		co2Grams := wallKwh * 300
		s := &models.ChargeSession{
			VehicleID: "rm1", UserID: repoTestUserIDPtr, PlugID: repoTestPlugIDPtr,
			StartKwh: 0.38, Status: "completed", CreatedAt: day2Time,
		}
		require.NoError(t, repo.Create(t.Context(), s))
		endKwh := s.StartKwh + batteryKwh
		require.NoError(t, repo.UpdateEndWithStats(t.Context(), s.ID, s.CreatedAt, endKwh, 80, batteryKwh, wallKwh, co2Grams, nil, 0, 0))
	}

	daily, err := repo.GetDailyEnergy(t.Context(), "rm1", time.Time{})
	require.NoError(t, err)
	require.Len(t, daily, 2)

	// Sorted ascending by date
	day1 := daily[0]
	day2 := daily[1]

	assert.Equal(t, day1Time.Format("2006-01-02"), day1.Date)
	assert.InDelta(t, 1.5, day1.BatteryKwh, 0.01)
	assert.Equal(t, 2, day1.SessionCount)
	assert.InDelta(t, (1.5/0.8)*200, day1.Co2Grams, 0.01)
	assert.NotNil(t, day1.AvgCarbonIntensityGCo2PerKwh)
	assert.InDelta(t, 200.0, *day1.AvgCarbonIntensityGCo2PerKwh, 0.01)

	assert.Equal(t, day2Time.Format("2006-01-02"), day2.Date)
	assert.InDelta(t, 2.0, day2.BatteryKwh, 0.01)
	assert.Equal(t, 1, day2.SessionCount)
	assert.InDelta(t, (2.0/0.8)*300, day2.Co2Grams, 0.01)
}

func TestChargeSessionRepository_GetDailyEnergy_TimeRange(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	now := time.Now()

	// Session in range
	s1 := &models.ChargeSession{
		VehicleID: "rm1", UserID: repoTestUserIDPtr, PlugID: repoTestPlugIDPtr,
		StartKwh: 0.38, Status: "completed", CreatedAt: now.Add(-24 * time.Hour),
	}
	require.NoError(t, repo.Create(t.Context(), s1))
	require.NoError(t, repo.UpdateEndWithStats(t.Context(), s1.ID, s1.CreatedAt, 1.88, 80, 1.5, 1.875, 468.75, nil, 0, 0))

	// Session out of range
	s2 := &models.ChargeSession{
		VehicleID: "rm1", UserID: repoTestUserIDPtr, PlugID: repoTestPlugIDPtr,
		StartKwh: 0.38, Status: "completed", CreatedAt: now.AddDate(0, 0, -10),
	}
	require.NoError(t, repo.Create(t.Context(), s2))
	require.NoError(t, repo.UpdateEndWithStats(t.Context(), s2.ID, s2.CreatedAt, 2.38, 80, 2.0, 2.5, 625, nil, 0, 0))

	cutoff := now.Add(-7 * 24 * time.Hour)
	daily, err := repo.GetDailyEnergy(t.Context(), "rm1", cutoff)
	require.NoError(t, err)
	require.Len(t, daily, 1)
	assert.InDelta(t, 1.5, daily[0].BatteryKwh, 0.01)
}

func TestChargeSessionRepository_GetDailyEnergy_Empty(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)
	daily, err := repo.GetDailyEnergy(t.Context(), "rm1", time.Time{})
	require.NoError(t, err)
	assert.Empty(t, daily)
}

func TestChargeSessionRepository_GetVehicleChargingEfficiency(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)
	eff, err := repo.GetVehicleChargingEfficiency(t.Context(), "rm1")
	require.NoError(t, err)
	assert.InDelta(t, 0.8, eff, 0.001)
}

func TestChargeSessionRepository_GetVehicleChargingEfficiency_Missing(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)
	_, err := repo.GetVehicleChargingEfficiency(t.Context(), "nonexistent")
	assert.Error(t, err)
}

func setupMultiUserDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	testdb.SeedMultiUser(t, db)
	return db
}

func TestChargeSessionRepository_GetActive_UserIsolation(t *testing.T) {
	db := setupMultiUserDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	userA := testdb.UserID2
	userB := testdb.UserID3
	plugIDA := testdb.PlugID2
	plugIDB := testdb.PlugID3
	vehicleIDA := "rm1-" + userA
	vehicleIDB := "rm1-" + userB

	sessionA := &models.ChargeSession{
		VehicleID: vehicleIDA,
		UserID:    &userA,
		PlugID:    &plugIDA,
		StartKwh:  0.5,
		TargetKwh: 2.0,
		Status:    "active",
	}
	require.NoError(t, repo.Create(t.Context(), sessionA))

	sessionB := &models.ChargeSession{
		VehicleID: vehicleIDB,
		UserID:    &userB,
		PlugID:    &plugIDB,
		StartKwh:  1.0,
		TargetKwh: 3.0,
		Status:    "active",
	}
	require.NoError(t, repo.Create(t.Context(), sessionB))

	ctxA := internal.WithUserID(t.Context(), userA)
	gotA, err := repo.GetActive(ctxA)
	require.NoError(t, err)
	require.NotNil(t, gotA, "user A must have an active session")
	assert.Equal(t, sessionA.ID, gotA.ID, "user A must see only their own session")

	ctxB := internal.WithUserID(t.Context(), userB)
	gotB, err := repo.GetActive(ctxB)
	require.NoError(t, err)
	require.NotNil(t, gotB, "user B must have an active session")
	assert.Equal(t, sessionB.ID, gotB.ID, "user B must see only their own session")

	assert.NotEqual(t, sessionB.ID, gotA.ID, "user A must not see user B's session")
	assert.NotEqual(t, sessionA.ID, gotB.ID, "user B must not see user A's session")
}

func TestChargeSessionRepository_GetActiveByVehicle_UserIsolation(t *testing.T) {
	db := setupMultiUserDB(t)
	defer db.Close()

	repo := NewChargeSessionRepository(db)

	userA := testdb.UserID2
	userB := testdb.UserID3
	plugIDA := testdb.PlugID2
	plugIDB := testdb.PlugID3
	vehicleIDA := "rm1-" + userA
	vehicleIDB := "rm1-" + userB

	sessionA := &models.ChargeSession{
		VehicleID: vehicleIDA,
		UserID:    &userA,
		PlugID:    &plugIDA,
		StartKwh:  0.5,
		TargetKwh: 2.0,
		Status:    "active",
	}
	require.NoError(t, repo.Create(t.Context(), sessionA))

	sessionB := &models.ChargeSession{
		VehicleID: vehicleIDB,
		UserID:    &userB,
		PlugID:    &plugIDB,
		StartKwh:  1.0,
		TargetKwh: 3.0,
		Status:    "active",
	}
	require.NoError(t, repo.Create(t.Context(), sessionB))

	ctxA := internal.WithUserID(t.Context(), userA)
	gotA, err := repo.GetActiveByVehicle(ctxA, vehicleIDA)
	require.NoError(t, err)
	require.NotNil(t, gotA, "user A must see their vehicle's session")
	assert.Equal(t, sessionA.ID, gotA.ID)

	// User A context must not be able to fetch user B's vehicle session.
	crossLookup, err := repo.GetActiveByVehicle(ctxA, vehicleIDB)
	require.NoError(t, err)
	assert.Nil(t, crossLookup, "user A context must not return user B's vehicle session")

	ctxB := internal.WithUserID(t.Context(), userB)
	gotB, err := repo.GetActiveByVehicle(ctxB, vehicleIDB)
	require.NoError(t, err)
	require.NotNil(t, gotB, "user B must see their vehicle's session")
	assert.Equal(t, sessionB.ID, gotB.ID)
}

// --- ended_at timezone round-trip regressions ---
//
// ended_at used to be written with the timezone-NAIVE time.DateTime format
// while created_at carried its offset. On a server running a non-UTC zone
// (TZ=Europe/London in production), the naive wall-clock string was read back
// as UTC, shifting every cancelled/ended session's end time by the UTC offset
// - the Charge History showed seconds-long cancelled sessions as "1h 0m".

// bstNow returns the current time in a fixed +01:00 zone so these tests fail
// under any server TZ (including the UTC used in CI) if a write path drops
// the offset.
func bstNow() time.Time {
	return time.Now().In(time.FixedZone("BST", 3600)).Truncate(time.Second)
}

func insertTimeTestSession(t *testing.T, repo *ChargeSessionRepository, status string) *models.ChargeSession {
	t.Helper()
	session := &models.ChargeSession{
		VehicleID: "rm1",
		UserID:    repoTestUserIDPtr,
		PlugID:    repoTestPlugIDPtr,
		StartKwh:  0.38,
		TargetKwh: 1.9,
		Status:    status,
	}
	require.NoError(t, repo.Create(t.Context(), session))
	return session
}

func TestChargeSessionRepository_UpdateCancelData_PreservesEndedAtInstant(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()
	repo := NewChargeSessionRepository(db)
	session := insertTimeTestSession(t, repo, "active")

	endedAt := bstNow()
	require.NoError(t, repo.UpdateCancelData(t.Context(), session.ID, endedAt, nil))

	found, err := repo.FindByID(t.Context(), session.ID)
	require.NoError(t, err)
	require.NotNil(t, found.EndedAt)
	assert.Equal(t, endedAt.Unix(), found.EndedAt.Unix(),
		"cancelled session's ended_at must round-trip to the same instant regardless of server TZ")
}

func TestChargeSessionRepository_CancelPending_PreservesEndedAtInstant(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()
	repo := NewChargeSessionRepository(db)
	session := insertTimeTestSession(t, repo, models.SessionStatusPending)

	endedAt := bstNow()
	require.NoError(t, repo.CancelPending(t.Context(), session.ID, endedAt))

	found, err := repo.FindByID(t.Context(), session.ID)
	require.NoError(t, err)
	require.NotNil(t, found.EndedAt)
	assert.Equal(t, endedAt.Unix(), found.EndedAt.Unix())
}

func TestChargeSessionRepository_UpdateEndedAt_PreservesEndedAtInstant(t *testing.T) {
	db := setupChargeSessionDB(t)
	defer db.Close()
	repo := NewChargeSessionRepository(db)
	session := insertTimeTestSession(t, repo, "active")

	endedAt := bstNow()
	require.NoError(t, repo.UpdateEndedAt(t.Context(), session.ID, endedAt))

	found, err := repo.FindByID(t.Context(), session.ID)
	require.NoError(t, err)
	require.NotNil(t, found.EndedAt)
	assert.Equal(t, endedAt.Unix(), found.EndedAt.Unix())
}
