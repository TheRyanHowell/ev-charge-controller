package repository

import (
	"database/sql"
	"testing"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/testdb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupScheduleTestDB(t *testing.T) *sql.DB {
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	testdb.SeedDefaultUser(t, db)
	return db
}

func TestScheduleRepository_Get(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	plugID := "plug"
	require.NoError(t, testdb.InsertPlug(db, plugID, "test-user", "Test Plug", "ns-test", "test"))
	require.NoError(t, testdb.InsertSchedule(db, &testdb.ScheduleOpts{
		ID:     "plug",
		PlugID: plugID,
		UserID: "test-user",
		Time:   "03:00",
		Enabled: true,
	}))

	schedule, err := repo.Get(t.Context())
	require.NoError(t, err)
	require.NotNil(t, schedule)
	assert.Equal(t, "plug", schedule.ID)
	assert.Equal(t, "03:00", schedule.Time)
	assert.True(t, schedule.Enabled)
}

func TestScheduleRepository_Get_ReadyBy(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	plugID := "plug"
	require.NoError(t, testdb.InsertPlug(db, plugID, "test-user", "Test Plug", "ns-test", "test"))
	require.NoError(t, testdb.InsertSchedule(db, &testdb.ScheduleOpts{
		ID:      "plug",
		PlugID:  plugID,
		UserID:  "test-user",
		Time:    "03:00",
		ReadyBy: "07:00",
		Enabled: true,
	}))

	schedule, err := repo.Get(t.Context())
	require.NoError(t, err)
	require.NotNil(t, schedule)
	require.NotNil(t, schedule.ReadyBy)
	assert.Equal(t, "07:00", *schedule.ReadyBy)
}

func TestScheduleRepository_Get_ReadyByNull(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	plugID := "plug"
	require.NoError(t, testdb.InsertPlug(db, plugID, "test-user", "Test Plug", "ns-test", "test"))
	require.NoError(t, testdb.InsertSchedule(db, &testdb.ScheduleOpts{
		ID:      "plug",
		PlugID:  plugID,
		UserID:  "test-user",
		Time:    "03:00",
		Enabled: true,
	}))

	schedule, err := repo.Get(t.Context())
	require.NoError(t, err)
	require.NotNil(t, schedule)
	assert.Nil(t, schedule.ReadyBy)
}

func TestScheduleRepository_Get_TwoStage(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	plugID := "plug"
	require.NoError(t, testdb.InsertPlug(db, plugID, "test-user", "Test Plug", "ns-test", "test"))
	require.NoError(t, testdb.InsertSchedule(db, &testdb.ScheduleOpts{
		ID:       "plug",
		PlugID:   plugID,
		UserID:   "test-user",
		Time:     "03:00",
		TwoStage: true,
		Enabled:  true,
	}))

	schedule, err := repo.Get(t.Context())
	require.NoError(t, err)
	require.NotNil(t, schedule)
	assert.True(t, schedule.TwoStage)
}

func TestScheduleRepository_Get_TwoStageDefaultsFalse(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	plugID := "plug"
	require.NoError(t, testdb.InsertPlug(db, plugID, "test-user", "Test Plug", "ns-test", "test"))
	require.NoError(t, testdb.InsertSchedule(db, &testdb.ScheduleOpts{
		ID:      "plug",
		PlugID:  plugID,
		UserID:  "test-user",
		Time:    "03:00",
		Enabled: true,
	}))

	schedule, err := repo.Get(t.Context())
	require.NoError(t, err)
	require.NotNil(t, schedule)
	assert.False(t, schedule.TwoStage)
}

func TestScheduleRepository_Get_Empty(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	schedule, err := repo.Get(t.Context())
	require.NoError(t, err)
	assert.Nil(t, schedule)
}

func TestScheduleRepository_Upsert_Insert(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	plugID := "plug"
	userID := "test-user"
	require.NoError(t, testdb.InsertPlug(db, plugID, userID, "Test Plug", "ns-test", "test"))
	schedule := &models.Schedule{
		ID:      "plug",
		PlugID:  &plugID,
		UserID:  &userID,
		Time:    "02:30",
		Enabled: true,
	}

	err := repo.Upsert(t.Context(), schedule)
	require.NoError(t, err)

	// Verify
	found, err := repo.Get(t.Context())
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "02:30", found.Time)
	assert.True(t, found.Enabled)
}

func TestScheduleRepository_Upsert_ReadyBy(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	plugID := "plug"
	userID := "test-user"
	require.NoError(t, testdb.InsertPlug(db, plugID, userID, "Test Plug", "ns-test", "test"))
	readyBy := "07:00"
	schedule := &models.Schedule{
		ID:      "plug",
		PlugID:  &plugID,
		UserID:  &userID,
		Time:    "02:30",
		ReadyBy: &readyBy,
		Enabled: true,
	}

	err := repo.Upsert(t.Context(), schedule)
	require.NoError(t, err)

	found, err := repo.Get(t.Context())
	require.NoError(t, err)
	require.NotNil(t, found)
	require.NotNil(t, found.ReadyBy)
	assert.Equal(t, "07:00", *found.ReadyBy)

	// Upsert again clearing readyBy - must null the column, not leave it stale.
	schedule.ReadyBy = nil
	err = repo.Upsert(t.Context(), schedule)
	require.NoError(t, err)

	found, err = repo.Get(t.Context())
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Nil(t, found.ReadyBy)
}

func TestScheduleRepository_Upsert_TwoStage(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	plugID := "plug"
	userID := "test-user"
	require.NoError(t, testdb.InsertPlug(db, plugID, userID, "Test Plug", "ns-test", "test"))
	schedule := &models.Schedule{
		ID:       "plug",
		PlugID:   &plugID,
		UserID:   &userID,
		Time:     "02:30",
		TwoStage: true,
		Enabled:  true,
	}

	err := repo.Upsert(t.Context(), schedule)
	require.NoError(t, err)

	found, err := repo.Get(t.Context())
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.True(t, found.TwoStage)

	// Upsert again with twoStage off - must clear the flag, not leave it stale.
	schedule.TwoStage = false
	err = repo.Upsert(t.Context(), schedule)
	require.NoError(t, err)

	found, err = repo.Get(t.Context())
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.False(t, found.TwoStage)
}

func TestScheduleRepository_Upsert_Update(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	plugID := "plug"
	userID := "test-user"
	require.NoError(t, testdb.InsertPlug(db, plugID, userID, "Test Plug", "ns-test", "test"))
	require.NoError(t, testdb.InsertSchedule(db, &testdb.ScheduleOpts{
		ID:      "plug",
		PlugID:  plugID,
		UserID:  userID,
		Time:    "03:00",
		Enabled: true,
	}))

	// Update with new time
	updated := &models.Schedule{
		ID:      "plug",
		PlugID:  &plugID,
		UserID:  &userID,
		Time:    "04:00",
		Enabled: false,
	}

	err := repo.Upsert(t.Context(), updated)
	require.NoError(t, err)

	found, err := repo.Get(t.Context())
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "04:00", found.Time)
	assert.False(t, found.Enabled)
}

func TestScheduleRepository_GetByPlugID(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	plugID := "plug-sched-123"
	require.NoError(t, testdb.InsertPlug(db, plugID, "test-user", "Test", "ns", "t"))
	require.NoError(t, testdb.InsertSchedule(db, &testdb.ScheduleOpts{
		ID:      "s1",
		PlugID:  plugID,
		UserID:  "test-user",
		Time:    "05:00",
		Enabled: true,
	}))

	sched, err := repo.GetByPlugID(t.Context(), plugID)
	require.NoError(t, err)
	require.NotNil(t, sched)
	assert.Equal(t, "05:00", sched.Time)
	assert.True(t, sched.Enabled)

	sched, err = repo.GetByPlugID(t.Context(), "nonexistent-plug")
	require.NoError(t, err)
	assert.Nil(t, sched)
}

func TestScheduleRepository_UpsertByPlugID(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	plugID := "plug-upsert-123"
	userID := "test-user"
	require.NoError(t, testdb.InsertPlug(db, plugID, userID, "Test", "ns", "t"))

	sched := &models.Schedule{
		ID:      "s1",
		PlugID:  &plugID,
		UserID:  &userID,
		Time:    "06:00",
		Enabled: true,
	}

	err := repo.UpsertByPlugID(t.Context(), sched)
	require.NoError(t, err)

	found, err := repo.GetByPlugID(t.Context(), plugID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "06:00", found.Time)

	// Upsert again to update
	sched.Time = "07:00"
	sched.Enabled = false
	err = repo.UpsertByPlugID(t.Context(), sched)
	require.NoError(t, err)

	found, err = repo.GetByPlugID(t.Context(), plugID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "07:00", found.Time)
	assert.False(t, found.Enabled)
}

func TestScheduleRepository_UpsertByPlugID_ReadyBy(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	plugID := "plug-upsert-readyby"
	userID := "test-user"
	require.NoError(t, testdb.InsertPlug(db, plugID, userID, "Test", "ns", "t"))

	readyBy := "07:00"
	sched := &models.Schedule{
		ID:      "s1",
		PlugID:  &plugID,
		UserID:  &userID,
		Time:    "06:00",
		ReadyBy: &readyBy,
		Enabled: true,
	}

	err := repo.UpsertByPlugID(t.Context(), sched)
	require.NoError(t, err)

	found, err := repo.GetByPlugID(t.Context(), plugID)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.NotNil(t, found.ReadyBy)
	assert.Equal(t, "07:00", *found.ReadyBy)

	// Upsert again clearing readyBy - must null the column.
	sched.ReadyBy = nil
	err = repo.UpsertByPlugID(t.Context(), sched)
	require.NoError(t, err)

	found, err = repo.GetByPlugID(t.Context(), plugID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Nil(t, found.ReadyBy)
}

func TestScheduleRepository_UpsertByPlugID_TwoStage(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	plugID := "plug-upsert-twostage"
	userID := "test-user"
	require.NoError(t, testdb.InsertPlug(db, plugID, userID, "Test", "ns", "t"))

	sched := &models.Schedule{
		ID:       "s1",
		PlugID:   &plugID,
		UserID:   &userID,
		Time:     "06:00",
		TwoStage: true,
		Enabled:  true,
	}

	err := repo.UpsertByPlugID(t.Context(), sched)
	require.NoError(t, err)

	found, err := repo.GetByPlugID(t.Context(), plugID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.True(t, found.TwoStage)

	// Upsert again with twoStage off - must clear the flag.
	sched.TwoStage = false
	err = repo.UpsertByPlugID(t.Context(), sched)
	require.NoError(t, err)

	found, err = repo.GetByPlugID(t.Context(), plugID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.False(t, found.TwoStage)
}

func TestScheduleRepository_ListAll(t *testing.T) {
	db := setupScheduleTestDB(t)

	repo := NewScheduleRepository(db)

	plugID1 := "plug-list-1"
	plugID2 := "plug-list-2"

	require.NoError(t, testdb.InsertPlug(db, plugID1, "test-user", "Test 1", "ns-list-1", "t1"))
	require.NoError(t, testdb.InsertPlug(db, plugID2, "test-user", "Test 2", "ns-list-2", "t2"))

	require.NoError(t, testdb.InsertSchedule(db, &testdb.ScheduleOpts{
		ID:      "s1",
		PlugID:  plugID1,
		UserID:  "test-user",
		Time:    "05:00",
		ReadyBy: "09:00",
		Enabled: true,
	}))
	require.NoError(t, testdb.InsertSchedule(db, &testdb.ScheduleOpts{
		ID:     "s2",
		PlugID: plugID2,
		UserID: "test-user",
		Time:   "06:00",
	}))

	list, err := repo.ListAll(t.Context())
	require.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, plugID1, *list[0].PlugID)
	require.NotNil(t, list[0].ReadyBy)
	assert.Equal(t, "09:00", *list[0].ReadyBy)
	assert.Equal(t, plugID2, *list[1].PlugID)
	assert.Nil(t, list[1].ReadyBy)
}
