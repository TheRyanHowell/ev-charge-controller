package services

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"ev-charge-controller/api/database"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSeedTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.SetupTestDB(true)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestSeedService(db *sql.DB) *SeedService {
	// No tasmota URLs - mock-tasmota is not available in unit tests.
	return NewSeedService(db, nil)
}

// TestSeedService_Reset_PreservesRefreshTokens asserts that calling Reset()
// after a refresh token has been issued does NOT delete that token. This is
// critical for E2E test isolation: the stateful suite calls /api/reset between
// every test while keeping browser auth cookies alive, so the refresh-token
// lookup that occurs on the next SSR page load must still succeed.
func TestSeedService_Reset_PreservesRefreshTokens(t *testing.T) {
	db := newSeedTestDB(t)
	svc := newTestSeedService(db)

	// First reset - creates the seed user.
	require.NoError(t, svc.Reset())

	// Insert a refresh token for the seed user.
	tokenRepo := repository.NewRefreshTokenRepository(db)
	tok := &models.RefreshToken{
		UserID:    seedUserID,
		TokenHash: "test-token-hash",
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}
	require.NoError(t, tokenRepo.Create(context.Background(), tok))
	tokenID := tok.ID
	require.NotEmpty(t, tokenID)

	// Second reset - must NOT delete the user or the refresh token.
	require.NoError(t, svc.Reset())

	// User row must still exist with the same ID.
	var userID string
	err := db.QueryRow("SELECT id FROM users WHERE email = ?", seedEmail).Scan(&userID)
	require.NoError(t, err, "seed user must survive Reset()")
	assert.Equal(t, seedUserID, userID)

	// Refresh token must still be retrievable.
	found, err := tokenRepo.FindByHash(context.Background(), "test-token-hash")
	require.NoError(t, err)
	require.NotNil(t, found, "refresh token must survive Reset()")
	assert.Equal(t, tokenID, found.ID)
	assert.Equal(t, seedUserID, found.UserID)
	assert.Nil(t, found.RevokedAt, "token must not be revoked")
}

// TestSeedService_Reset_IdempotentUser asserts that repeated Reset() calls
// never fail with a UNIQUE constraint on the users table, even when the user
// already exists from a prior reset.
func TestSeedService_Reset_IdempotentUser(t *testing.T) {
	db := newSeedTestDB(t)
	svc := newTestSeedService(db)

	for i := range 3 {
		err := svc.Reset()
		require.NoError(t, err, "Reset() #%d failed", i+1)
	}

	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", seedEmail).Scan(&count))
	assert.Equal(t, 1, count, "exactly one seed user row after multiple resets")
}

// TestSeedService_Reset_CostDataPresent asserts that seeded completed sessions have
// cost_pence and off_peak_kwh populated, and that a tariff is seeded for the user.
func TestSeedService_Reset_CostDataPresent(t *testing.T) {
	db := newSeedTestDB(t)
	svc := newTestSeedService(db)

	require.NoError(t, svc.Reset())

	// Tariff must be seeded.
	var baseRate float64
	err := db.QueryRow("SELECT base_rate_pence FROM tariff_settings WHERE user_id = ?", seedUserID).Scan(&baseRate)
	require.NoError(t, err, "tariff_settings must have a row for the seed user")
	assert.InDelta(t, seedTariffBaseRatePence, baseRate, 0.001)

	var windowCount int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM tariff_off_peak_windows WHERE user_id = ?", seedUserID).Scan(&windowCount))
	assert.Equal(t, 1, windowCount, "one off-peak window must be seeded")

	// All completed sessions with wall_kwh must have cost_pence set.
	var missingCost int
	require.NoError(t, db.QueryRow(`
		SELECT COUNT(*) FROM charge_sessions
		WHERE status = 'completed' AND wall_kwh IS NOT NULL AND cost_pence IS NULL`,
	).Scan(&missingCost))
	assert.Equal(t, 0, missingCost, "no completed session should have NULL cost_pence when wall_kwh is set")

	// off_peak_kwh must also be set for sessions with cost_pence.
	var missingOffPeak int
	require.NoError(t, db.QueryRow(`
		SELECT COUNT(*) FROM charge_sessions
		WHERE status = 'completed' AND cost_pence IS NOT NULL AND off_peak_kwh IS NULL`,
	).Scan(&missingOffPeak))
	assert.Equal(t, 0, missingOffPeak, "no completed session should have NULL off_peak_kwh when cost_pence is set")

	// Vehicle total_cost_pence must be positive (seeded sessions have cost data).
	var zeroCostVehicles int
	require.NoError(t, db.QueryRow(`
		SELECT COUNT(*) FROM vehicles WHERE total_cost_pence = 0 AND total_sessions > 0`,
	).Scan(&zeroCostVehicles))
	assert.Equal(t, 0, zeroCostVehicles, "vehicles with sessions must have non-zero total_cost_pence")
}

// TestSeedService_Reset_TodaySessionsPresent asserts that generateSessions always
// emits at least one completed session dated today for each vehicle with a plug,
// so the history page's default "today" filter is never empty after a reset.
func TestSeedService_Reset_TodaySessionsPresent(t *testing.T) {
	db := newSeedTestDB(t)
	svc := newTestSeedService(db)

	require.NoError(t, svc.Reset())

	today := time.Now().UTC().Format("2006-01-02")
	rows, err := db.Query(`
		SELECT COUNT(*) FROM charge_sessions
		WHERE DATE(created_at) = ? AND status = 'completed'`, today)
	require.NoError(t, err)
	defer rows.Close()

	var count int
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(&count))
	assert.GreaterOrEqual(t, count, 2, "at least 2 completed sessions must be seeded for today")
}

// TestSeedService_Reset_ProvisionsMockTasmotaMQTTOnlyOnce asserts that
// repeated Reset() calls only push MQTT config (/cm) to mock-tasmota once,
// while still resetting each instance's power/energy state (/reset) every
// time. Namespace and topic are deterministic and never change across
// resets, so redoing the dynsec+config-push+reconnect cycle before every
// e2e test bought nothing but forced every plug's MQTT connection to drop
// and re-establish - which raced the always-on carbon-aware schedule
// fixture and caused intermittent power-confirmation timeouts in CI
// (see resetMockTasmota's doc comment).
func TestSeedService_Reset_ProvisionsMockTasmotaMQTTOnlyOnce(t *testing.T) {
	origTimeout := plugOnlineTimeout
	plugOnlineTimeout = 100 * time.Millisecond // no real broker in this test, so it always hits the deadline
	t.Cleanup(func() { plugOnlineTimeout = origTimeout })

	var mu sync.Mutex
	var configPushes, energyResets int
	tasmota := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		switch {
		case r.URL.Path == "/cm":
			configPushes++
		case r.URL.Path == "/reset":
			energyResets++
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(tasmota.Close)

	db := newSeedTestDB(t)
	svc := NewSeedService(db, []string{tasmota.URL, tasmota.URL, tasmota.URL})

	require.NoError(t, svc.Reset())
	mu.Lock()
	firstConfigPushes, firstEnergyResets := configPushes, energyResets
	mu.Unlock()
	assert.Positive(t, firstConfigPushes, "first Reset() must push MQTT config to mock-tasmota")
	assert.Equal(t, 3, firstEnergyResets, "first Reset() must reset energy state for all 3 seeded plugs")

	require.NoError(t, svc.Reset())
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, firstConfigPushes, configPushes, "second Reset() must not push MQTT config again")
	assert.Equal(t, firstEnergyResets*2, energyResets, "second Reset() must still reset energy state for all 3 plugs")
}
