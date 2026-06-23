package models

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

type RefreshToken struct {
	ID        string     `json:"id"`
	UserID    string     `json:"userId"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expiresAt"`
	RevokedAt *time.Time `json:"-"`
	CreatedAt time.Time  `json:"createdAt"`
}

type MqttCredential struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"createdAt"`
}
