package entity

import (
	"time"

	"github.com/google/uuid"
)

// DeliveryStatus representa el estado de entrega de una notificación
type DeliveryStatus string

const (
	DeliveryStatusPending   DeliveryStatus = "pending"
	DeliveryStatusSent      DeliveryStatus = "sent"
	DeliveryStatusDelivered DeliveryStatus = "delivered"
	DeliveryStatusFailed    DeliveryStatus = "failed"
	DeliveryStatusExpired   DeliveryStatus = "expired"
)

// DeliveryTracking registra el estado de entrega de una notificación
type DeliveryTracking struct {
	ID             uuid.UUID      `json:"id"`
	NotificationID uuid.UUID      `json:"notification_id"`
	DeviceID       uuid.UUID      `json:"device_id"`
	Channel        TokenType      `json:"channel"`
	Status         DeliveryStatus `json:"status"`
	SentAt         *time.Time     `json:"sent_at,omitempty"`
	DeliveredAt    *time.Time     `json:"delivered_at,omitempty"`
	FailedAt       *time.Time     `json:"failed_at,omitempty"`
	RetryCount     int            `json:"retry_count"`
	ErrorMessage   string         `json:"error_message,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// NewDeliveryTracking crea un nuevo registro de seguimiento de entrega
func NewDeliveryTracking(notificationID, deviceID uuid.UUID, channel TokenType) *DeliveryTracking {
	now := time.Now()
	return &DeliveryTracking{
		ID:             uuid.New(),
		NotificationID: notificationID,
		DeviceID:       deviceID,
		Channel:        channel,
		Status:         DeliveryStatusPending,
		RetryCount:     0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// MarkAsSent marca la notificación como enviada
func (d *DeliveryTracking) MarkAsSent() {
	now := time.Now()
	d.Status = DeliveryStatusSent
	d.SentAt = &now
	d.UpdatedAt = now
}

// MarkAsDelivered marca la notificación como entregada
func (d *DeliveryTracking) MarkAsDelivered() {
	now := time.Now()
	d.Status = DeliveryStatusDelivered
	d.DeliveredAt = &now
	d.UpdatedAt = now
}

// MarkAsFailed marca la notificación como fallida
func (d *DeliveryTracking) MarkAsFailed(errorMsg string) {
	now := time.Now()
	d.Status = DeliveryStatusFailed
	d.FailedAt = &now
	d.ErrorMessage = errorMsg
	d.RetryCount++
	d.UpdatedAt = now
}

// MarkAsExpired marca la notificación como expirada
func (d *DeliveryTracking) MarkAsExpired() {
	now := time.Now()
	d.Status = DeliveryStatusExpired
	d.UpdatedAt = now
}

// ShouldRetry determina si se debe reintentar la entrega
func (d *DeliveryTracking) ShouldRetry(maxRetries int) bool {
	return d.Status == DeliveryStatusFailed && d.RetryCount < maxRetries
}
