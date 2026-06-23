package main

type EnergyData struct {
	Total     float64 `json:"total"`
	LastTotal float64 `json:"lastTotal"`
	Yesterday float64 `json:"yesterday"`
	Today     float64 `json:"today"`
	Period    int     `json:"period"`
	Power     float64 `json:"power"`
	Apparent  float64 `json:"apparent"`
	Reactive  float64 `json:"reactive"`
	Freq      float64 `json:"freq"`
	Voltage   float64 `json:"voltage"`
	Current   float64 `json:"current"`
}

type PowerResponse struct {
	POWER string `json:"POWER"`
}

type StatusResponse struct {
	Status map[string]interface{} `json:"Status"`
}

type CommandResponse struct {
	Command string `json:"Command"`
}

// mqttConfig holds the MQTT connection parameters pushed by ConfigureTasmotaDevice.
type mqttConfig struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Namespace string `json:"namespace"` // parsed from FullTopic: evcc/<namespace>/...
	Slug      string `json:"slug"`      // Topic value
}
