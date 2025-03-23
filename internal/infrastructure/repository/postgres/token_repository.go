package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"notification-service/internal/domain/entity"
	"notification-service/internal/domain/repository"
)

// TokenRepository implementa repository.TokenRepository
type TokenRepository struct {
	db *sql.DB
}

// NewTokenRepository crea una instancia de TokenRepository
func NewTokenRepository(db *sql.DB) repository.TokenRepository {
	return &TokenRepository{db: db}
}

// Create crea un nuevo token
func (r *TokenRepository) Create(ctx context.Context, token *entity.NotificationToken) error {
	query := `
        INSERT INTO notification_service.notification_tokens 
        (id, device_id, token, token_type, created_at, updated_at, expires_at, is_active, is_revoked)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `
	
	_, err := r.db.ExecContext(
		ctx,
		query,
		token.ID,
		token.DeviceID,
		token.Token,
		token.TokenType,
		token.CreatedAt,
		token.UpdatedAt,
		token.ExpiresAt,
		token.IsActive,
		token.IsRevoked,
	)
	
	return err
}

// GetByDeviceAndType obtiene un token por ID de dispositivo y tipo
func (r *TokenRepository) GetByDeviceAndType(ctx context.Context, deviceID uuid.UUID, tokenType entity.TokenType) (*entity.NotificationToken, error) {
	query := `
        SELECT id, device_id, token, token_type, created_at, updated_at, expires_at, is_active, is_revoked
        FROM notification_service.notification_tokens
        WHERE device_id = $1 AND token_type = $2 AND is_active = true AND is_revoked = false AND expires_at > $3
        ORDER BY created_at DESC
        LIMIT 1
    `
	
	var token entity.NotificationToken
	err := r.db.QueryRowContext(ctx, query, deviceID, tokenType, time.Now()).Scan(
		&token.ID,
		&token.DeviceID,
		&token.Token,
		&token.TokenType,
		&token.CreatedAt,
		&token.UpdatedAt,
		&token.ExpiresAt,
		&token.IsActive,
		&token.IsRevoked,
	)
	
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrTokenNotFound
		}
		return nil, err
	}
	
	return &token, nil
}

// GetAllForDevice obtiene todos los tokens para un dispositivo
func (r *TokenRepository) GetAllForDevice(ctx context.Context, deviceID uuid.UUID) ([]*entity.NotificationToken, error) {
	query := `
        SELECT id, device_id, token, token_type, created_at, updated_at, expires_at, is_active, is_revoked
        FROM notification_service.notification_tokens
        WHERE device_id = $1 AND is_active = true AND is_revoked = false AND expires_at > $2
        ORDER BY created_at DESC
    `
	
	rows, err := r.db.QueryContext(ctx, query, deviceID, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tokens []*entity.NotificationToken
	
	for rows.Next() {
		var token entity.NotificationToken
		
		err := rows.Scan(
			&token.ID,
			&token.DeviceID,
			&token.Token,
			&token.TokenType,
			&token.CreatedAt,
			&token.UpdatedAt,
			&token.ExpiresAt,
			&token.IsActive,
			&token.IsRevoked,
		)
		
		if err != nil {
			return nil, err
		}
		
		tokens = append(tokens, &token)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return tokens, nil
}

// GetAllForUser obtiene todos los tokens para un usuario
func (r *TokenRepository) GetAllForUser(ctx context.Context, userID uint) ([]*entity.NotificationToken, error) {
	query := `
        SELECT t.id, t.device_id, t.token, t.token_type, t.created_at, t.updated_at, t.expires_at, t.is_active, t.is_revoked
        FROM notification_service.notification_tokens t
        JOIN notification_service.devices d ON t.device_id = d.id
        WHERE d.user_id = $1 AND t.is_active = true AND t.is_revoked = false AND t.expires_at > $2
        ORDER BY t.created_at DESC
    `
	
	rows, err := r.db.QueryContext(ctx, query, userID, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tokens []*entity.NotificationToken
	
	for rows.Next() {
		var token entity.NotificationToken
		
		err := rows.Scan(
			&token.ID,
			&token.DeviceID,
			&token.Token,
			&token.TokenType,
			&token.CreatedAt,
			&token.UpdatedAt,
			&token.ExpiresAt,
			&token.IsActive,
			&token.IsRevoked,
		)
		
		if err != nil {
			return nil, err
		}
		
		tokens = append(tokens, &token)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return tokens, nil
}

// Update actualiza un token existente
func (r *TokenRepository) Update(ctx context.Context, token *entity.NotificationToken) error {
	query := `
        UPDATE notification_service.notification_tokens
        SET token = $2, updated_at = $3, expires_at = $4, is_active = $5, is_revoked = $6
        WHERE id = $1
    `
	
	_, err := r.db.ExecContext(
		ctx,
		query,
		token.ID,
		token.Token,
		time.Now(),
		token.ExpiresAt,
		token.IsActive,
		token.IsRevoked,
	)
	
	return err
}

// Revoke revoca un token
func (r *TokenRepository) Revoke(ctx context.Context, tokenID uuid.UUID) error {
	query := `
        UPDATE notification_service.notification_tokens
        SET is_active = false, is_revoked = true, updated_at = $2
        WHERE id = $1
    `
	
	_, err := r.db.ExecContext(ctx, query, tokenID, time.Now())
	return err
}

// RevokeAllForDevice revoca todos los tokens de un dispositivo
func (r *TokenRepository) RevokeAllForDevice(ctx context.Context, deviceID uuid.UUID) error {
	query := `
        UPDATE notification_service.notification_tokens
        SET is_active = false, is_revoked = true, updated_at = $2
        WHERE device_id = $1
    `
	
	_, err := r.db.ExecContext(ctx, query, deviceID, time.Now())
	return err
}

// RevokeAllForUser revoca todos los tokens de un usuario
func (r *TokenRepository) RevokeAllForUser(ctx context.Context, userID uint) error {
	query := `
        UPDATE notification_service.notification_tokens t
        SET is_active = false, is_revoked = true, updated_at = $2
        FROM notification_service.devices d
        WHERE t.device_id = d.id AND d.user_id = $1
    `
	
	_, err := r.db.ExecContext(ctx, query, userID, time.Now())
	return err
}

// CleanupExpired limpia tokens expirados
func (r *TokenRepository) CleanupExpired(ctx context.Context) error {
	query := `
        UPDATE notification_service.notification_tokens
        SET is_active = false, updated_at = $2
        WHERE expires_at < $1
    `
	
	_, err := r.db.ExecContext(ctx, query, time.Now(), time.Now())
	return err
}

// Upsert actualiza o crea un token
func (r *TokenRepository) Upsert(ctx context.Context, deviceID uuid.UUID, tokenValue string, tokenType entity.TokenType) error {
	// Primero intentamos actualizar
	query := `
        UPDATE notification_service.notification_tokens
        SET token = $3, updated_at = $4, expires_at = $5, is_active = true, is_revoked = false
        WHERE device_id = $1 AND token_type = $2
    `
	
	expiresAt := time.Now().Add(30 * 24 * time.Hour) // 30 días
	result, err := r.db.ExecContext(ctx, query, deviceID, tokenType, tokenValue, time.Now(), expiresAt)
	if err != nil {
		return err
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	// Si no se actualizó ningún registro, insertamos uno nuevo
	if rowsAffected == 0 {
		insertQuery := `
            INSERT INTO notification_service.notification_tokens
            (id, device_id, token, token_type, created_at, updated_at, expires_at, is_active, is_revoked)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        `
		
		now := time.Now()
		_, err = r.db.ExecContext(
			ctx,
			insertQuery,
			uuid.New(),
			deviceID,
			tokenValue,
			tokenType,
			now,
			now,
			expiresAt,
			true,
			false,
		)
		
		return err
	}
	
	return nil
}