package models

import (
	"time"
)

type PushSubscription struct {
	ID        string    `json:"id"`
	Endpoint  string    `json:"endpoint"`
	P256dhKey string    `json:"p256dhKey"`
	AuthKey   string    `json:"authKey"`
	UserID    *string   `json:"userId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}
