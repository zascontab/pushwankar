package repository

import (
	"context"
	"time"

	"notification-service/internal/domain/entity"

	"github.com/google/uuid"
)

// DeviceRepository define las operaciones para gestionar dispositivos
type DeviceRepository interface {
	// Obtener un dispositivo por su ID
	GetByID(ctx context.Context, id uuid.UUID) (*entity.Device, error)

	// Obtener un dispositivo por su identificador único
	GetByDeviceIdentifier(ctx context.Context, identifier string) (*entity.Device, error)

	// Obtener todos los dispositivos de un usuario
	GetByUserID(ctx context.Context, userID uint) ([]*entity.Device, error)

	// Guardar un nuevo dispositivo
	Save(ctx context.Context, device *entity.Device) error

	// Actualizar un dispositivo existente
	Update(ctx context.Context, device *entity.Device) error

	// Vincular un dispositivo a un usuario
	LinkToUser(ctx context.Context, deviceID uuid.UUID, userID uint) error

	// Actualizar el último acceso de un dispositivo
	UpdateLastAccess(ctx context.Context, deviceID uuid.UUID, lastAccess time.Time) error

	// Marcar un dispositivo como verificado
	MarkAsVerified(ctx context.Context, deviceID uuid.UUID) error

	// Obtener dispositivos inactivos (sin acceso durante cierto tiempo)
	GetInactiveDevices(ctx context.Context, threshold time.Time) ([]*entity.Device, error)

	// Eliminar un dispositivo (marcarlo como eliminado)
	Delete(ctx context.Context, deviceID uuid.UUID) error
}
