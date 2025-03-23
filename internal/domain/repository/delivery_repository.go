package repository

import (
	"context"
	"time"

	"notification-service/internal/domain/entity"

	"github.com/google/uuid"
)

// DeliveryRepository define las operaciones para seguimiento de entregas
type DeliveryRepository interface {
	// Crear un nuevo registro de entrega
	Create(ctx context.Context, delivery *entity.DeliveryTracking) error

	// Obtener por ID
	GetByID(ctx context.Context, id uuid.UUID) (*entity.DeliveryTracking, error)

	// Obtener todos los registros para una notificación
	GetByNotificationID(ctx context.Context, notificationID uuid.UUID) ([]*entity.DeliveryTracking, error)

	// Obtener registros de entrega por dispositivo
	GetByDeviceID(ctx context.Context, deviceID uuid.UUID) ([]*entity.DeliveryTracking, error)

	// Actualizar estado a enviado
	MarkAsSent(ctx context.Context, id uuid.UUID) error

	// Actualizar estado a entregado
	MarkAsDelivered(ctx context.Context, id uuid.UUID) error

	// Actualizar estado a fallido
	MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error

	// Actualizar estado
	UpdateStatus(ctx context.Context, id uuid.UUID, status entity.DeliveryStatus) error

	// Obtener entregas pendientes para reintento
	GetPendingForRetry(ctx context.Context, maxRetries int) ([]*entity.DeliveryTracking, error)

	// Obtener entregas fallidas por período
	GetFailedByTimeRange(ctx context.Context, start, end time.Time) ([]*entity.DeliveryTracking, error)

	// Obtener estadísticas de entrega por usuario
	GetUserDeliveryStats(ctx context.Context, userID string) (map[entity.DeliveryStatus]int, error)
}
