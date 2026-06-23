package repository

import (
	"testing"

	"ev-charge-controller/api/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVehicleModelRepository_List(t *testing.T) {
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	defer db.Close()

	repo := NewVehicleModelRepository(db)
	models, err := repo.List(t.Context())
	require.NoError(t, err)
	assert.Len(t, models, 3)

	ids := make(map[string]bool)
	for _, m := range models {
		ids[m.ID] = true
		assert.NotEmpty(t, m.Name)
		assert.Greater(t, m.CapacityKwh, 0.0)
	}
	assert.True(t, ids["rm1"])
	assert.True(t, ids["rm1s"])
	assert.True(t, ids["rm2"])
}

func TestVehicleModelRepository_FindByID(t *testing.T) {
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	defer db.Close()

	repo := NewVehicleModelRepository(db)

	m, err := repo.FindByID(t.Context(), "rm1")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "rm1", m.ID)
	assert.Equal(t, "Maeving RM1", m.Name)
	assert.InDelta(t, 2.026, m.CapacityKwh, 0.001)
	assert.Equal(t, 600.0, m.ChargerOutputW)
	assert.Equal(t, 0.8, m.ChargingEfficiency)
	require.NotNil(t, m.PackVoltageMaxV)
	assert.InDelta(t, 58.8, *m.PackVoltageMaxV, 0.01)

	none, err := repo.FindByID(t.Context(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, none)
}
