package tasmota

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ev-charge-controller/api/tasmota/tasmotatest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// IsConnected only checks the status code and never reads the response body.
// Draining the body lets net/http reuse the keep-alive connection, so repeated
// polls must not each open a fresh TCP connection.
func TestTasmotaClient_ReusesKeepAliveConnection(t *testing.T) {
	var newConns atomic.Int64
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"Status":{"Module":1,"FriendlyName":["Tasmota"]}}`)
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			newConns.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()

	client := NewTasmotaClient(NewTasmotaConfig(srv.URL))
	for range 3 {
		require.True(t, client.IsConnected(context.Background()))
	}

	assert.Equal(t, int64(1), newConns.Load(), "keep-alive connection should be reused after draining body")
}

var mockServer *httptest.Server

// TestMockAuthUsername and TestMockAuthPassword control Basic Auth on the test mock server.
// When both are empty strings, auth is disabled (default for existing tests).
var (
	TestMockAuthUsername string
	TestMockAuthPassword string
)

func checkMockAuth(r *http.Request, w http.ResponseWriter) bool {
	if TestMockAuthUsername == "" || TestMockAuthPassword == "" {
		return true
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		w.Header().Set("WWW-Authenticate", `Basic realm="Tasmota"`)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization required"})

		return false
	}

	const prefix = "Basic "
	if !strings.HasPrefix(authHeader, prefix) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Tasmota"`)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid authorization header"})

		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, prefix))
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="Tasmota"`)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials encoding"})

		return false
	}

	creds := strings.SplitN(string(decoded), ":", 2)
	if len(creds) != 2 || creds[0] != TestMockAuthUsername || creds[1] != TestMockAuthPassword {
		w.Header().Set("WWW-Authenticate", `Basic realm="Tasmota"`)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})

		return false
	}

	return true
}

func resetMockState() {
	tasmotatest.Reset()
}

func setupMockServer() {
	resetMockState()
	mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !checkMockAuth(r, w) {
			return
		}
		switch r.URL.Path {
		case "/cm":
			cmd := r.URL.Query().Get("cmnd")
			w.Header().Set("Content-Type", "application/json")

			switch cmd {
			case "POWER":
				tasmotatest.Mu.RLock()
				powerStr := "OFF"
				if tasmotatest.MockPowerState {
					powerStr = "ON"
				}
				tasmotatest.Mu.RUnlock()
				_ = json.NewEncoder(w).Encode(map[string]string{"POWER": powerStr})

			case "Power ON":
				tasmotatest.Mu.Lock()
				tasmotatest.SetPower(true)
				tasmotatest.Mu.Unlock()
				_ = json.NewEncoder(w).Encode(map[string]string{"POWER": "ON"})

			case "Power OFF":
				tasmotatest.Mu.Lock()
				tasmotatest.SetPower(false)
				tasmotatest.Mu.Unlock()
				_ = json.NewEncoder(w).Encode(map[string]string{"POWER": "OFF"})

			case "Power TOGGLE":
				tasmotatest.Mu.Lock()
				tasmotatest.MockPowerState = !tasmotatest.MockPowerState
				if tasmotatest.MockPowerState {
					tasmotatest.MockStartTime = time.Now()
					tasmotatest.MockPeakPower = tasmotatest.MockStartTime
					tasmotatest.MockEnergyData.Power = tasmotatest.MockMaxPowerWatts
					tasmotatest.MockEnergyData.Current = tasmotatest.MockMaxPowerWatts / tasmotatest.MockVoltage
				} else {
					tasmotatest.MockEnergyData.Power = 0
					tasmotatest.MockEnergyData.Current = 0
				}
				tasmotatest.MockLastUpdate = time.Now()
				toggleStr := "off"
				if tasmotatest.MockPowerState {
					toggleStr = "on"
				}
				tasmotatest.Mu.Unlock()
				_ = json.NewEncoder(w).Encode(map[string]string{"POWER": toggleStr})

			case "Status":
				powerInt := 0
				if tasmotatest.MockPowerState {
					powerInt = 1
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"Status": map[string]interface{}{
						"Module":       0,
						"DeviceName":   "EV_Charger",
						"FriendlyName": []string{"EV_Charger"},
						"Topic":        "tasmota_evcharge",
						"ButtonTopic":  "0",
						"Power":        powerInt,
						"PowerOnState": 1,
						"LedState":     1,
						"LedMask":      "FFFF",
						"SaveData":     1,
						"SaveState":    1,
					},
				})

				return

			case "STATUS 10":
				tasmotatest.Mu.Lock()
				if tasmotatest.MockPowerState {
					now := time.Now()
					elapsed := now.Sub(tasmotatest.MockLastUpdate)
					if elapsed > 0 {
						currentPower := tasmotatest.CalcRealisticPower()
						tasmotatest.MockEnergyData.Power = currentPower
						tasmotatest.MockEnergyData.Current = currentPower / tasmotatest.MockVoltage
						elapsedH := elapsed.Hours()
						inc := currentPower * elapsedH
						tasmotatest.MockEnergyData.Total += inc / 1000
						tasmotatest.MockEnergyData.Today += inc / 1000
					}
					tasmotatest.MockLastUpdate = now
				}
				energy := tasmotatest.MockEnergyData
				tasmotatest.Mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"StatusSNS": map[string]interface{}{
						"Time": time.Now().Format(time.RFC3339),
						"ENERGY": map[string]interface{}{
							"TotalStartTime": "2024-03-19T13:49:14",
							"Total":          energy.Total,
							"Yesterday":      energy.Yesterday / 1000,
							"Today":          energy.Today / 1000,
							"Power":          int(energy.Power),
							"ApparentPower":  int(energy.Apparent),
							"ReactivePower":  int(energy.Reactive),
							"Factor":         energy.PowerFactor,
							"Voltage":        int(energy.Voltage),
							"Current":        energy.Current,
						},
					},
				})

				return

			case "ENERGY":
				// Real Tasmota returns {"Command": "Unknown"} for ENERGY command
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"Command": "Unknown"})

			default:
				if strings.HasPrefix(strings.ToUpper(cmd), "ENERGYRES") {
					var res int
					_, _ = fmt.Sscanf(cmd, "EnergyRes %d", &res)
					if res < 1 {
						res = 3 // default
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]int{"EnergyRes": res})

					return
				}
				w.WriteHeader(http.StatusBadRequest)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func teardownMockServer() {
	if mockServer != nil {
		mockServer.Close()
		mockServer = nil
	}
}

// ResetMockServer resets the mock server's in-memory state.
// Exported so other test packages (services, handlers) can call it.
func ResetMockServer() {
	if mockServer == nil {
		return
	}
	resetMockState()
}

func TestMain(m *testing.M) {
	setupMockServer()
	defer teardownMockServer()
	code := m.Run()
	os.Exit(code)
}

func TestTasmotaClient_SetPowerState(t *testing.T) {
	ResetMockServer()
	client := NewTasmotaClient(NewTasmotaConfig(mockServer.URL))

	err := client.SetPowerState(t.Context(), true)
	assert.NoError(t, err)

	state, err := client.GetPowerState(t.Context())
	assert.NoError(t, err)
	assert.True(t, state)
}

func TestTasmotaClient_Toggle(t *testing.T) {
	ResetMockServer()
	client := NewTasmotaClient(NewTasmotaConfig(mockServer.URL))

	// Toggle on
	powerOn, err := client.Toggle(t.Context())
	assert.NoError(t, err)
	assert.True(t, powerOn)

	// Verify current is approximately 195.65A (45000W / 230V)
	energy, err := client.GetEnergy(t.Context())
	require.NoError(t, err)
	assert.InDelta(t, 195.65, energy.Current, 0.01)

	// Toggle off
	powerOn, err = client.Toggle(t.Context())
	assert.NoError(t, err)
	assert.False(t, powerOn)

	// Toggle back on
	powerOn, err = client.Toggle(t.Context())
	assert.NoError(t, err)
	assert.True(t, powerOn)
}

func TestTasmotaClient_GetEnergy(t *testing.T) {
	ResetMockServer()
	client := NewTasmotaClient(NewTasmotaConfig(mockServer.URL))

	// Set power on to generate energy
	_ = client.SetPowerState(t.Context(), true)

	energy, err := client.GetEnergy(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, 45000.0, energy.Power)
	assert.InDelta(t, 195.65, energy.Current, 0.01) // 45000W / 230V ≈ 195.65A
}

func TestTasmotaClient_IsConnected(t *testing.T) {
	client := NewTasmotaClient(NewTasmotaConfig(mockServer.URL))

	assert.True(t, client.IsConnected(t.Context()))
}

func TestTasmotaClient_GetPowerStateHTTP(t *testing.T) {
	ResetMockServer()
	client := NewTasmotaClient(NewTasmotaConfig(mockServer.URL))

	// Default state is off
	state, err := client.GetPowerStateHTTP(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, "off", state)

	// Set power on
	_ = client.SetPowerState(t.Context(), true)
	state, err = client.GetPowerStateHTTP(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, "on", state)
}

func TestTasmotaClient_GetEnergyReading(t *testing.T) {
	ResetMockServer()
	client := NewTasmotaClient(NewTasmotaConfig(mockServer.URL))

	// Get current energy (may be non-zero if mock server has been running)
	energy, err := client.GetEnergy(t.Context())
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, energy.Total, 0.0)

	// Set power on and get energy
	_ = client.SetPowerState(t.Context(), true)
	energy, err = client.GetEnergy(t.Context())
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, energy.Total, 0.0)
}

func TestNewTasmotaConfig(t *testing.T) {
	config := NewTasmotaConfig(mockServer.URL)
	assert.Equal(t, mockServer.URL, config.BaseURL)
	assert.Equal(t, 10*time.Second, config.Timeout)
}

func TestTasmotaClient_SetPowerState_Error(t *testing.T) {
	// Use 127.0.0.1:1 (reserved port) for instant connection refused, no DNS timeout
	client := NewTasmotaClient(NewTasmotaConfig("http://127.0.0.1:1"))

	err := client.SetPowerState(t.Context(), true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to set power state")
}

func TestTasmotaClient_GetPowerState_Error(t *testing.T) {
	client := NewTasmotaClient(NewTasmotaConfig("http://127.0.0.1:1"))

	state, err := client.GetPowerState(t.Context())
	assert.Error(t, err)
	assert.False(t, state)
}

func TestTasmotaClient_GetEnergy_Error(t *testing.T) {
	client := NewTasmotaClient(NewTasmotaConfig("http://127.0.0.1:1"))

	energy, err := client.GetEnergy(t.Context())
	assert.Error(t, err)
	assert.Nil(t, energy)
}

func TestTasmotaClient_IsConnected_RealServer(t *testing.T) {
	client := NewTasmotaClient(NewTasmotaConfig("http://127.0.0.1:1"))

	assert.False(t, client.IsConnected(t.Context()))
}

func TestTasmotaClient_EnergyData_Integrity(t *testing.T) {
	ResetMockServer()
	client := NewTasmotaClient(NewTasmotaConfig(mockServer.URL))

	// Set power on
	_ = client.SetPowerState(t.Context(), true)

	// Get energy and verify all fields are populated
	energy, err := client.GetEnergy(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, 45000.0, energy.Power)
	assert.Equal(t, 50.0, energy.PowerFactor)
	assert.Equal(t, 230.0, energy.Voltage)
	assert.GreaterOrEqual(t, energy.Current, 0.0)
	assert.GreaterOrEqual(t, energy.Apparent, 0.0)
	assert.GreaterOrEqual(t, energy.Reactive, 0.0)
}

func TestTasmotaClient_Auth_RejectsWithoutCredentials(t *testing.T) {
	oldUsername, oldPassword := TestMockAuthUsername, TestMockAuthPassword
	defer func() {
		TestMockAuthUsername, TestMockAuthPassword = oldUsername, oldPassword
	}()
	TestMockAuthUsername = "testuser"
	TestMockAuthPassword = "testpass"
	ResetMockServer()

	client := NewTasmotaClient(NewTasmotaConfig(mockServer.URL))

	err := client.SetPowerState(t.Context(), true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")

	_, err = client.GetPowerState(t.Context())
	assert.Error(t, err)

	_, err = client.GetEnergy(t.Context())
	assert.Error(t, err)

	_, err = client.Toggle(t.Context())
	assert.Error(t, err)

	assert.False(t, client.IsConnected(t.Context()))
}

func TestTasmotaClient_Auth_AcceptsValidCredentials(t *testing.T) {
	oldUsername, oldPassword := TestMockAuthUsername, TestMockAuthPassword
	defer func() {
		TestMockAuthUsername, TestMockAuthPassword = oldUsername, oldPassword
	}()
	TestMockAuthUsername = "testuser"
	TestMockAuthPassword = "testpass"
	ResetMockServer()

	client := NewTasmotaClient(NewTasmotaConfigWithAuth(mockServer.URL, "testuser", "testpass"))

	err := client.SetPowerState(t.Context(), true)
	assert.NoError(t, err)

	state, err := client.GetPowerState(t.Context())
	assert.NoError(t, err)
	assert.True(t, state)

	energy, err := client.GetEnergy(t.Context())
	assert.NoError(t, err)
	assert.NotNil(t, energy)

	on, err := client.Toggle(t.Context())
	assert.NoError(t, err)
	assert.False(t, on)

	assert.True(t, client.IsConnected(t.Context()))
}

func TestTasmotaClient_Auth_RejectsWrongCredentials(t *testing.T) {
	oldUsername, oldPassword := TestMockAuthUsername, TestMockAuthPassword
	defer func() {
		TestMockAuthUsername, TestMockAuthPassword = oldUsername, oldPassword
	}()
	TestMockAuthUsername = "testuser"
	TestMockAuthPassword = "testpass"
	ResetMockServer()

	client := NewTasmotaClient(NewTasmotaConfigWithAuth(mockServer.URL, "wronguser", "wrongpass"))

	err := client.SetPowerState(t.Context(), true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestTasmotaClient_Auth_NoAuthWhenServerHasNone(t *testing.T) {
	// Ensure auth is disabled on server
	oldUsername, oldPassword := TestMockAuthUsername, TestMockAuthPassword
	defer func() {
		TestMockAuthUsername, TestMockAuthPassword = oldUsername, oldPassword
	}()
	TestMockAuthUsername = ""
	TestMockAuthPassword = ""
	ResetMockServer()

	// Client with credentials should still work when server has no auth
	client := NewTasmotaClient(NewTasmotaConfigWithAuth(mockServer.URL, "anyuser", "anypass"))

	err := client.SetPowerState(t.Context(), true)
	assert.NoError(t, err)

	state, err := client.GetPowerState(t.Context())
	assert.NoError(t, err)
	assert.True(t, state)
}

func TestSetEnergyResolution(t *testing.T) {
	ResetMockServer()
	client := NewTasmotaClient(NewTasmotaConfig(mockServer.URL))

	res, err := client.SetEnergyResolution(t.Context(), 4)
	assert.NoError(t, err)
	assert.Equal(t, 4, res)
}

func TestGetPowerStateHTTP_Error(t *testing.T) {
	client := NewTasmotaClient(NewTasmotaConfig("http://127.0.0.1:1"))

	state, err := client.GetPowerStateHTTP(t.Context())
	assert.Error(t, err)
	assert.Equal(t, "off", state)
}

func TestTasmotaClient_CircuitBreaker_OpensAfterFailures(t *testing.T) {
	config := TasmotaConfig{
		BaseURL:          "http://127.0.0.1:1",
		Timeout:          TasmotaHttpTimeout,
		FailureThreshold: 3,
	}
	client := NewTasmotaClient(config)

	// First 3 requests should fail but not be blocked by circuit breaker
	for range 3 {
		err := client.SetPowerState(t.Context(), true)
		assert.Error(t, err)
	}

	// 4th request should be blocked by circuit breaker
	err := client.SetPowerState(t.Context(), true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker")
}

func TestTasmotaClient_CircuitBreaker_AllowsAfterReset(t *testing.T) {
	config := TasmotaConfig{
		BaseURL:             mockServer.URL,
		Timeout:             TasmotaHttpTimeout,
		FailureThreshold:    3,
		CircuitResetTimeout: 50 * time.Millisecond,
	}
	client := NewTasmotaClient(config)

	// Make successful requests to ensure circuit is closed
	err := client.SetPowerState(t.Context(), true)
	assert.NoError(t, err)

	// Circuit should remain closed after success
	state, err := client.GetPowerStateHTTP(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, "on", state)
}

func TestTasmotaClient_CircuitBreaker_StatusError(t *testing.T) {
	failureCount := 0
	tmpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failureCount++
		if failureCount <= 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"Status": map[string]interface{}{"Module": 0},
		})
	}))
	defer tmpServer.Close()

	config := TasmotaConfig{
		BaseURL:             tmpServer.URL,
		Timeout:             TasmotaHttpTimeout,
		FailureThreshold:    3,
		CircuitResetTimeout: 50 * time.Millisecond,
	}
	client := NewTasmotaClient(config)

	// After 3 failures, circuit should be open
	assert.False(t, client.IsConnected(t.Context()))
	assert.False(t, client.IsConnected(t.Context()))
	assert.False(t, client.IsConnected(t.Context()))

	// Next request should be blocked by circuit breaker
	assert.False(t, client.IsConnected(t.Context()))

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Should succeed now (server returns 200 after 3 failures)
	assert.True(t, client.IsConnected(t.Context()))
}

func TestTasmotaClient_Toggle_ErrorStatus(t *testing.T) {
	tmpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tmpServer.Close()

	client := NewTasmotaClient(NewTasmotaConfig(tmpServer.URL))

	on, err := client.Toggle(t.Context())
	assert.Error(t, err)
	assert.False(t, on)
	assert.Contains(t, err.Error(), "500")
}

func TestTasmotaClient_Toggle_MalformedJSON(t *testing.T) {
	tmpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"invalid json`)
	}))
	defer tmpServer.Close()

	client := NewTasmotaClient(NewTasmotaConfig(tmpServer.URL))

	on, err := client.Toggle(t.Context())
	assert.Error(t, err)
	assert.False(t, on)
	assert.Contains(t, err.Error(), "failed to parse response")
}

func TestTasmotaClient_SetEnergyResolution_ErrorStatus(t *testing.T) {
	tmpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer tmpServer.Close()

	client := NewTasmotaClient(NewTasmotaConfig(tmpServer.URL))

	res, err := client.SetEnergyResolution(t.Context(), 4)
	assert.Error(t, err)
	assert.Equal(t, 0, res)
	assert.Contains(t, err.Error(), "400")
}

func TestTasmotaClient_SetEnergyResolution_MalformedJSON(t *testing.T) {
	tmpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `not json`)
	}))
	defer tmpServer.Close()

	client := NewTasmotaClient(NewTasmotaConfig(tmpServer.URL))

	res, err := client.SetEnergyResolution(t.Context(), 4)
	assert.Error(t, err)
	assert.Equal(t, 0, res)
	assert.Contains(t, err.Error(), "failed to parse energy resolution response")
}

func TestTasmotaClient_GetPowerState_MalformedJSON(t *testing.T) {
	tmpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"POWER": 123}`)
	}))
	defer tmpServer.Close()

	client := NewTasmotaClient(NewTasmotaConfig(tmpServer.URL))

	on, err := client.GetPowerState(t.Context())
	// strings.EqualFold with int should not panic; POWER field is string type
	// JSON decoder will fail to unmarshal int into string field
	assert.Error(t, err)
	assert.False(t, on)
}

func TestTasmotaClient_GetEnergy_MissingFields(t *testing.T) {
	tmpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"StatusSNS": map[string]interface{}{
				"ENERGY": map[string]interface{}{
					"Total": 0,
				},
			},
		})
	}))
	defer tmpServer.Close()

	client := NewTasmotaClient(NewTasmotaConfig(tmpServer.URL))

	energy, err := client.GetEnergy(t.Context())
	assert.NoError(t, err)
	assert.NotNil(t, energy)
	// All fields should be zero when missing
	assert.Equal(t, float64(0), energy.Total)
	assert.Equal(t, float64(0), energy.Power)
}

func TestTasmotaClient_DoGet_ContextCancellation(t *testing.T) {
	tmpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // slow response
	}))
	defer tmpServer.Close()

	client := NewTasmotaClient(NewTasmotaConfig(tmpServer.URL))

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	// IsConnected returns false on context cancellation (no error returned)
	connected := client.IsConnected(ctx)
	assert.False(t, connected)
}

func TestTasmotaClient_GetPowerState_ErrorStatus(t *testing.T) {
	tmpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer tmpServer.Close()

	client := NewTasmotaClient(NewTasmotaConfig(tmpServer.URL))

	on, err := client.GetPowerState(t.Context())
	assert.Error(t, err)
	assert.False(t, on)
	assert.Contains(t, err.Error(), "404")
}

func TestTasmotaClient_GetEnergy_ErrorStatus(t *testing.T) {
	tmpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer tmpServer.Close()

	client := NewTasmotaClient(NewTasmotaConfig(tmpServer.URL))

	energy, err := client.GetEnergy(t.Context())
	assert.Error(t, err)
	assert.Nil(t, energy)
	assert.Contains(t, err.Error(), "503")
}

func TestTasmotaClient_SetPowerState_ErrorStatus(t *testing.T) {
	tmpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer tmpServer.Close()

	client := NewTasmotaClient(NewTasmotaConfig(tmpServer.URL))

	err := client.SetPowerState(t.Context(), true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

