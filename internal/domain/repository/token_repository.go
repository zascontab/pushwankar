package repository

import (
	"context"

	"github.com/google/uuid"
	"notification-service/internal/domain/entity"
)

// TokenRepository define las operaciones para gestionar tokens de notificación
type TokenRepository interface {
	// Crear un nuevo token
	Create(ctx context.Context, token *entity.NotificationToken) error
	
	// Obtener un token por ID de dispositivo y tipo
	GetByDeviceAndType(ctx context.Context, deviceID uuid.UUID, tokenType entity.TokenType) (*entity.NotificationToken, error)
	
	// Obtener todos los tokens para un dispositivo
	GetAllForDevice(ctx context.Context, deviceID uuid.UUID) ([]*entity.NotificationToken, error)
	
	// Obtener todos los tokens para un usuario
	GetAllForUser(ctx context.Context, userID uint) ([]*entity.NotificationToken, error)
	
	// Actualizar un token existente
	Update(ctx context.Context, token *entity.NotificationToken) error
	
	// Revocar un token
	Revoke(ctx context.Context, tokenID uuid.UUID) error
	
	// Revocar todos los tokens de un dispositivo
	RevokeAllForDevice(ctx context.Context, deviceID uuid.UUID) error
	
	// Revocar todos los tokens de un usuario
	RevokeAllForUser(ctx context.Context, userID uint) error
	
	// Limpiar tokens expirados
	CleanupExpired(ctx context.Context) error
	
	// Upsert (actualizar si existe, crear si no) un token
	Upsert(ctx context.Context, deviceID uuid.UUID, tokenValue string, tokenType entity.TokenType) error
}