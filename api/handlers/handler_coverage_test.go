package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ev-charge-controller/api/carbonintensity"
	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/middleware"
	"ev-charge-controller/api/repository"
	"ev-charge-controller/api/services"
	"ev-charge-controller/api/tasmota"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// PowerReadingsHandler - validation error paths
// ---------------------------------------------------------------------------

func TestPowerReadingsHandler_InvalidSessionID(t *testing.T) {
	db := setupHandlerTestDB(t)
	defer db.Close()
	repo := repository.NewChargeSessionRepository(db)
	handler := NewPowerReadingsHandler(services.NewChartDataService(repo))

	// Session ID longer than 36 chars (UUID max) should be rejected
	longID := strings.Repeat("a", 37)
	req, _ := http.NewRequest(http.MethodGet, "/api/power-readings?sessionId="+longID, nil)
	rr := httptest.NewRecorder()
	handler.GetReadings(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPowerReadingsHandler_InvalidVehicleID(t *testing.T) {
	db := setupHandlerTestDB(t)
	defer db.Close()
	repo := repository.NewChargeSessionRepository(db)
	handler := NewPowerReadingsHandler(services.NewChartDataService(repo))

	// Vehicle ID longer than 50 chars should be rejected
	longID := strings.Repeat("b", 51)
	req, _ := http.NewRequest(http.MethodGet, "/api/power-readings?vehicleId="+longID, nil)
	rr := httptest.NewRecorder()
	handler.GetReadings(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---------------------------------------------------------------------------
// ChargeSessionHandler - Stop relay failure path
// ---------------------------------------------------------------------------

// conditionalPlugController succeeds on the first N calls, then fails.
type conditionalPlugController struct {
	callCount int
	failAfter int
}

func (c *conditionalPlugController) SetPower(ctx context.Context, plugID string, on bool) error {
	c.callCount++
	if c.callCount > c.failAfter {
		return errors.New("relay control failed")
	}
	return nil
}

func (c *conditionalPlugController) SetPowerAndWait(ctx context.Context, plugID string, on bool, _ time.Duration) (bool, error) {
	if err := c.SetPower(ctx, plugID, on); err != nil {
		return false, err
	}
	return true, nil
}

func (c *conditionalPlugController) LastEnergy(plugID string) *tasmota.EnergyData {
	return nil
}

func TestChargeSessionHandler_Stop_RelayFailure(t *testing.T) {
	db := setupHandlerTestDB(t)
	defer db.Close()

	// Succeeds on first call (Start), fails on second (Stop)
	plugCtrl := &conditionalPlugController{failAfter: 1}
	chargeRepo := repository.NewChargeSessionRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	plugRepo := repository.NewPlugRepository(db)
	svc := services.NewChargeSessionService(
		context.Background(),
		chargeRepo,
		vehicleRepo,
		plugRepo,
		plugCtrl,
		nil,
		nil,
	)
	handler := NewChargeSessionHandler(svc)

	// Create and activate a session
	body, _ := json.Marshal(StartRequest{
		PlugID:        "test-plug",
		VehicleID:     "rm1",
		StartPercent:  20,
		TargetPercent: 80,
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Start(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, "Start should succeed: %s", rr.Body.String())

	// Transition to active
	_, err := db.Exec("UPDATE charge_sessions SET status = 'active' WHERE status = 'pending'")
	require.NoError(t, err)

	// Stop should return 503 because relay control fails
	req2, _ := http.NewRequest(http.MethodPatch, "/api/charge-sessions?vehicleId=rm1", nil)
	rr2 := httptest.NewRecorder()
	handler.Stop(rr2, req2)

	assert.Equal(t, http.StatusServiceUnavailable, rr2.Code)
}

// ---------------------------------------------------------------------------
// ChargeSessionHandler - UpdateTarget with status="stopped" delegates to Stop
// ---------------------------------------------------------------------------

func TestChargeSessionHandler_UpdateTarget_StatusStopped(t *testing.T) {
	db, handler := setupChargeSessionTestDB(t)

	// Create and activate a session
	body, _ := json.Marshal(StartRequest{VehicleID: "rm1", StartPercent: 20, TargetPercent: 80})
	req, _ := http.NewRequest(http.MethodPost, "/api/charge-sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Start(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Transition to active
	_, err := db.Exec("UPDATE charge_sessions SET status = 'active' WHERE status = 'pending'")
	require.NoError(t, err)

	// Send status="stopped" - should delegate to Stop and return 204
	stopBody, _ := json.Marshal(map[string]string{"status": "stopped"})
	req2, _ := http.NewRequest(http.MethodPatch, "/api/charge-sessions?vehicleId=rm1", bytes.NewReader(stopBody))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.UpdateTarget(rr2, req2)

	assert.Equal(t, http.StatusNoContent, rr2.Code)
}

func TestChargeSessionHandler_UpdateTarget_NegativeTargetPercent(t *testing.T) {
	_, handler := setupChargeSessionTestDB(t)

	body, _ := json.Marshal(map[string]any{"targetPercent": -5})
	req, _ := http.NewRequest(http.MethodPatch, "/api/charge-sessions/target", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.UpdateTarget(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---------------------------------------------------------------------------
// PlugHandler - service error paths (not ErrPlugNotFound)
// ---------------------------------------------------------------------------

func TestPlugHandler_GetByID_ServiceError(t *testing.T) {
	db := setupHandlerTestDB(t)

	_, err := db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash) VALUES (?, ?, ?)`,
		testPlugHandlerUserID, "plug-handler-test@example.com", "")
	require.NoError(t, err)

	plugRepo := repository.NewPlugRepository(db)
	svc := services.NewMqttProvisioningService(plugRepo, nil, &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"})
	handler := NewPlugHandler(svc)

	// Close DB to force a service error
	db.Close()

	req := plugHandlerReqWithUser(http.MethodGet, "/api/plugs/test-plug", nil, testPlugHandlerUserID)
	req.SetPathValue("id", "test-plug")
	rr := httptest.NewRecorder()
	handler.GetByID(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestPlugHandler_Update_ServiceError(t *testing.T) {
	db := setupHandlerTestDB(t)

	_, err := db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash) VALUES (?, ?, ?)`,
		testPlugHandlerUserID, "plug-handler-test@example.com", "")
	require.NoError(t, err)

	plugRepo := repository.NewPlugRepository(db)
	svc := services.NewMqttProvisioningService(plugRepo, nil, &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"})
	handler := NewPlugHandler(svc)

	// Close DB to force a service error
	db.Close()

	newName := "Updated"
	body, _ := json.Marshal(map[string]string{"name": newName})
	req := plugHandlerReqWithUser(http.MethodPatch, "/api/plugs/test-plug", body, testPlugHandlerUserID)
	req.SetPathValue("id", "test-plug")
	rr := httptest.NewRecorder()
	handler.Update(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestPlugHandler_Delete_ServiceError(t *testing.T) {
	db := setupHandlerTestDB(t)

	_, err := db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash) VALUES (?, ?, ?)`,
		testPlugHandlerUserID, "plug-handler-test@example.com", "")
	require.NoError(t, err)

	plugRepo := repository.NewPlugRepository(db)
	svc := services.NewMqttProvisioningService(plugRepo, nil, &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"})
	handler := NewPlugHandler(svc)

	// Close DB to force a service error
	db.Close()

	req := plugHandlerReqWithUser(http.MethodDelete, "/api/plugs/test-plug", nil, testPlugHandlerUserID)
	req.SetPathValue("id", "test-plug")
	rr := httptest.NewRecorder()
	handler.Delete(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestPlugHandler_ConfigureDevice_ServiceError(t *testing.T) {
	db := setupHandlerTestDB(t)

	_, err := db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash) VALUES (?, ?, ?)`,
		testPlugHandlerUserID, "plug-handler-test@example.com", "")
	require.NoError(t, err)

	plugRepo := repository.NewPlugRepository(db)
	svc := services.NewMqttProvisioningService(plugRepo, nil, &internal.Config{MQTTExternalIP: "test.local", MQTTExternalPort: "1883"})
	handler := NewPlugHandler(svc)

	// Close DB to force a service error
	db.Close()

	req := plugHandlerReqWithUser(http.MethodPost, "/api/plugs/test-plug/configure", []byte("{}"), testPlugHandlerUserID)
	req.SetPathValue("id", "test-plug")
	rr := httptest.NewRecorder()
	handler.ConfigureDevice(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

func TestValidateSessionID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"empty is ok", "", false},
		{"valid uuid", "550e8400-e29b-41d4-a716-446655440000", false},
		{"too long", strings.Repeat("a", 37), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSessionID(tt.id)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateVehicleID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"empty is ok", "", false},
		{"valid id", "rm1", false},
		{"too long", strings.Repeat("b", 51), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVehicleID(tt.id)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// logReq - request ID branch
// ---------------------------------------------------------------------------

func TestLogReq_WithRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Pass through the middleware to set the request ID on the context
	var capturedCtx context.Context
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.RequestIDMiddleware(next)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	attrs := logReq(req.WithContext(capturedCtx))
	assert.Contains(t, attrs, "req_id")
}

func TestLogReq_WithoutRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	attrs := logReq(req)
	assert.Contains(t, attrs, "path")
	assert.Contains(t, attrs, "/test")
	assert.NotContains(t, attrs, "req_id")
}

// ---------------------------------------------------------------------------
// HistoryHandler - error paths
// ---------------------------------------------------------------------------

func TestHistoryHandler_Get_InvalidLimit(t *testing.T) {
	db := setupHandlerTestDB(t)
	defer db.Close()

	chargeRepo := repository.NewChargeSessionRepository(db)
	historySvc := services.NewHistoryService(chargeRepo, chargeRepo)
	handler := NewHistoryHandler(historySvc)

	req, _ := http.NewRequest(http.MethodGet, "/api/history?limit=0", nil)
	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHistoryHandler_Get_InvalidLimitOver1000(t *testing.T) {
	db := setupHandlerTestDB(t)
	defer db.Close()

	chargeRepo := repository.NewChargeSessionRepository(db)
	historySvc := services.NewHistoryService(chargeRepo, chargeRepo)
	handler := NewHistoryHandler(historySvc)

	req, _ := http.NewRequest(http.MethodGet, "/api/history?limit=1001", nil)
	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHistoryHandler_Get_InvalidOffset(t *testing.T) {
	db := setupHandlerTestDB(t)
	defer db.Close()

	chargeRepo := repository.NewChargeSessionRepository(db)
	historySvc := services.NewHistoryService(chargeRepo, chargeRepo)
	handler := NewHistoryHandler(historySvc)

	req, _ := http.NewRequest(http.MethodGet, "/api/history?offset=-1", nil)
	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHistoryHandler_Get_DBError(t *testing.T) {
	db := setupHandlerTestDB(t)

	chargeRepo := repository.NewChargeSessionRepository(db)
	historySvc := services.NewHistoryService(chargeRepo, chargeRepo)
	handler := NewHistoryHandler(historySvc)

	// Close DB to force error
	db.Close()

	// Must provide date or limit parameter (handler validation)
	req, _ := http.NewRequest(http.MethodGet, "/api/history?limit=10", nil)
	rr := httptest.NewRecorder()
	handler.Get(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ---------------------------------------------------------------------------
// VehicleStatsHandler - GetAllStats error path
// ---------------------------------------------------------------------------

func TestVehicleStatsHandler_GetAllStats_DBError(t *testing.T) {
	db := setupHandlerTestDB(t)

	vehicleRepo := repository.NewVehicleRepository(db)
	chargeRepo := repository.NewChargeSessionRepository(db)
	statsSvc := services.NewVehicleStatsServiceWithRepos(vehicleRepo, chargeRepo)
	handler := NewVehicleStatsHandler(statsSvc)

	// Close DB to force error
	db.Close()

	req, _ := http.NewRequest(http.MethodGet, "/api/vehicles/stats", nil)
	rr := httptest.NewRecorder()
	handler.GetAllStats(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ---------------------------------------------------------------------------
// SchemaHandler - error paths
// ---------------------------------------------------------------------------
// AuthHandler - additional error paths
// ---------------------------------------------------------------------------

func TestAuthHandler_Register_DBError(t *testing.T) {
	db := setupHandlerTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	authSvc := services.NewAuthService(userRepo, tokenRepo, "test-secret-key-for-jwt")
	handler := NewAuthHandler(authSvc)

	// Drop users table to force error
	_, err := db.Exec("DROP TABLE users")
	require.NoError(t, err)

	body, _ := json.Marshal(map[string]string{
		"email":    "new@test.com",
		"password": "strongpassword123",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Register(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestAuthHandler_Login_DBError(t *testing.T) {
	db := setupHandlerTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	authSvc := services.NewAuthService(userRepo, tokenRepo, "test-secret-key-for-jwt")
	handler := NewAuthHandler(authSvc)

	// Drop users table to force error
	_, err := db.Exec("DROP TABLE users")
	require.NoError(t, err)

	body, _ := json.Marshal(map[string]string{
		"email":    "test@test.com",
		"password": "password",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.Login(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestAuthHandler_Refresh_DBError(t *testing.T) {
	db := setupHandlerTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	authSvc := services.NewAuthService(userRepo, tokenRepo, "test-secret-key-for-jwt")
	handler := NewAuthHandler(authSvc)

	// Drop refresh_tokens table to force error
	_, err := db.Exec("DROP TABLE refresh_tokens")
	require.NoError(t, err)

	// Provide a valid JSON body so extractRefreshToken doesn't panic
	body, _ := json.Marshal(map[string]string{"refreshToken": "fake-token"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handler.Refresh(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestAuthHandler_Logout_DBError(t *testing.T) {
	db := setupHandlerTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	authSvc := services.NewAuthService(userRepo, tokenRepo, "test-secret-key-for-jwt")
	handler := NewAuthHandler(authSvc)

	// Drop refresh_tokens table to force error
	_, err := db.Exec("DROP TABLE refresh_tokens")
	require.NoError(t, err)

	// Provide a valid JSON body so extractRefreshToken doesn't panic
	body, _ := json.Marshal(map[string]string{"refreshToken": "fake-token"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handler.Logout(rr, req)

	// Logout still returns 204 because it ignores errors from token deletion
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

// ---------------------------------------------------------------------------
// CarbonIntensityHandler - network error path (unreachable server)
// ---------------------------------------------------------------------------

func TestCarbonIntensityHandler_GetCurrent_NetworkError(t *testing.T) {
	// Use a client pointing to a port that won't respond
	client := carbonintensity.NewClient()
	client.SetBaseURL("http://localhost:59999")
	handler := NewCarbonIntensityHandler(client)

	req, _ := http.NewRequest(http.MethodGet, "/api/carbon-intensity", nil)
	rr := httptest.NewRecorder()
	handler.GetCurrent(rr, req)

	// Should return 503 when the network request fails
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

// ---------------------------------------------------------------------------
// Middleware - RequestID helper used in logReq tests
// ---------------------------------------------------------------------------

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	var capturedID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = middleware.GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.RequestIDMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.NotEmpty(t, capturedID)
	assert.Contains(t, capturedID, "req-")
	assert.Equal(t, capturedID, rr.Header().Get(middleware.RequestIDHeader))
}

func TestRequestIDMiddleware_EchoesValidID(t *testing.T) {
	var capturedID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = middleware.GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.RequestIDMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(middleware.RequestIDHeader, "my-custom-id")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "my-custom-id", capturedID)
	assert.Equal(t, "my-custom-id", rr.Header().Get(middleware.RequestIDHeader))
}

func TestRequestIDMiddleware_RejectsInvalidID(t *testing.T) {
	var capturedID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = middleware.GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.RequestIDMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(middleware.RequestIDHeader, "<script>alert(1)</script>")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.NotEqual(t, "<script>alert(1)</script>", capturedID)
	assert.Contains(t, capturedID, "req-")
}
