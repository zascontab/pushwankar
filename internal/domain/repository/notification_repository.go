package repository

import (
	"context"

	"github.com/google/uuid"
	"notification-service/internal/domain/entity"
)

// NotificationRepository define las operaciones para gestionar notificaciones
type NotificationRepository interface {
	// Guardar una nueva notificación
	Save(ctx context.Context, notification *entity.Notification) error
	
	// Obtener una notificación por su ID
	GetByID(ctx context.Context, id uuid.UUID) (*entity.Notification, error)
	
	// Obtener notificaciones por usuario
	GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*entity.Notification, error)
	
	// Actualizar una notificación
	Update(ctx context.Context, notification *entity.Notification) error
	
	// Eliminar una notificación
	Delete(ctx context.Context, id uuid.UUID) error
	
	// Marcar notificación como expirada
	MarkAsExpired(ctx context.Context, id uuid.UUID) error
	
	// Obtener notificaciones expiradas
	GetExpiredNotifications(ctx context.Context) ([]*entity.Notification, error)
	
	// Contar notificaciones no leídas por usuario
	CountUnreadByUser(ctx context.Context, userID string) (int, error)
}