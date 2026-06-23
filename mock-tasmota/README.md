# mock-tasmota

Simulated Tasmota smart plug for local development and E2E testing. Implements
the Tasmota HTTP endpoints and MQTT behaviour used by the EV Charge Controller
Go API, including a realistic CC/CV charge curve and full MQTT telemetry
publishing.

Used automatically when running `make dev`. See the main
[README](../README.md) for getting started.

## Implemented Endpoints

| Command | Endpoint | Purpose |
|---------|----------|---------|
| POWER | `/cm?cmnd=POWER` | Read power state |
| Power ON | `/cm?cmnd=Power%20ON` | Turn relay on |
| Power OFF | `/cm?cmnd=Power%20OFF` | Turn relay off |
| Power TOGGLE | `/cm?cmnd=Power%20TOGGLE` | Toggle relay state |
| Status | `/cm?cmnd=Status` | Device connectivity check |
| STATUS 10 | `/cm?cmnd=STATUS%2010` | Energy/sensor readings |
| EnergyRes | `/cm?cmnd=EnergyRes%20N` | Set energy resolution |
| MQTTHost | `/cm?cmnd=MQTTHost%20<host>` | Set MQTT broker host |
| MQTTPort | `/cm?cmnd=MQTTPort%20<port>` | Set MQTT broker port |
| MQTTUser | `/cm?cmnd=MQTTUser%20<user>` | Set MQTT username |
| MQTTPassword | `/cm?cmnd=MQTTPassword%20<pass>` | Set MQTT password |
| FullTopic | `/cm?cmnd=FullTopic%20<pattern>` | Set MQTT topic pattern (parses namespace) |
| Topic | `/cm?cmnd=Topic%20<slug>` | Set MQTT topic slug |
| Restart 1 | `/cm?cmnd=Restart%201` | Trigger MQTT reconnect with current config |
| Unknown | any other `cmnd=` | Returns `{"Command":"Unknown"}` with HTTP 200 |
| Health | `/health` | Liveness probe |
| Reset | `/reset` | Reset power and energy state (preserves MQTT config) |
| Status | `/status` | JSON snapshot of current power and energy state |

## MQTT Behaviour

Once configured via the MQTT commands above, the mock connects to the broker and:

- Publishes energy sensor readings (`SENSOR`) on a ticker
- Publishes LWT `Online`/`Offline` messages
- Subscribes to relay commands (`cmnd/.../POWER`) and updates power state
- Publishes relay state responses (`stat/.../POWER`)

MQTT config is persisted to a state file and survives container restarts. The
`/reset` endpoint resets only power and energy state; MQTT config and the active
connection are preserved so the broker subscription stays live.

## CC/CV Charging Curve

When power is ON, the mock simulates a realistic Constant Current / Constant
Voltage charging curve:

- **0-15% progress**: full power (100% of max)
- **15-80% progress**: linearly ramps from 100% to 60%
- **80-100% progress**: linearly ramps from 60% to 20%
- **Sinusoidal noise**: +/-3% variation
- **Floor**: 20% of max power
- **Cap**: 100% of max power

Energy accumulates based on the simulated power over time, giving realistic
cumulative readings for testing the charge session service.

## Configuration

All configuration is via environment variables:

| Env Var | Default | Description |
|---------|---------|-------------|
| PORT | 8081 | HTTP listen port |
| POWER_WATTS | (required) | Base/peak power in watts |
| VOLTAGE | 230 | Voltage |
| FREQUENCY | 50 | Frequency in Hz |
| USERNAME | (empty) | Basic Auth username (empty = no auth) |
| PASSWORD | (empty) | Basic Auth password |

## Commands

All commands run inside Docker via `make` from the repo root.

```bash
make test-mock          # Run all mock-tasmota tests
make test-race-mock     # Run tests under the race detector
make cover-mock         # Tests with coverage report
make lint-mock          # golangci-lint
make vet-mock           # go vet
make fix-mock           # Auto-fix lint issues (golangci-lint --fix)
make deadcode-mock      # Detect unreachable code
```

## Usage in Go Tests

The handler can be embedded in test servers directly:

```go
package mytest

import (
    "net/http/httptest"
    "testing"
    "mock-tasmota"
)

func TestMyFeature(t *testing.T) {
    handler := &TasmotaHandler{
        energyData:     EnergyData{Total: 0, TotalWh: 1092},
        basePowerWatts: 39312.0,
        maxPowerWatts:  39312.0,
        voltage:        230.0,
        frequency:      50.0,
        energyRes:      4,
    }
    server := httptest.NewServer(handler)
    defer server.Close()

    // server.URL is the mock Tasmota URL (e.g. http://127.0.0.1:54321)
    client := NewTasmotaClient(server.URL)
    // ... test code
}
```

### Reset state between tests

```go
handler.Reset() // resets power and energy; MQTT config is preserved
```

### Setting power state programmatically

```go
handler.SetPower(true)  // Turn on
handler.SetPower(false) // Turn off
```
