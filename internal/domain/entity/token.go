package entity

import (
	"time"

	"github.com/google/uuid"
)

// TokenType representa los diferentes tipos de tokens de notificación
type TokenType string

const (
	TokenTypeWebSocket TokenType = "websocket"
	TokenTypeAPNS      TokenType = "apns"
	TokenTypeFCM       TokenType = "fcm"
)

// NotificationToken almacena información de tokens para diferentes canales
type NotificationToken struct {
	ID        uuid.UUID `json:"id"`
	DeviceID  uuid.UUID `json:"device_id"`
	Token     string    `json:"token"`
	TokenType TokenType `json:"token_type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	IsActive  bool      `json:"is_active"`
	IsRevoked bool      `json:"is_revoked"`
}

// NewNotificationToken crea un nuevo token de notificación
func NewNotificationToken(deviceID uuid.UUID, token string, tokenType TokenType) *NotificationToken {
	now := time.Now()
	return &NotificationToken{
		ID:        uuid.New(),
		DeviceID:  deviceID,
		Token:     token,
		TokenType: tokenType,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(30 * 24 * time.Hour), // 30 días por defecto
		IsActive:  true,
		IsRevoked: false,
	}
}

// UpdateToken actualiza el token
func (t *NotificationToken) UpdateToken(newToken string) {
	t.Token = newToken
	t.UpdatedAt = time.Now()
	t.ExpiresAt = time.Now().Add(30 * 24 * time.Hour)
	t.IsActive = true
	t.IsRevoked = false
}

// Revoke revoca el token
func (t *NotificationToken) Revoke() {
	t.IsRevoked = true
	t.IsActive = false
	t.UpdatedAt = time.Now()
}

// IsValid verifica si el token es válido
func (t *NotificationToken) IsValid() bool {
	return t.IsActive && !t.IsRevoked && time.Now().Before(t.ExpiresAt)
}