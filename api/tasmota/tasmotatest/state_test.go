package tasmotatest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReset(t *testing.T) {
	Reset() // Reset() acquires Mu internally

	Mu.Lock()
	assert.False(t, MockPowerState)
	assert.Equal(t, 0.0, MockEnergyData.Power)
	assert.Equal(t, 0.0, MockEnergyData.Current)
	assert.Equal(t, 1.092, MockEnergyData.Total)
	assert.Equal(t, 230.0, MockEnergyData.Voltage)
	assert.Equal(t, 50.0, MockEnergyData.PowerFactor)
	assert.Equal(t, MockFrequency, MockEnergyData.PowerFactor)
	assert.Equal(t, MockVoltage, MockEnergyData.Voltage)
	Mu.Unlock()
}

func TestSetPower_On(t *testing.T) {
	Mu.Lock()
	SetPower(true)
	Mu.Unlock()

	assert.True(t, MockPowerState)
	assert.Equal(t, MockMaxPowerWatts, MockEnergyData.Power)
	assert.Equal(t, MockMaxPowerWatts/MockVoltage, MockEnergyData.Current)
	assert.True(t, time.Since(MockStartTime) < time.Second)
	assert.True(t, time.Since(MockPeakPower) < time.Second)
}

func TestSetPower_Off(t *testing.T) {
	Mu.Lock()
	SetPower(false)
	Mu.Unlock()

	assert.False(t, MockPowerState)
	assert.Equal(t, 0.0, MockEnergyData.Power)
	assert.Equal(t, 0.0, MockEnergyData.Current)
}

func TestSetTotalKwh(t *testing.T) {
	Mu.Lock()
	SetEnergy(func(e *MockEnergy) {
		e.Total = 5.0
	})
	assert.Equal(t, 5.0, MockEnergyData.Total)
	Mu.Unlock()
}

func TestSetEnergy(t *testing.T) {
	Mu.Lock()
	SetEnergy(func(e *MockEnergy) {
		e.Power = 999.0
	})
	assert.Equal(t, 999.0, MockEnergyData.Power)
	Mu.Unlock()
}

func TestCalcRealisticPower_Early(t *testing.T) {
	Mu.Lock()
	MockPowerState = true
	MockStartTime = time.Now().Add(-1 * time.Second)
	Mu.Unlock()

	power := CalcRealisticPower()

	assert.GreaterOrEqual(t, power, MockMaxPowerWatts*0.2)
	assert.LessOrEqual(t, power, MockMaxPowerWatts)
}

func TestCalcRealisticPower_Capped(t *testing.T) {
	Mu.Lock()
	MockPowerState = true
	MockStartTime = time.Now().Add(-9 * time.Hour)
	Mu.Unlock()

	power := CalcRealisticPower()

	assert.LessOrEqual(t, power, MockMaxPowerWatts)
}
