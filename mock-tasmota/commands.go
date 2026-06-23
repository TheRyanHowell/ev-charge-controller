package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

func (h *TasmotaHandler) handleCM(w http.ResponseWriter, r *http.Request) {
	cmd := r.URL.Query().Get("cmnd")
	w.Header().Set("Content-Type", "application/json")

	if h.handleMQTTConfigCmd(w, cmd) {
		return
	}

	switch {
	case cmd == "POWER":
		h.mu.RLock()
		defer h.mu.RUnlock()
		_ = json.NewEncoder(w).Encode(PowerResponse{POWER: h.getPowerState()})

	case cmd == "Power ON":
		h.mu.Lock()
		h.powerState = true
		h.startTime = time.Now()
		h.energyData.Power = h.maxPowerWatts
		h.energyData.Current = h.maxPowerWatts / h.voltage
		h.lastUpdate = time.Now()
		h.mu.Unlock()
		h.publishPowerState("ON")
		_ = json.NewEncoder(w).Encode(PowerResponse{POWER: "ON"})

	case cmd == "Power OFF":
		h.mu.Lock()
		h.powerState = false
		h.energyData.Power = 0
		h.energyData.Current = 0
		h.lastUpdate = time.Now()
		h.mu.Unlock()
		h.publishPowerState("OFF")
		_ = json.NewEncoder(w).Encode(PowerResponse{POWER: "OFF"})

	case cmd == "Power TOGGLE":
		h.mu.Lock()
		h.powerState = !h.powerState
		if h.powerState {
			h.startTime = time.Now()
			h.energyData.Power = h.maxPowerWatts
			h.energyData.Current = h.maxPowerWatts / h.voltage
		} else {
			h.energyData.Power = 0
			h.energyData.Current = 0
		}
		state := h.getPowerState()
		h.lastUpdate = time.Now()
		h.mu.Unlock()
		h.publishPowerState(state)
		_ = json.NewEncoder(w).Encode(PowerResponse{POWER: state})

	case cmd == "Status":
		h.mu.RLock()
		powerInt := 0
		if h.powerState {
			powerInt = 1
		}
		h.mu.RUnlock()
		switchMode := make([]int, 30)
		_ = json.NewEncoder(w).Encode(StatusResponse{
			Status: map[string]interface{}{
				"Module": 0, "DeviceName": "LB_12",
				"FriendlyName": []string{"LB_12"}, "Topic": "tasmota_evcharge",
				"ButtonTopic": "0", "Power": powerInt,
				"PowerOnState": 1, "LedState": 1, "LedMask": "FFFF",
				"SaveData": 1, "SaveState": 1, "SwitchTopic": "0",
				"SwitchMode":   switchMode,
				"ButtonRetain": 0, "SwitchRetain": 0, "SensorRetain": 0,
				"PowerRetain":  0, "InfoRetain": 0, "StateRetain": 0,
				"StatusRetain": 0,
			},
		})

	case cmd == "STATUS 0":
		h.mu.RLock()
		powerInt := 0
		if h.powerState {
			powerInt = 1
		}
		h.mu.RUnlock()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"Status": map[string]interface{}{
				"Module":       0,
				"DeviceName":   "Mock Tasmota",
				"FriendlyName": []string{"Mock Tasmota"},
				"Topic":        "tasmota_mock",
				"ButtonTopic":  "0",
				"Power":        powerInt,
				"PowerOnState": 1,
				"LedState":     1,
				"LedMask":      "FFFF",
				"SaveData":     1,
				"SaveState":    1,
			},
		})

	case cmd == "STATUS 5":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"StatusNET": map[string]interface{}{
				"Hostname":   "tasmota-mock",
				"IPAddress":  "127.0.0.1",
				"Gateway":    "192.168.1.1",
				"Subnetmask": "255.255.255.0",
				"DNSServer1": "192.168.1.1",
				"DNSServer2": "0.0.0.0",
				"Mac":        "AA:BB:CC:DD:EE:FF",
				"IP6Global":  "",
				"IP6Local":   "",
			},
		})

	case cmd == "STATUS 10":
		h.mu.Lock()
		if h.powerState {
			curr := h.calculateRealisticPower()
			h.energyData.Power = curr
			h.energyData.Current = curr / h.voltage
			elapsed := time.Since(h.lastUpdate).Hours()
			kwh := (curr * elapsed) / 1000
			h.energyData.Total += kwh
			h.energyData.Today += kwh
			h.lastUpdate = time.Now()
		}
		energy := h.energyData
		h.mu.Unlock()
		factor := 0.70
		apparent := math.Round(energy.Power / factor)
		reactive := math.Sqrt(apparent*apparent - energy.Power*energy.Power)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"StatusSNS": map[string]interface{}{
				"Time": time.Now().Format(time.RFC3339),
				"ENERGY": map[string]interface{}{
					"TotalStartTime": "2024-03-19T13:49:14",
					"Total": energy.Total, "Yesterday": energy.Yesterday / 1000,
					"Today": energy.Today, "Power": int(energy.Power),
					"ApparentPower": int(apparent), "ReactivePower": int(reactive),
					"Factor": factor, "Voltage": int(energy.Voltage), "Current": energy.Current,
				},
			},
		})

	case cmd == "Restart 1":
		_ = json.NewEncoder(w).Encode(map[string]string{"Restart": "1"})
		go h.reconnectMQTT()

	case strings.HasPrefix(strings.ToUpper(cmd), "ENERGYRES"):
		var res int
		_, _ = fmt.Sscanf(cmd, "EnergyRes %d", &res)
		if res < 1 {
			res = 3
		}
		h.mu.Lock()
		h.energyRes = res
		h.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]int{"EnergyRes": res})

	case strings.ToUpper(cmd) == "POWERRETAIN 1" || strings.ToUpper(cmd) == "POWERRETAIN ON":
		_ = json.NewEncoder(w).Encode(map[string]int{"PowerRetain": 1})

	case strings.ToUpper(cmd) == "POWERRETAIN 0" || strings.ToUpper(cmd) == "POWERRETAIN OFF":
		_ = json.NewEncoder(w).Encode(map[string]int{"PowerRetain": 0})

	case strings.ToUpper(cmd) == "POWERRETAIN":
		_ = json.NewEncoder(w).Encode(map[string]int{"PowerRetain": 1})

	case strings.ToUpper(cmd) == "SENSORRETAIN 1" || strings.ToUpper(cmd) == "SENSORRETAIN ON":
		_ = json.NewEncoder(w).Encode(map[string]int{"SensorRetain": 1})

	case strings.ToUpper(cmd) == "SENSORRETAIN 0" || strings.ToUpper(cmd) == "SENSORRETAIN OFF":
		_ = json.NewEncoder(w).Encode(map[string]int{"SensorRetain": 0})

	case strings.ToUpper(cmd) == "SENSORRETAIN":
		_ = json.NewEncoder(w).Encode(map[string]int{"SensorRetain": 1})

	case strings.ToUpper(cmd) == "TELEPERIOD 10":
		_ = json.NewEncoder(w).Encode(map[string]int{"TelePeriod": 10})

	case strings.ToUpper(cmd) == "TELEPERIOD":
		_ = json.NewEncoder(w).Encode(map[string]int{"TelePeriod": 10})

	case strings.ToUpper(cmd) == "SETOPTION3 1":
		_ = json.NewEncoder(w).Encode(map[string]string{"SetOption3": "ON"})

	default:
		_ = json.NewEncoder(w).Encode(CommandResponse{Command: "Unknown"})
	}
}
