package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"notification-service/internal/domain/entity"
	"notification-service/internal/domain/repository"
)

// NotificationRepository implementa repository.NotificationRepository
type NotificationRepository struct {
	db *sql.DB
}

// NewNotificationRepository crea una instancia de NotificationRepository
func NewNotificationRepository(db *sql.DB) repository.NotificationRepository {
	return &NotificationRepository{db: db}
}

// Save guarda una nueva notificación
func (r *NotificationRepository) Save(ctx context.Context, notification *entity.Notification) error {
	query := `
		INSERT INTO notification_service.notifications 
		(id, user_id, title, message, data, notification_type, sender_id, priority, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		notification.ID,
		notification.UserID,
		notification.Title,
		notification.Message,
		notification.Data,
		notification.NotificationType,
		notification.SenderID,
		notification.Priority,
		notification.CreatedAt,
		notification.ExpiresAt,
	)

	return err
}

// GetByID obtiene una notificación por su ID
func (r *NotificationRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Notification, error) {
	query := `
		SELECT id, user_id, title, message, data, notification_type, sender_id, priority, created_at, expires_at
		FROM notification_service.notifications
		WHERE id = $1
	`

	var notification entity.Notification
	var expiresAt sql.NullTime
	var data []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&notification.ID,
		&notification.UserID,
		&notification.Title,
		&notification.Message,
		&data,
		&notification.NotificationType,
		&notification.SenderID,
		&notification.Priority,
		&notification.CreatedAt,
		&expiresAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotificationNotFound
		}
		return nil, err
	}

	notification.Data = json.RawMessage(data)

	if expiresAt.Valid {
		notification.ExpiresAt = &expiresAt.Time
	}

	return &notification, nil
}

// GetByUserID obtiene notificaciones por usuario
func (r *NotificationRepository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*entity.Notification, error) {
	query := `
		SELECT id, user_id, title, message, data, notification_type, sender_id, priority, created_at, expires_at
		FROM notification_service.notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []*entity.Notification

	for rows.Next() {
		var notification entity.Notification
		var expiresAt sql.NullTime
		var data []byte

		err := rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.Title,
			&notification.Message,
			&data,
			&notification.NotificationType,
			&notification.SenderID,
			&notification.Priority,
			&notification.CreatedAt,
			&expiresAt,
		)

		if err != nil {
			return nil, err
		}

		notification.Data = json.RawMessage(data)

		if expiresAt.Valid {
			notification.ExpiresAt = &expiresAt.Time
		}

		notifications = append(notifications, &notification)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notifications, nil
}

// Update actualiza una notificación
func (r *NotificationRepository) Update(ctx context.Context, notification *entity.Notification) error {
	query := `
		UPDATE notification_service.notifications
		SET title = $2, message = $3, data = $4, notification_type = $5, 
		    sender_id = $6, priority = $7, expires_at = $8
		WHERE id = $1
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		notification.ID,
		notification.Title,
		notification.Message,
		notification.Data,
		notification.NotificationType,
		notification.SenderID,
		notification.Priority,
		notification.ExpiresAt,
	)

	return err
}

// Delete elimina una notificación
func (r *NotificationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		DELETE FROM notification_service.notifications
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// MarkAsExpired marca una notificación como expirada
func (r *NotificationRepository) MarkAsExpired(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE notification_service.notifications
		SET expires_at = $2
		WHERE id = $1 AND (expires_at IS NULL OR expires_at > $2)
	`

	_, err := r.db.ExecContext(ctx, query, id, time.Now())
	return err
}

// GetExpiredNotifications obtiene notificaciones expiradas
func (r *NotificationRepository) GetExpiredNotifications(ctx context.Context) ([]*entity.Notification, error) {
	query := `
		SELECT id, user_id, title, message, data, notification_type, sender_id, priority, created_at, expires_at
		FROM notification_service.notifications
		WHERE expires_at IS NOT NULL AND expires_at < $1
	`

	rows, err := r.db.QueryContext(ctx, query, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []*entity.Notification

	for rows.Next() {
		var notification entity.Notification
		var expiresAt sql.NullTime
		var data []byte

		err := rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.Title,
			&notification.Message,
			&data,
			&notification.NotificationType,
			&notification.SenderID,
			&notification.Priority,
			&notification.CreatedAt,
			&expiresAt,
		)

		if err != nil {
			return nil, err
		}

		notification.Data = json.RawMessage(data)

		if expiresAt.Valid {
			notification.ExpiresAt = &expiresAt.Time
		}

		notifications = append(notifications, &notification)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notifications, nil
}

// CountUnreadByUser cuenta notificaciones no leídas por usuario
func (r *NotificationRepository) CountUnreadByUser(ctx context.Context, userID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM notification_service.notifications n
		LEFT JOIN notification_service.delivery_tracking d ON n.id = d.notification_id
		WHERE n.user_id = $1 AND d.status != 'delivered'
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}