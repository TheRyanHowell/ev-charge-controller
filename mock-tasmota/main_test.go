package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestHandler creates an isolated handler for each test.
func newTestHandler() *TasmotaHandler {
	h := &TasmotaHandler{
		energyData: EnergyData{
			Total: 1.092, LastTotal: 0,
			Yesterday: 0, Today: 0, Period: 0,
			Power: 0, Apparent: 0, Reactive: 0,
			Freq: 50.0, Voltage: 230.0, Current: 0,
		},
		maxPowerWatts: 1510.0,
		voltage:       230.0,
		frequency:     50.0,
		energyRes:     4,
	}
	return h
}

// testServer creates an httptest.Server from a handler.
func testServer(h *TasmotaHandler) *httptest.Server {
	return httptest.NewServer(h)
}

// getBody performs a GET request and returns the response body.
func getBody(t *testing.T, client *http.Client, url string) []byte {
	t.Helper()
	resp, err := client.Get(url)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	t.Cleanup(func() { resp.Body.Close() })
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return body
}

// --- Power Control ---

func TestPowerQuery_Off(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=POWER")
	var resp PowerResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "OFF", resp.POWER)
}

func TestPowerQuery_On(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	h.SetPower(true)
	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=POWER")
	var resp PowerResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "ON", resp.POWER)
}

func TestPowerOn(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Power%20ON")
	var resp PowerResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "ON", resp.POWER)

	// Verify state persisted
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=POWER")
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "ON", resp.POWER)
}

func TestPowerOff(t *testing.T) {
	h := newTestHandler()
	h.SetPower(true)
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Power%20OFF")
	var resp PowerResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "OFF", resp.POWER)

	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=POWER")
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "OFF", resp.POWER)
}

func TestPowerToggle(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	// Toggle from off -> on
	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Power%20TOGGLE")
	var resp PowerResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "ON", resp.POWER)

	// Toggle from on -> off
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Power%20TOGGLE")
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "OFF", resp.POWER)
}

func TestPowerState_Persistence(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	h.SetPower(true)

	// Multiple queries should all show ON
	for i := 0; i < 5; i++ {
		body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=POWER")
		var resp PowerResponse
		require.NoError(t, json.Unmarshal(body, &resp))
		assert.Equal(t, "ON", resp.POWER, "iteration %d", i)
	}
}

// --- Energy / STATUS 10 ---

func TestStatus10_OffState(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))

	energy := result["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})
	assert.Equal(t, 0, int(energy["Power"].(float64)))
}

func TestStatus10_PowerFactor(t *testing.T) {
	h := newTestHandler()
	h.SetPower(true)
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))

	energy := result["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})
	factor := energy["Factor"].(float64)
	assert.Greater(t, factor, 0.0)
	assert.Less(t, factor, 1.0)
}

func TestStatus10_ApparentPower(t *testing.T) {
	h := newTestHandler()
	h.SetPower(true)
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))

	energy := result["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})
	power := energy["Power"].(float64)
	apparent := energy["ApparentPower"].(float64)
	assert.GreaterOrEqual(t, apparent, power)
}

func TestStatus10_ReactivePower(t *testing.T) {
	h := newTestHandler()
	h.SetPower(true)
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))

	energy := result["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})
	power := energy["Power"].(float64)
	apparent := energy["ApparentPower"].(float64)
	reactive := energy["ReactivePower"].(float64)

	// Reactive = sqrt(Apparent^2 - Power^2)
	expectedReactive := math.Sqrt(apparent*apparent - power*power)
	assert.InDelta(t, expectedReactive, reactive, 1.0)
}

func TestStatus10_EnergyAccumulation(t *testing.T) {
	h := newTestHandler()
	h.SetPower(true)
	server := testServer(h)
	defer server.Close()

	// First reading
	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result1 map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result1))
	energy1 := result1["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})
	total1 := energy1["Total"].(float64)

	// Wait for accumulation
	time.Sleep(100 * time.Millisecond)

	// Second reading
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result2 map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result2))
	energy2 := result2["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})
	total2 := energy2["Total"].(float64)

	assert.Greater(t, total2, total1, "energy should increase when power is on")
}

// --- Auth ---

func TestAuth_NoAuthRequired(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=POWER")
	var resp PowerResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "OFF", resp.POWER)
}

func TestAuth_RequiresCredentials(t *testing.T) {
	h := newTestHandler()
	h.authUsername = "admin"
	h.authPassword = "secret"
	server := testServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/cm?cmnd=POWER")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_InvalidCredentials(t *testing.T) {
	h := newTestHandler()
	h.authUsername = "admin"
	h.authPassword = "secret"
	server := testServer(h)
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL+"/cm?cmnd=POWER", nil)
	require.NoError(t, err)
	req.SetBasicAuth("wrong", "creds")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_ValidCredentials(t *testing.T) {
	h := newTestHandler()
	h.authUsername = "admin"
	h.authPassword = "secret"
	server := testServer(h)
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL+"/cm?cmnd=POWER", nil)
	require.NoError(t, err)
	req.SetBasicAuth("admin", "secret")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// --- EnergyRes ---

func TestEnergyRes(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=EnergyRes%204")
	var resp map[string]int
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, 4, resp["EnergyRes"])
}

// --- Unknown Commands ---

func TestUnknownCommand(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=FooBar")
	var resp CommandResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "Unknown", resp.Command)
}

func TestEnergyCommand(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=ENERGY")
	var resp CommandResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "Unknown", resp.Command)
}

// --- Handler as httptest.Server ---

func TestHandler_HttptestServer(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	// Verify ServeHTTP works through httptest.Server
	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=POWER")
	var resp PowerResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "OFF", resp.POWER)

	// Verify /cm routing works
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var statusResp map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &statusResp))
	assert.Contains(t, statusResp, "Status")
}

func TestHandler_Reset(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	// Set power on
	h.SetPower(true)
	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=POWER")
	var resp PowerResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "ON", resp.POWER)

	// Reset
	h.Reset()

	// Should be off again
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=POWER")
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "OFF", resp.POWER)
}

// --- Health ---

func TestHealth(t *testing.T) {
	h := newTestHandler()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		h.ServeHTTP(w, r)
	}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "ok", string(body))
}

// --- CC/CV Curve ---

func TestCalcRealisticPower_Fixed(t *testing.T) {
	h := newTestHandler()
	h.SetPower(true)

	for i := 0; i < 10; i++ {
		time.Sleep(10 * time.Millisecond)
		power := h.calculateRealisticPower()
		assert.Equal(t, h.maxPowerWatts, power,
			"power should be fixed at maxPowerWatts %.0f at iteration %d",
			h.maxPowerWatts, i)
	}
}

// --- Status ---

func TestStatus_Response(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	require.NotNil(t, resp.Status)
	assert.Equal(t, "LB_12", resp.Status["DeviceName"])
	assert.InDelta(t, 0.0, resp.Status["Power"], 0.01) // power is off
}

func TestStatus_PowerOn(t *testing.T) {
	h := newTestHandler()
	h.SetPower(true)
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.InDelta(t, 1.0, resp.Status["Power"], 0.01)
}

// --- Root ---

func TestRoot_NoMParam(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "LB_12")
}

// --- NotFound ---

func TestNotFound(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/nonexistent")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// --- Status with SENSOR command ---

func TestSensorCommand(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=SENSOR")
	var resp CommandResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "Unknown", resp.Command)
}

// --- Power OFF after accumulation ---

func TestPowerOff_ResetsEnergy(t *testing.T) {
	h := newTestHandler()
	h.SetPower(true)
	server := testServer(h)
	defer server.Close()

	// Get energy while on
	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result1 map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result1))
	energy1 := result1["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})
	power1 := energy1["Power"].(float64)
	assert.Greater(t, power1, 0.0)

	// Turn off
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Power%20OFF")
	var resp PowerResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "OFF", resp.POWER)

	// Check energy after off
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result2 map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result2))
	energy2 := result2["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})
	power2 := energy2["Power"].(float64)
	assert.Equal(t, 0.0, power2)
}

// --- Status 10 Time format ---

func TestStatus10_TimeFormat(t *testing.T) {
	h := newTestHandler()
	h.SetPower(true)
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))

	timeStr := result["StatusSNS"].(map[string]interface{})["Time"].(string)
	_, err := time.Parse(time.RFC3339, timeStr)
	require.NoError(t, err, "Time should be valid RFC3339: %s", timeStr)
}

// --- Status 10 TotalStartTime ---

func TestStatus10_TotalStartTime(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))

	totalStartTime := result["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})["TotalStartTime"].(string)
	assert.Contains(t, totalStartTime, "T") // valid datetime format
}

// --- Status 10 Voltage ---

func TestStatus10_Voltage(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))

	voltage := result["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})["Voltage"].(float64)
	assert.Equal(t, 230.0, voltage)
}

// --- Status 10 Current calculation ---

func TestStatus10_Current(t *testing.T) {
	h := newTestHandler()
	h.SetPower(true)
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))

	energy := result["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})
	power := energy["Power"].(float64)
	current := energy["Current"].(float64)
	voltage := energy["Voltage"].(float64)

	// Current = Power / Voltage (approximately, due to power factor)
	expectedCurrent := power / voltage
	assert.InDelta(t, expectedCurrent, current, 0.1)
}

// --- Multiple toggles ---

func TestPowerToggle_Multiple(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	expected := []string{"ON", "OFF", "ON"}
	for _, exp := range expected {
		body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Power%20TOGGLE")
		var resp PowerResponse
		require.NoError(t, json.Unmarshal(body, &resp))
		assert.Equal(t, exp, resp.POWER)
	}
}

// --- Status 10 with no power state changes when off ---

func TestStatus10_NoAccumulation_WhenOff(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	// First reading while off
	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result1 map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result1))
	total1 := result1["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})["Total"].(float64)

	// Wait
	time.Sleep(100 * time.Millisecond)

	// Second reading while still off
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result2 map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result2))
	total2 := result2["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})["Total"].(float64)

	assert.Equal(t, total1, total2, "energy should not change when power is off")
}

// --- EnergyRes sets value ---

func TestEnergyRes_Set(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	// Set to 3
	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=EnergyRes%203")
	var resp map[string]int
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, 3, resp["EnergyRes"])

	// Verify it persisted
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=EnergyRes%205")
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, 5, resp["EnergyRes"])
}

// --- Status with SwitchMode array ---

func TestStatus_SwitchModeArray(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	switchMode, ok := resp.Status["SwitchMode"].([]interface{})
	require.True(t, ok, "SwitchMode should be an array")
	assert.Len(t, switchMode, 30, "SwitchMode should have 30 elements")
}

// --- Status with all expected fields ---

func TestStatus_AllFields(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	expectedFields := []string{
		"Module", "DeviceName", "FriendlyName", "Topic", "ButtonTopic",
		"Power", "PowerOnState", "LedState", "LedMask", "SaveData",
		"SaveState", "SwitchTopic", "SwitchMode", "ButtonRetain",
		"SwitchRetain", "SensorRetain", "PowerRetain", "InfoRetain",
		"StateRetain", "StatusRetain",
	}
	for _, field := range expectedFields {
		assert.Contains(t, resp.Status, field, "Status should contain field %s", field)
	}
}

// --- EnergyData integrity ---

func TestStatus10_EnergyDataIntegrity(t *testing.T) {
	h := newTestHandler()
	h.SetPower(true)
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))

	energy := result["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})

	// All numeric fields should be present and non-negative (when power is on)
	assert.GreaterOrEqual(t, energy["Total"].(float64), 0.0)
	assert.GreaterOrEqual(t, energy["Today"].(float64), 0.0)
	assert.GreaterOrEqual(t, energy["Yesterday"].(float64), 0.0)
	assert.GreaterOrEqual(t, energy["Power"].(float64), 0.0)
	assert.GreaterOrEqual(t, energy["Voltage"].(float64), 0.0)
	assert.GreaterOrEqual(t, energy["Current"].(float64), 0.0)
}

// --- getPowerState helper ---

func TestGetPowerState(t *testing.T) {
	h := newTestHandler()
	assert.Equal(t, "OFF", h.getPowerState())

	h.SetPower(true)
	assert.Equal(t, "ON", h.getPowerState())

	h.SetPower(false)
	assert.Equal(t, "OFF", h.getPowerState())
}

// --- SetPower state transitions ---

func TestSetPower_Transitions(t *testing.T) {
	h := newTestHandler()
	assert.False(t, h.powerState)

	h.SetPower(true)
	assert.True(t, h.powerState)
	assert.True(t, !h.startTime.IsZero())

	h.SetPower(false)
	assert.False(t, h.powerState)
}

// --- EnergyRes edge case: value < 1 defaults to 3 ---

func TestEnergyRes_BelowMinimum(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=EnergyRes%200")
	var resp map[string]int
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, 3, resp["EnergyRes"])
}

// --- cmnd with empty value returns Unknown ---

func TestEmptyCommand(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm")
	var resp CommandResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "Unknown", resp.Command)
}

// --- cmHandler locks and unlocks correctly (no deadlock) ---

func TestCMHandler_NoDeadlock(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	done := make(chan bool, 1)
	go func() {
		for i := 0; i < 10; i++ {
			getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
		}
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("handler deadlocked")
	}
}

// --- EnergyData Today and Total are independent ---

func TestEnergyTodayVsTotal(t *testing.T) {
	h := newTestHandler()
	h.SetPower(true)
	server := testServer(h)
	defer server.Close()

	// Initial reading
	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result1 map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result1))
	energy1 := result1["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})
	total1 := energy1["Total"].(float64)
	today1 := energy1["Today"].(float64)

	// Turn off and back on (simulates a new day)
	h.SetPower(false)
	time.Sleep(50 * time.Millisecond)
	h.SetPower(true)
	time.Sleep(100 * time.Millisecond)

	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result2 map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result2))
	energy2 := result2["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})
	total2 := energy2["Total"].(float64)
	today2 := energy2["Today"].(float64)

	// Total should have accumulated
	assert.Greater(t, total2, total1)
	// Today should also have accumulated from the second power-on
	assert.GreaterOrEqual(t, today2, today1)
}

// --- Auth with bad base64 encoding ---

func TestAuth_BadBase64(t *testing.T) {
	h := newTestHandler()
	h.authUsername = "admin"
	h.authPassword = "secret"
	server := testServer(h)
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL+"/cm?cmnd=POWER", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Basic not-valid-base64!!!")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// --- Auth with no Basic prefix ---

func TestAuth_NoBasicPrefix(t *testing.T) {
	h := newTestHandler()
	h.authUsername = "admin"
	h.authPassword = "secret"
	server := testServer(h)
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL+"/cm?cmnd=POWER", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "admin:secret")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// --- Status 10 with custom voltage ---

func TestStatus10_CustomVoltage(t *testing.T) {
	h := newTestHandler()
	h.voltage = 120.0
	h.energyData.Voltage = 120.0
	h.SetPower(true)
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))

	voltage := result["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})["Voltage"].(float64)
	assert.Equal(t, 120.0, voltage)
}

// --- Power ON sets startTime ---

func TestPowerOn_SetsStartTime(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	before := time.Now()
	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Power%20ON")
	after := time.Now()

	var resp PowerResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "ON", resp.POWER)

	// Verify startTime is within the window
	h.mu.RLock()
	start := h.startTime
	h.mu.RUnlock()
	assert.True(t, !start.IsZero())
	assert.True(t, start.After(before.Add(-time.Second)))
	assert.True(t, start.Before(after.Add(time.Second)))
}

// --- Multiple STATUS 10 queries while power on ---

func TestStatus10_MultipleQueries(t *testing.T) {
	h := newTestHandler()
	h.SetPower(true)
	server := testServer(h)
	defer server.Close()

	var totals []float64
	for i := 0; i < 5; i++ {
		body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &result))
		total := result["StatusSNS"].(map[string]interface{})["ENERGY"].(map[string]interface{})["Total"].(float64)
		totals = append(totals, total)
		time.Sleep(50 * time.Millisecond)
	}

	// Each query should show equal or more energy than the previous
	for i := 1; i < len(totals); i++ {
		assert.GreaterOrEqual(t, totals[i], totals[i-1],
			"total should not decrease: %.6f -> %.6f", totals[i-1], totals[i])
	}
}

// --- Test that the defaultHandler package variable works ---

func TestDefaultHandler_MainBehavior(t *testing.T) {
	// The defaultHandler should have initial state (power off)
	assert.False(t, defaultHandler.powerState)
	assert.Equal(t, 0.0, defaultHandler.maxPowerWatts)
	assert.Equal(t, 230.0, defaultHandler.voltage)
	assert.Equal(t, 50.0, defaultHandler.frequency)
}

// --- Server returns correct Content-Type ---

func TestContentTypes(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	tests := []struct {
		path    string
		content string
	}{
		{"/cm?cmnd=POWER", "application/json"},
		{"/cm?cmnd=Power%20ON", "application/json"},
		{"/cm?cmnd=Status", "application/json"},
		{"/cm?cmnd=STATUS%2010", "application/json"},
		{"/", "text/html"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resp, err := http.Get(server.URL + tt.path)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, tt.content, resp.Header.Get("Content-Type"))
		})
	}
}

// --- Test that /cm path only matches /cm exactly ---

func TestCMPathMatching(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	// /cm should work
	resp, err := http.Get(server.URL + "/cm?cmnd=POWER")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// /cm/extra should not work (404)
	resp, err = http.Get(server.URL + "/cm/extra")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// --- Test EnergyRes with various values ---

func TestEnergyRes_VariousValues(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	for _, res := range []int{1, 2, 3, 4, 5, 6} {
		path := fmt.Sprintf("/cm?cmnd=EnergyRes%%20%d", res)
		body := getBody(t, &http.Client{}, server.URL+path)
		var resp map[string]int
		require.NoError(t, json.Unmarshal(body, &resp))
		assert.Equal(t, res, resp["EnergyRes"], "EnergyRes %d should be confirmed", res)
	}
}

// --- Test that power state is correctly reflected in Status ---

func TestStatus_PowerStateMatches(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	// Initially off
	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var statusResp StatusResponse
	require.NoError(t, json.Unmarshal(body, &statusResp))
	assert.InDelta(t, 0.0, statusResp.Status["Power"], 0.01)

	// Turn on
	getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Power%20ON")
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	require.NoError(t, json.Unmarshal(body, &statusResp))
	assert.InDelta(t, 1.0, statusResp.Status["Power"], 0.01)

	// Turn off
	getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Power%20OFF")
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	require.NoError(t, json.Unmarshal(body, &statusResp))
	assert.InDelta(t, 0.0, statusResp.Status["Power"], 0.01)
}

// --- Test that TOGGLE preserves power state ---

func TestToggle_PreservesState(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	// Toggle off -> on
	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Power%20TOGGLE")
	var resp PowerResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "ON", resp.POWER)

	// Verify POWER query matches
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=POWER")
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "ON", resp.POWER)

	// Toggle on -> off
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Power%20TOGGLE")
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "OFF", resp.POWER)

	// Verify POWER query matches
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=POWER")
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "OFF", resp.POWER)
}

// --- Test EnergyRes with negative value ---

func TestEnergyRes_NegativeValue(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=EnergyRes%20-1")
	var resp map[string]int
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, 3, resp["EnergyRes"]) // defaults to 3
}

// --- Test that Status returns correct FriendlyName ---

func TestStatus_FriendlyName(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	friendlyName, ok := resp.Status["FriendlyName"].([]interface{})
	require.True(t, ok)
	require.Len(t, friendlyName, 1)
	assert.Equal(t, "LB_12", friendlyName[0])
}

// --- Test that Status returns correct Topic ---

func TestStatus_Topic(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.Equal(t, "tasmota_evcharge", resp.Status["Topic"])
}

// --- Test that Status returns correct LedMask ---

func TestStatus_LedMask(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.Equal(t, "FFFF", resp.Status["LedMask"])
}

// --- Test that Status returns correct SaveData ---

func TestStatus_SaveData(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.InDelta(t, 1.0, resp.Status["SaveData"], 0.01)
}

// --- Test that Status returns correct SaveState ---

func TestStatus_SaveState(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.InDelta(t, 1.0, resp.Status["SaveState"], 0.01)
}

// --- Test that Status returns correct PowerOnState ---

func TestStatus_PowerOnState(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.InDelta(t, 1.0, resp.Status["PowerOnState"], 0.01)
}

// --- Test that Status returns correct LedState ---

func TestStatus_LedState(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.InDelta(t, 1.0, resp.Status["LedState"], 0.01)
}

// --- Test that Status returns correct ButtonTopic ---

func TestStatus_ButtonTopic(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.Equal(t, "0", resp.Status["ButtonTopic"])
}

// --- Test that Status returns correct Module ---

func TestStatus_Module(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.InDelta(t, 0.0, resp.Status["Module"], 0.01)
}

// --- Test that Status returns correct SwitchTopic ---

func TestStatus_SwitchTopic(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.Equal(t, "0", resp.Status["SwitchTopic"])
}

// --- Test that Status returns correct ButtonRetain ---

func TestStatus_ButtonRetain(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.InDelta(t, 0.0, resp.Status["ButtonRetain"], 0.01)
}

// --- Test that Status returns correct SwitchRetain ---

func TestStatus_SwitchRetain(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.InDelta(t, 0.0, resp.Status["SwitchRetain"], 0.01)
}

// --- Test that Status returns correct SensorRetain ---

func TestStatus_SensorRetain(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.InDelta(t, 0.0, resp.Status["SensorRetain"], 0.01)
}

// --- Test that Status returns correct PowerRetain ---

func TestStatus_PowerRetain(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.InDelta(t, 0.0, resp.Status["PowerRetain"], 0.01)
}

// --- Test that Status returns correct InfoRetain ---

func TestStatus_InfoRetain(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.InDelta(t, 0.0, resp.Status["InfoRetain"], 0.01)
}

// --- Test that Status returns correct StateRetain ---

func TestStatus_StateRetain(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.InDelta(t, 0.0, resp.Status["StateRetain"], 0.01)
}

// --- Test that Status returns correct StatusRetain ---

func TestStatus_StatusRetain(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=Status")
	var resp StatusResponse
	require.NoError(t, json.Unmarshal(body, &resp))

	assert.InDelta(t, 0.0, resp.Status["StatusRetain"], 0.01)
}

// --- EnergyTotal (set the cumulative meter, mirrors real Tasmota) ---

func TestEnergyTotal_Set(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=EnergyTotal%205.5")
	var resp map[string]float64
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.InDelta(t, 5.5, resp["EnergyTotal"], 1e-9)

	// The new total must be reflected in Status 10.
	body = getBody(t, &http.Client{}, server.URL+"/cm?cmnd=STATUS%2010")
	var status struct {
		StatusSNS struct {
			ENERGY struct {
				Total float64 `json:"Total"`
			} `json:"ENERGY"`
		} `json:"StatusSNS"`
	}
	require.NoError(t, json.Unmarshal(body, &status))
	assert.InDelta(t, 5.5, status.StatusSNS.ENERGY.Total, 1e-9)
}

func TestEnergyTotal_Invalid(t *testing.T) {
	h := newTestHandler()
	server := testServer(h)
	defer server.Close()

	before := h.energyData.Total
	body := getBody(t, &http.Client{}, server.URL+"/cm?cmnd=EnergyTotal%20abc")
	var resp CommandResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "Unknown", resp.Command)
	assert.InDelta(t, before, h.energyData.Total, 1e-9)
}
