package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// NotificationType representa los diferentes tipos de notificaciones
type NotificationType string

const (
	NotificationTypeNormal  NotificationType = "normal"
	NotificationTypeUrgent  NotificationType = "urgent"
	NotificationTypeSystem  NotificationType = "system"
	NotificationTypeMessage NotificationType = "message"
)

// Notification representa una notificación a enviar
type Notification struct {
	ID               uuid.UUID        `json:"id"`
	UserID           string           `json:"user_id"`
	Title            string           `json:"title"`
	Message          string           `json:"message"`
	Data             json.RawMessage  `json:"data,omitempty"`
	NotificationType NotificationType `json:"notification_type"`
	SenderID         string           `json:"sender_id,omitempty"`
	Priority         int              `json:"priority"`
	CreatedAt        time.Time        `json:"created_at"`
	ExpiresAt        *time.Time       `json:"expires_at,omitempty"`
}

// NewNotification crea una nueva notificación
func NewNotification(userID, title, message string, data map[string]interface{}, notificationType NotificationType) (*Notification, error) {
	var dataJSON json.RawMessage
	if data != nil {
		var err error
		dataJSON, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}

	return &Notification{
		ID:               uuid.New(),
		UserID:           userID,
		Title:            title,
		Message:          message,
		Data:             dataJSON,
		NotificationType: notificationType,
		Priority:         0, // Prioridad normal por defecto
		CreatedAt:        time.Now(),
	}, nil
}

// SetPriority establece la prioridad de la notificación
func (n *Notification) SetPriority(priority int) {
	n.Priority = priority
}

// SetSender establece el remitente de la notificación
func (n *Notification) SetSender(senderID string) {
	n.SenderID = senderID
}

// SetExpiry establece el tiempo de expiración
func (n *Notification) SetExpiry(duration time.Duration) {
	expiryTime := n.CreatedAt.Add(duration)
	n.ExpiresAt = &expiryTime
}

// IsExpired verifica si la notificación ha expirado
func (n *Notification) IsExpired() bool {
	if n.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*n.ExpiresAt)
}

// GetDataMap convierte los datos de la notificación a un mapa
func (n *Notification) GetDataMap() (map[string]interface{}, error) {
	if len(n.Data) == 0 {
		return make(map[string]interface{}), nil
	}

	var result map[string]interface{}
	err := json.Unmarshal(n.Data, &result)
	return result, err
}
