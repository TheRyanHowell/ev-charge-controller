package models

import "time"

type Plug struct {
	ID                    string     `json:"id"`
	UserID                string     `json:"userId"`
	Name                  string     `json:"name"`
	Namespace             string     `json:"namespace"`
	MqttTopic             string     `json:"mqttTopic"`
	TLS                   bool       `json:"tls"`
	Online                bool       `json:"online"`
	Initialized           bool       `json:"initialized"`
	Type                  string     `json:"type"`
	PowerOn               bool       `json:"powerOn"`
	LastSeen              *time.Time `json:"lastSeen,omitempty"`
	LastOfflineNotifiedAt *time.Time `json:"-"`
	VehicleID             *string    `json:"vehicleId,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
}
