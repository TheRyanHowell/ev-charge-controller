package tasmota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// drainAndClose reads any unread response body to EOF and closes it. net/http
// can only reuse a keep-alive connection if the body is fully consumed; without
// this, every poll (every few seconds) opens a fresh TCP connection to the device.
func drainAndClose(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// EnergyData represents the energy readings from Tasmota.
type EnergyData struct {
	Total       float64 `json:"total"` // in kWh, from Tasmota STATUS 10
	LastTotal   float64 `json:"lastTotal"`
	Yesterday   float64 `json:"yesterday"`
	Today       float64 `json:"today"`
	Period      int     `json:"period"`
	Power       float64 `json:"power"`
	Apparent    float64 `json:"apparent"`
	Reactive    float64 `json:"reactive"`
	PowerFactor float64 `json:"freq"`
	Voltage     float64 `json:"voltage"`
	Current     float64 `json:"current"`
}

// PowerResponse represents the power state response from Tasmota.
type PowerResponse struct {
	POWER string `json:"POWER"`
}

// TasmotaClient is a Tasmota client for controlling power and reading energy data.
type TasmotaClient struct {
	baseURL        string
	httpClient     *http.Client
	username       string
	password       string
	circuitBreaker *CircuitBreaker
	mu             sync.RWMutex
	energyData     *EnergyData
}

// TasmotaConfig holds configuration for Tasmota integration.
type TasmotaConfig struct {
	BaseURL            string
	Timeout            time.Duration
	Username           string
	Password           string
	FailureThreshold   int
	CircuitResetTimeout time.Duration
}

// NewTasmotaConfig creates a default configuration.
func NewTasmotaConfig(baseURL string) TasmotaConfig {
	return TasmotaConfig{
		BaseURL: baseURL,
		Timeout: TasmotaHttpTimeout,
	}
}

// NewTasmotaConfigWithAuth creates a configuration with Basic Auth credentials.
func NewTasmotaConfigWithAuth(baseURL, username, password string) TasmotaConfig {
	return TasmotaConfig{
		BaseURL:  baseURL,
		Timeout:  TasmotaHttpTimeout,
		Username: username,
		Password: password,
	}
}

// NewTasmotaClient creates a new Tasmota client.
func NewTasmotaClient(config TasmotaConfig) *TasmotaClient {
	return &TasmotaClient{
		baseURL:        config.BaseURL,
		httpClient:     &http.Client{Timeout: config.Timeout},
		username:       config.Username,
		password:       config.Password,
		circuitBreaker: NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: config.FailureThreshold,
			ResetTimeout:     config.CircuitResetTimeout,
		}),
		energyData: &EnergyData{},
	}
}

// doGet performs an authenticated GET request to the Tasmota device.
// Checks circuit breaker before sending and records success/failure after.
func (c *TasmotaClient) doGet(ctx context.Context, url string) (*http.Response, error) {
	// ShouldBlock is the single-flight gate: it rejects while open and admits
	// exactly one probe per reset window when half-open.
	if c.circuitBreaker.ShouldBlock() {
		return nil, fmt.Errorf("tasmota request blocked by circuit breaker: %w", ErrCircuitOpen)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.circuitBreaker.Failure()
		return nil, err
	}
	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.circuitBreaker.Failure()
		return nil, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		c.circuitBreaker.Failure()
		return resp, nil
	}

	c.circuitBreaker.Success()
	return resp, nil
}

// SetPowerState sets the power state on the Tasmota device.
func (c *TasmotaClient) SetPowerState(ctx context.Context, powerOn bool) error {
	var reqURL string
	if powerOn {
		reqURL = fmt.Sprintf("%s/cm?cmnd=Power%%20ON", c.baseURL)
	} else {
		reqURL = fmt.Sprintf("%s/cm?cmnd=Power%%20OFF", c.baseURL)
	}
	resp, err := c.doGet(ctx, reqURL)
	if err != nil {
		return fmt.Errorf("failed to set power state: %w", err)
	}
	defer drainAndClose(resp)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Tasmota returned status %d", resp.StatusCode)
	}

	return nil
}

// GetPowerState returns the current power state.
func (c *TasmotaClient) GetPowerState(ctx context.Context) (bool, error) {
	reqURL := fmt.Sprintf("%s/cm?cmnd=POWER", c.baseURL)
	resp, err := c.doGet(ctx, reqURL)
	if err != nil {
		return false, fmt.Errorf("failed to get power state: %w", err)
	}
	defer drainAndClose(resp)

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("Tasmota returned status %d", resp.StatusCode)
	}

	var powerResp PowerResponse
	if err := json.NewDecoder(resp.Body).Decode(&powerResp); err != nil {
		return false, fmt.Errorf("failed to parse response: %w", err)
	}

	return strings.EqualFold(powerResp.POWER, "on"), nil
}

// GetEnergy returns the current energy readings from the Tasmota device.
// Uses /cm?cmnd=STATUS%2010 which returns sensor data including energy readings.
func (c *TasmotaClient) GetEnergy(ctx context.Context) (*EnergyData, error) {
	reqURL := fmt.Sprintf("%s/cm?cmnd=STATUS%%20%d", c.baseURL, TasmotaStatusLevel)
	resp, err := c.doGet(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get energy: %w", err)
	}
	defer drainAndClose(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tasmota returned status %d", resp.StatusCode)
	}

	var statusSNS struct {
		StatusSNS struct {
			ENERGY struct {
				Total          float64 `json:"Total"`
				Yesterday      float64 `json:"Yesterday"`
				Today          float64 `json:"Today"`
				Power          float64 `json:"Power"`
				ApparentPower  float64 `json:"ApparentPower"`
				ReactivePower  float64 `json:"ReactivePower"`
				Factor         float64 `json:"Factor"`
				Voltage        float64 `json:"Voltage"`
				Current        float64 `json:"Current"`
				TotalStartTime string  `json:"TotalStartTime"`
			} `json:"ENERGY"`
		} `json:"StatusSNS"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&statusSNS); err != nil {
		return nil, fmt.Errorf("failed to parse energy response: %w", err)
	}

	energy := &EnergyData{
		Total:     statusSNS.StatusSNS.ENERGY.Total,
		Yesterday: statusSNS.StatusSNS.ENERGY.Yesterday,
		Today:     statusSNS.StatusSNS.ENERGY.Today,
		Power:     statusSNS.StatusSNS.ENERGY.Power,
		Apparent:  statusSNS.StatusSNS.ENERGY.ApparentPower,
		Reactive:  statusSNS.StatusSNS.ENERGY.ReactivePower,
		PowerFactor: statusSNS.StatusSNS.ENERGY.Factor,
		Voltage:   statusSNS.StatusSNS.ENERGY.Voltage,
		Current:   statusSNS.StatusSNS.ENERGY.Current,
	}

	c.mu.Lock()
	c.energyData = energy
	c.mu.Unlock()

	return energy, nil
}

// Toggle toggles the power state.
func (c *TasmotaClient) Toggle(ctx context.Context) (bool, error) {
	reqURL := fmt.Sprintf("%s/cm?cmnd=Power%%20TOGGLE", c.baseURL)
	resp, err := c.doGet(ctx, reqURL)
	if err != nil {
		return false, fmt.Errorf("failed to toggle power: %w", err)
	}
	defer drainAndClose(resp)

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("Tasmota returned status %d", resp.StatusCode)
	}

	var powerResp PowerResponse
	if err := json.NewDecoder(resp.Body).Decode(&powerResp); err != nil {
		return false, fmt.Errorf("failed to parse response: %w", err)
	}

	return strings.EqualFold(powerResp.POWER, "on"), nil
}

// SetEnergyResolution sets the Tasmota energy counter decimal-place precision.
// Value 1-6; higher values give finer resolution (e.g., 4 = 0.0001 kWh = 0.1Wh).
// Returns the confirmed resolution or an error.
func (c *TasmotaClient) SetEnergyResolution(ctx context.Context, res int) (int, error) {
	reqURL := fmt.Sprintf("%s/cm?cmnd=EnergyRes%%20%d", c.baseURL, res)
	resp, err := c.doGet(ctx, reqURL)
	if err != nil {
		return 0, fmt.Errorf("failed to set energy resolution: %w", err)
	}
	defer drainAndClose(resp)

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Tasmota returned status %d", resp.StatusCode)
	}

	var result struct {
		EnergyRes int `json:"EnergyRes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to parse energy resolution response: %w", err)
	}

	return result.EnergyRes, nil
}

// IsConnected checks if the Tasmota device is reachable via /cm?cmnd=Status.
func (c *TasmotaClient) IsConnected(ctx context.Context) bool {
	reqURL := fmt.Sprintf("%s/cm?cmnd=Status", c.baseURL)
	resp, err := c.doGet(ctx, reqURL)
	if err != nil {
		return false
	}
	defer drainAndClose(resp)

	return resp.StatusCode == http.StatusOK
}


// GetPowerStateHTTP queries the Tasmota device and returns the power state as a string.
func (c *TasmotaClient) GetPowerStateHTTP(ctx context.Context) (string, error) {
	on, err := c.GetPowerState(ctx)
	if err != nil {
		return "off", err
	}
	if on {
		return "on", nil
	}

	return "off", nil
}
