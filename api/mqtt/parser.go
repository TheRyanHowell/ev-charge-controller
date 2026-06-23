package mqtt

import (
	"encoding/json"
	"fmt"
	"strings"

	"ev-charge-controller/api/tasmota"
)

// Topic layout: evcc/<namespace>/<prefix>/<slug>/<leaf>
// e.g.: evcc/ns-abc123/tele/garage/SENSOR
//        evcc/ns-abc123/cmnd/garage/POWER
//        evcc/ns-abc123/stat/garage/POWER
//        evcc/ns-abc123/tele/garage/LWT

const topicPrefixEvcc = "evcc/"

// ParsedTopic holds the decomposed fields of a Tasmota MQTT topic.
type ParsedTopic struct {
	Namespace string // e.g. "ns-abc123"
	Prefix    string // "tele", "cmnd", "stat"
	Slug      string // Tasmota Topic value, e.g. "garage"
	Leaf      string // "SENSOR", "POWER", "LWT", etc.
}

// ParseTopic parses a full MQTT topic string into its parts.
// Returns an error if the topic doesn't match the expected evcc layout.
func ParseTopic(topic string) (*ParsedTopic, error) {
	if !strings.HasPrefix(topic, topicPrefixEvcc) {
		return nil, fmt.Errorf("unexpected topic prefix: %s", topic)
	}
	rest := topic[len(topicPrefixEvcc):]
	parts := strings.SplitN(rest, "/", 4)
	if len(parts) != 4 {
		return nil, fmt.Errorf("expected 4 parts after evcc/, got %d in %q", len(parts), topic)
	}
	return &ParsedTopic{
		Namespace: parts[0],
		Prefix:    parts[1],
		Slug:      parts[2],
		Leaf:      parts[3],
	}, nil
}

// sensorPayload matches the Tasmota tele/SENSOR JSON envelope.
type sensorPayload struct {
	Time   string `json:"Time"`
	ENERGY struct {
		Total         float64 `json:"Total"`
		Yesterday     float64 `json:"Yesterday"`
		Today         float64 `json:"Today"`
		Power         float64 `json:"Power"`
		ApparentPower float64 `json:"ApparentPower"`
		ReactivePower float64 `json:"ReactivePower"`
		Factor        float64 `json:"Factor"`
		Voltage       float64 `json:"Voltage"`
		Current       float64 `json:"Current"`
	} `json:"ENERGY"`
}

// ParseSENSOR decodes a tele/SENSOR payload into a tasmota.EnergyData.
func ParseSENSOR(payload []byte) (*tasmota.EnergyData, error) {
	var p sensorPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("parse SENSOR: %w", err)
	}
	return &tasmota.EnergyData{
		Total:       p.ENERGY.Total,
		Yesterday:   p.ENERGY.Yesterday,
		Today:       p.ENERGY.Today,
		Power:       p.ENERGY.Power,
		Apparent:    p.ENERGY.ApparentPower,
		Reactive:    p.ENERGY.ReactivePower,
		PowerFactor: p.ENERGY.Factor,
		Voltage:     p.ENERGY.Voltage,
		Current:     p.ENERGY.Current,
	}, nil
}

// statePayload matches the Tasmota tele/STATE JSON envelope.
type statePayload struct {
	Power string `json:"POWER"`
}

// ParseSTATE decodes a tele/STATE payload and returns whether power is ON.
func ParseSTATE(payload []byte) (bool, error) {
	var p statePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return false, fmt.Errorf("parse STATE: %w", err)
	}
	return strings.EqualFold(p.Power, "ON"), nil
}

// ParsePowerState decodes a stat/POWER payload (plain text "ON" or "OFF")
// and returns whether power is ON.
func ParsePowerState(payload []byte) (bool, error) {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return false, fmt.Errorf("parse power state: empty payload")
	}
	return strings.EqualFold(trimmed, "ON"), nil
}
