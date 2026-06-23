package tasmota

import "time"

// Tasmota-specific constants.
const (
	// TasmotaHttpTimeout is the HTTP client timeout for Tasmota requests.
	TasmotaHttpTimeout = 10 * time.Second

	// TasmotaStatusLevel is the Tasmota STATUS command level for sensor data.
	TasmotaStatusLevel = 10
)
