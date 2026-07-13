package mqtt_test

import (
	"testing"

	"ev-charge-controller/api/mqtt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTopic(t *testing.T) {
	tests := []struct {
		topic     string
		wantNS    string
		wantPfx   string
		wantSlug  string
		wantLeaf  string
		wantErr   bool
	}{
		{
			topic: "evcc/ns-abc123/tele/garage/SENSOR",
			wantNS: "ns-abc123", wantPfx: "tele", wantSlug: "garage", wantLeaf: "SENSOR",
		},
		{
			topic: "evcc/ns-abc123/cmnd/garage/POWER",
			wantNS: "ns-abc123", wantPfx: "cmnd", wantSlug: "garage", wantLeaf: "POWER",
		},
		{
			topic: "evcc/ns-abc123/tele/garage/LWT",
			wantNS: "ns-abc123", wantPfx: "tele", wantSlug: "garage", wantLeaf: "LWT",
		},
		{topic: "wrong/prefix/tele/slug/LEAF", wantErr: true},
		{topic: "evcc/only-three/parts", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			got, err := mqtt.ParseTopic(tt.topic)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantNS, got.Namespace)
			assert.Equal(t, tt.wantPfx, got.Prefix)
			assert.Equal(t, tt.wantSlug, got.Slug)
			assert.Equal(t, tt.wantLeaf, got.Leaf)
		})
	}
}

func TestParseSENSOR(t *testing.T) {
	payload := []byte(`{
		"Time":"2024-01-01T12:00:00",
		"ENERGY":{
			"Total":1.234,
			"Yesterday":0.5,
			"Today":0.1,
			"Power":1500,
			"ApparentPower":1520,
			"ReactivePower":250,
			"Factor":0.98,
			"Voltage":230,
			"Current":6.52
		}
	}`)
	got, err := mqtt.ParseSENSOR(payload)
	require.NoError(t, err)
	assert.InDelta(t, 1.234, got.Total, 0.0001)
	assert.InDelta(t, 1500.0, got.Power, 0.0001)
	assert.InDelta(t, 230.0, got.Voltage, 0.0001)
	assert.InDelta(t, 0.98, got.PowerFactor, 0.0001)
}

func TestParseSENSOR_InvalidJSON(t *testing.T) {
	_, err := mqtt.ParseSENSOR([]byte(`not-json`))
	assert.Error(t, err)
}

func TestParseSTATE(t *testing.T) {
	onPayload := []byte(`{"POWER":"ON","Uptime":"0T00:01:23"}`)
	on, err := mqtt.ParseSTATE(onPayload)
	require.NoError(t, err)
	assert.True(t, on)

	offPayload := []byte(`{"POWER":"OFF"}`)
	off, err := mqtt.ParseSTATE(offPayload)
	require.NoError(t, err)
	assert.False(t, off)
}

func TestParseSTATE_InvalidJSON(t *testing.T) {
	_, err := mqtt.ParseSTATE([]byte(`not-json`))
	assert.Error(t, err)
}

func TestParseSTATE_CaseInsensitive(t *testing.T) {
	on, err := mqtt.ParseSTATE([]byte(`{"POWER":"on"}`))
	require.NoError(t, err)
	assert.True(t, on)
}

func TestParsePowerState(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		wantOn  bool
		wantErr bool
	}{
		{name: "ON uppercase", payload: []byte("ON"), wantOn: true},
		{name: "OFF uppercase", payload: []byte("OFF"), wantOn: false},
		{name: "on lowercase", payload: []byte("on"), wantOn: true},
		{name: "off lowercase", payload: []byte("off"), wantOn: false},
		{name: "On mixed case", payload: []byte("On"), wantOn: true},
		{name: "with whitespace", payload: []byte("  ON  "), wantOn: true},
		{name: "with newline", payload: []byte("OFF\n"), wantOn: false},
		{name: "empty payload", payload: []byte(""), wantErr: true},
		{name: "whitespace only", payload: []byte("   "), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mqtt.ParsePowerState(tt.payload)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOn, got)
		})
	}
}

func TestParseSTATUS10(t *testing.T) {
	payload := []byte(`{"StatusSNS":{"Time":"2026-07-13T01:00:00","ENERGY":{"TotalStartTime":"2024-03-19T13:49:14","Total":12.345,"Yesterday":0.5,"Today":0.1,"Power":600,"ApparentPower":857,"ReactivePower":612,"Factor":0.7,"Voltage":230,"Current":2.6}}}`)
	energy, err := mqtt.ParseSTATUS10(payload)
	if err != nil {
		t.Fatalf("ParseSTATUS10 returned error: %v", err)
	}
	if energy.Total != 12.345 {
		t.Errorf("Total = %v, want 12.345", energy.Total)
	}
	if energy.Power != 600 {
		t.Errorf("Power = %v, want 600", energy.Power)
	}
	if energy.Voltage != 230 {
		t.Errorf("Voltage = %v, want 230", energy.Voltage)
	}
}

func TestParseSTATUS10_InvalidJSON(t *testing.T) {
	if _, err := mqtt.ParseSTATUS10([]byte("not-json")); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseSTATUS10_MissingStatusSNS(t *testing.T) {
	if _, err := mqtt.ParseSTATUS10([]byte(`{"Status":{"Module":0}}`)); err == nil {
		t.Fatal("expected error when StatusSNS envelope is missing")
	}
}
