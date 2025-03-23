package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"notification-service/internal/domain/entity"
	"notification-service/internal/domain/repository"

	"github.com/google/uuid"
)

// DeliveryRepository implementa repository.DeliveryRepository
type DeliveryRepository struct {
	db *sql.DB
}

// GetUserDeliveryStats implements repository.DeliveryRepository.
func (r *DeliveryRepository) GetUserDeliveryStats(ctx context.Context, userID string) (map[entity.DeliveryStatus]int, error) {
	panic("unimplemented")
}

// NewDeliveryRepository crea una instancia de DeliveryRepository
func NewDeliveryRepository(db *sql.DB) repository.DeliveryRepository {
	return &DeliveryRepository{db: db}
}

// Create crea un nuevo registro de entrega
func (r *DeliveryRepository) Create(ctx context.Context, delivery *entity.DeliveryTracking) error {
	query := `
		INSERT INTO notification_service.delivery_tracking 
		(id, notification_id, device_id, channel, status, retry_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		delivery.ID,
		delivery.NotificationID,
		delivery.DeviceID,
		delivery.Channel,
		delivery.Status,
		delivery.RetryCount,
		delivery.CreatedAt,
		delivery.UpdatedAt,
	)

	return err
}

// GetByID obtiene un registro de entrega por su ID
func (r *DeliveryRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.DeliveryTracking, error) {
	query := `
		SELECT id, notification_id, device_id, channel, status, sent_at, delivered_at, failed_at, 
		       retry_count, error_message, created_at, updated_at
		FROM notification_service.delivery_tracking
		WHERE id = $1
	`

	var delivery entity.DeliveryTracking
	var sentAt, deliveredAt, failedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&delivery.ID,
		&delivery.NotificationID,
		&delivery.DeviceID,
		&delivery.Channel,
		&delivery.Status,
		&sentAt,
		&deliveredAt,
		&failedAt,
		&delivery.RetryCount,
		&delivery.ErrorMessage,
		&delivery.CreatedAt,
		&delivery.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("delivery tracking not found")
		}
		return nil, err
	}

	if sentAt.Valid {
		delivery.SentAt = &sentAt.Time
	}
	if deliveredAt.Valid {
		delivery.DeliveredAt = &deliveredAt.Time
	}
	if failedAt.Valid {
		delivery.FailedAt = &failedAt.Time
	}

	return &delivery, nil
}

// GetByNotificationID obtiene registros de entrega por ID de notificación
func (r *DeliveryRepository) GetByNotificationID(ctx context.Context, notificationID uuid.UUID) ([]*entity.DeliveryTracking, error) {
	query := `
		SELECT id, notification_id, device_id, channel, status, sent_at, delivered_at, failed_at, 
		       retry_count, error_message, created_at, updated_at
		FROM notification_service.delivery_tracking
		WHERE notification_id = $1
	`

	rows, err := r.db.QueryContext(ctx, query, notificationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []*entity.DeliveryTracking

	for rows.Next() {
		var delivery entity.DeliveryTracking
		var sentAt, deliveredAt, failedAt sql.NullTime

		err := rows.Scan(
			&delivery.ID,
			&delivery.NotificationID,
			&delivery.DeviceID,
			&delivery.Channel,
			&delivery.Status,
			&sentAt,
			&deliveredAt,
			&failedAt,
			&delivery.RetryCount,
			&delivery.ErrorMessage,
			&delivery.CreatedAt,
			&delivery.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		if sentAt.Valid {
			delivery.SentAt = &sentAt.Time
		}
		if deliveredAt.Valid {
			delivery.DeliveredAt = &deliveredAt.Time
		}
		if failedAt.Valid {
			delivery.FailedAt = &failedAt.Time
		}

		deliveries = append(deliveries, &delivery)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return deliveries, nil
}

// GetByDeviceID obtiene registros de entrega por ID de dispositivo
func (r *DeliveryRepository) GetByDeviceID(ctx context.Context, deviceID uuid.UUID) ([]*entity.DeliveryTracking, error) {
	query := `
		SELECT id, notification_id, device_id, channel, status, sent_at, delivered_at, failed_at, 
		       retry_count, error_message, created_at, updated_at
		FROM notification_service.delivery_tracking
		WHERE device_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []*entity.DeliveryTracking

	for rows.Next() {
		var delivery entity.DeliveryTracking
		var sentAt, deliveredAt, failedAt sql.NullTime

		err := rows.Scan(
			&delivery.ID,
			&delivery.NotificationID,
			&delivery.DeviceID,
			&delivery.Channel,
			&delivery.Status,
			&sentAt,
			&deliveredAt,
			&failedAt,
			&delivery.RetryCount,
			&delivery.ErrorMessage,
			&delivery.CreatedAt,
			&delivery.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		if sentAt.Valid {
			delivery.SentAt = &sentAt.Time
		}
		if deliveredAt.Valid {
			delivery.DeliveredAt = &deliveredAt.Time
		}
		if failedAt.Valid {
			delivery.FailedAt = &failedAt.Time
		}

		deliveries = append(deliveries, &delivery)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return deliveries, nil
}

// MarkAsSent marca un registro como enviado
func (r *DeliveryRepository) MarkAsSent(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE notification_service.delivery_tracking
		SET status = $2, sent_at = $3, updated_at = $4
		WHERE id = $1
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, id, string(entity.DeliveryStatusSent), now, now)
	return err
}

// MarkAsDelivered marca un registro como entregado
func (r *DeliveryRepository) MarkAsDelivered(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE notification_service.delivery_tracking
		SET status = $2, delivered_at = $3, updated_at = $4
		WHERE id = $1
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, id, string(entity.DeliveryStatusDelivered), now, now)
	return err
}

// MarkAsFailed marca un registro como fallido
func (r *DeliveryRepository) MarkAsFailed(ctx context.Context, id uuid.UUID, errorMsg string) error {
	query := `
		UPDATE notification_service.delivery_tracking
		SET status = $2, failed_at = $3, error_message = $4, retry_count = retry_count + 1, updated_at = $5
		WHERE id = $1
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, id, string(entity.DeliveryStatusFailed), now, errorMsg, now)
	return err
}

// UpdateStatus actualiza el estado de un registro
func (r *DeliveryRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.DeliveryStatus) error {
	query := `
		UPDATE notification_service.delivery_tracking
		SET status = $2, updated_at = $3
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, id, string(status), time.Now())
	return err
}

// GetPendingForRetry obtiene entregas pendientes para reintento
func (r *DeliveryRepository) GetPendingForRetry(ctx context.Context, maxRetries int) ([]*entity.DeliveryTracking, error) {
	query := `
		SELECT id, notification_id, device_id, channel, status, sent_at, delivered_at, failed_at, 
		       retry_count, error_message, created_at, updated_at
		FROM notification_service.delivery_tracking
		WHERE status IN ($1, $2) AND retry_count < $3
		ORDER BY updated_at ASC
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		string(entity.DeliveryStatusPending),
		string(entity.DeliveryStatusFailed),
		maxRetries,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []*entity.DeliveryTracking

	for rows.Next() {
		var delivery entity.DeliveryTracking
		var sentAt, deliveredAt, failedAt sql.NullTime

		err := rows.Scan(
			&delivery.ID,
			&delivery.NotificationID,
			&delivery.DeviceID,
			&delivery.Channel,
			&delivery.Status,
			&sentAt,
			&deliveredAt,
			&failedAt,
			&delivery.RetryCount,
			&delivery.ErrorMessage,
			&delivery.CreatedAt,
			&delivery.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		if sentAt.Valid {
			delivery.SentAt = &sentAt.Time
		}
		if deliveredAt.Valid {
			delivery.DeliveredAt = &deliveredAt.Time
		}
		if failedAt.Valid {
			delivery.FailedAt = &failedAt.Time
		}

		deliveries = append(deliveries, &delivery)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return deliveries, nil
}

// GetFailedByTimeRange obtiene entregas fallidas por período
func (r *DeliveryRepository) GetFailedByTimeRange(ctx context.Context, start, end time.Time) ([]*entity.DeliveryTracking, error) {
	query := `
		SELECT id, notification_id, device_id, channel, status, sent_at, delivered_at, failed_at, 
		       retry_count, error_message, created_at, updated_at
		FROM notification_service.delivery_tracking
		WHERE status = $1 AND failed_at BETWEEN $2 AND $3
		ORDER BY failed_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, string(entity.DeliveryStatusFailed), start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deliveries []*entity.DeliveryTracking

	for rows.Next() {
		var delivery entity.DeliveryTracking
		var sentAt, deliveredAt, failedAt sql.NullTime

		err := rows.Scan(
			&delivery.ID,
			&delivery.NotificationID,
			&delivery.DeviceID,
			&delivery.Channel,
			&delivery.Status,
			&sentAt,
			&deliveredAt,
			&failedAt,
			&delivery.RetryCount,
			&delivery.ErrorMessage,
			&delivery.CreatedAt,
			&delivery.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		if sentAt.Valid {
			delivery.SentAt = &sentAt.Time
		}
		if deliveredAt.Valid {
			delivery.DeliveredAt = &deliveredAt.Time
		}
		if failedAt.Valid {
			delivery.FailedAt = &failedAt.Time
		}

		deliveries = append(deliveries, &delivery)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return deliveries, nil
}
