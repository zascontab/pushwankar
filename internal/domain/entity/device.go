package entity

import (
	"time"

	"github.com/google/uuid"
)

// Device representa un dispositivo que puede recibir notificaciones
type Device struct {
	ID               uuid.UUID  `json:"id"`
	UserID           *uint      `json:"user_id,omitempty"`
	Model            *string    `json:"model,omitempty"`
	DeviceIdentifier string     `json:"device_identifier"`
	Verified         bool       `json:"verified"`
	LastAccess       time.Time  `json:"last_access"`
	LastUsed         time.Time  `json:"last_used"`
	CreateTime       time.Time  `json:"create_time"`
	UpdateTime       time.Time  `json:"update_time"`
	DeleteTime       *time.Time `json:"delete_time,omitempty"`
	Status           string     `json:"status,omitempty"`
}

// NewDevice crea una nueva instancia de Device
func NewDevice(deviceIdentifier string, userID *uint, model *string) *Device {
	now := time.Now()
	return &Device{
		ID:               uuid.New(),
		UserID:           userID,
		Model:            model,
		DeviceIdentifier: deviceIdentifier,
		Verified:         false,
		LastAccess:       now,
		LastUsed:         now,
		CreateTime:       now,
		UpdateTime:       now,
		Status:           "active",
	}
}

// UpdateLastAccess actualiza el último tiempo de acceso del dispositivo
func (d *Device) UpdateLastAccess() {
	d.LastAccess = time.Now()
	d.UpdateTime = time.Now()
}

// MarkVerified marca el dispositivo como verificado
func (d *Device) MarkVerified() {
	d.Verified = true
	d.UpdateTime = time.Now()
}

// IsActive verifica si el dispositivo está activo
func (d *Device) IsActive() bool {
	return d.Status == "active" && d.DeleteTime == nil
}

// LinkToUser vincula el dispositivo a un usuario
func (d *Device) LinkToUser(userID uint) {
	d.UserID = &userID
	d.UpdateTime = time.Now()
}
