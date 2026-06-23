// Package tasmotatest provides shared mock state for Tasmota integration tests.
// Both the tasmota package tests and other test packages (services, handlers)
// import this to share the same in-memory mock state.
package tasmotatest

import (
	"math"
	"sync"
	"time"
)

var (
	MockPowerState bool
	MockEnergyData MockEnergy
	MockLastUpdate time.Time
	MockStartTime  time.Time
	MockPeakPower  time.Time
	Mu             sync.RWMutex
)

var MockMaxPowerWatts = 45000.0

const (
	MockBasePowerWatts = 39312.0
	MockVoltage        = 230.0
	MockFrequency      = 50.0
	// MockDeviceMac is the deterministic MAC returned by STATUS 5 in tests.
	MockDeviceMac      = "AA:BB:CC:DD:EE:FF"
	// MockDeviceName is the deterministic device name returned by STATUS 0 in tests.
	MockDeviceName     = "EV_Charger"
)

type MockEnergy struct {
	Total     float64 // in kWh
	LastTotal float64
	Yesterday float64
	Today     float64
	Period    int
	Power     float64
	Apparent  float64
	Reactive    float64
	PowerFactor float64
	Voltage     float64
	Current   float64
}

func init() {
	Reset()
}

func Reset() {
	Mu.Lock()
	defer Mu.Unlock()
	MockPowerState = false
	MockEnergyData = MockEnergy{
		Total:     1.092,
		LastTotal: 0,
		Yesterday: 0,
		Today:     0,
		Period:    0,
		Power:     0,
		Apparent:  0,
		Reactive:  0,
		PowerFactor: MockFrequency,
		Voltage:   MockVoltage,
		Current:   0,
	}
	MockLastUpdate = time.Now()
	MockStartTime = time.Time{}
	MockPeakPower = time.Time{}
}

func CalcRealisticPower() float64 {
	elapsed := time.Since(MockStartTime)
	elapsedSec := elapsed.Seconds()
	totalChargeSec := (MockBasePowerWatts / 1000) * 8.0 * 60.0
	progress := math.Min(elapsedSec/totalChargeSec, 1.0)

	var factor float64
	if progress < 0.15 {
		factor = 1.0
	} else if progress < 0.80 {
		factor = 1.0 - (progress-0.15)/0.65*0.4
	} else {
		factor = 0.6 - (progress-0.80)/0.20*0.4
	}

	seed := int64(elapsedSec * 100)
	noise := math.Sin(float64(seed)*0.1) * 0.03
	power := MockMaxPowerWatts * factor * (1 + noise)
	if power < MockMaxPowerWatts*0.2 {
		power = MockMaxPowerWatts * 0.2
	}
	if power > MockMaxPowerWatts {
		power = MockMaxPowerWatts
	}

	return power
}

// SetPower updates the mock power state.
// The caller is responsible for managing Mu synchronization.
func SetPower(on bool) {
	MockPowerState = on
	if on {
		MockStartTime = time.Now()
		MockPeakPower = MockStartTime
		MockEnergyData.Power = MockMaxPowerWatts
		MockEnergyData.Current = MockMaxPowerWatts / MockVoltage
	} else {
		MockEnergyData.Power = 0
		MockEnergyData.Current = 0
	}
	MockLastUpdate = time.Now()
}

// SetEnergy calls fn with a pointer to the current energy data.
// The caller is responsible for managing Mu synchronization.
func SetEnergy(fn func(*MockEnergy)) {
	fn(&MockEnergyData)
}
