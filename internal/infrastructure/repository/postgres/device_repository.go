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

// DeviceRepository implementa repository.DeviceRepository
type DeviceRepository struct {
	db *sql.DB
}

// NewDeviceRepository crea una instancia de DeviceRepository
func NewDeviceRepository(db *sql.DB) repository.DeviceRepository {
	return &DeviceRepository{db: db}
}

// GetByID obtiene un dispositivo por su ID
func (r *DeviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Device, error) {
	query := `
        SELECT id, user_id, model, device_identifier, verified, last_access, created_at, updated_at 
        FROM notification_service.devices 
        WHERE id = $1
    `

	var device entity.Device
	var userID sql.NullInt64
	var model sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&device.ID,
		&userID,
		&model,
		&device.DeviceIdentifier,
		&device.Verified,
		&device.LastAccess,
		&device.CreateTime,
		&device.UpdateTime,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrDeviceNotFound
		}
		return nil, err
	}

	if userID.Valid {
		uintUserID := uint(userID.Int64)
		device.UserID = &uintUserID
	}

	if model.Valid {
		device.Model = &model.String
	}

	return &device, nil
}

// GetByDeviceIdentifier obtiene un dispositivo por su identificador
func (r *DeviceRepository) GetByDeviceIdentifier(ctx context.Context, identifier string) (*entity.Device, error) {
	query := `
        SELECT id, user_id, model, device_identifier, verified, last_access, created_at, updated_at 
        FROM notification_service.devices 
        WHERE device_identifier = $1
    `

	var device entity.Device
	var userID sql.NullInt64
	var model sql.NullString

	err := r.db.QueryRowContext(ctx, query, identifier).Scan(
		&device.ID,
		&userID,
		&model,
		&device.DeviceIdentifier,
		&device.Verified,
		&device.LastAccess,
		&device.CreateTime,
		&device.UpdateTime,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrDeviceNotFound
		}
		return nil, err
	}

	if userID.Valid {
		uintUserID := uint(userID.Int64)
		device.UserID = &uintUserID
	}

	if model.Valid {
		device.Model = &model.String
	}

	return &device, nil
}

// GetByUserID obtiene todos los dispositivos de un usuario
func (r *DeviceRepository) GetByUserID(ctx context.Context, userID uint) ([]*entity.Device, error) {
	query := `
        SELECT id, user_id, model, device_identifier, verified, last_access, created_at, updated_at 
        FROM notification_service.devices 
        WHERE user_id = $1
    `

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*entity.Device

	for rows.Next() {
		var device entity.Device
		var dbUserID sql.NullInt64
		var model sql.NullString

		err := rows.Scan(
			&device.ID,
			&dbUserID,
			&model,
			&device.DeviceIdentifier,
			&device.Verified,
			&device.LastAccess,
			&device.CreateTime,
			&device.UpdateTime,
		)

		if err != nil {
			return nil, err
		}

		if dbUserID.Valid {
			uintUserID := uint(dbUserID.Int64)
			device.UserID = &uintUserID
		}

		if model.Valid {
			device.Model = &model.String
		}

		devices = append(devices, &device)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return devices, nil
}

// Save guarda un nuevo dispositivo
func (r *DeviceRepository) Save(ctx context.Context, device *entity.Device) error {
	query := `
        INSERT INTO notification_service.devices (id, user_id, model, device_identifier, verified, last_access, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `

	var userID sql.NullInt64
	if device.UserID != nil {
		userID.Int64 = int64(*device.UserID)
		userID.Valid = true
	}

	var model sql.NullString
	if device.Model != nil {
		model.String = *device.Model
		model.Valid = true
	}

	_, err := r.db.ExecContext(
		ctx,
		query,
		device.ID,
		userID,
		model,
		device.DeviceIdentifier,
		device.Verified,
		device.LastAccess,
		device.CreateTime,
		device.UpdateTime,
	)

	return err
}

// Update actualiza un dispositivo existente
func (r *DeviceRepository) Update(ctx context.Context, device *entity.Device) error {
	query := `
        UPDATE notification_service.devices 
        SET user_id = $2, model = $3, device_identifier = $4, verified = $5, last_access = $6, updated_at = $7
        WHERE id = $1
    `

	var userID sql.NullInt64
	if device.UserID != nil {
		userID.Int64 = int64(*device.UserID)
		userID.Valid = true
	}

	var model sql.NullString
	if device.Model != nil {
		model.String = *device.Model
		model.Valid = true
	}

	_, err := r.db.ExecContext(
		ctx,
		query,
		device.ID,
		userID,
		model,
		device.DeviceIdentifier,
		device.Verified,
		device.LastAccess,
		time.Now(),
	)

	return err
}

// LinkToUser vincula un dispositivo a un usuario
func (r *DeviceRepository) LinkToUser(ctx context.Context, deviceID uuid.UUID, userID uint) error {
	query := `
        UPDATE notification_service.devices 
        SET user_id = $2, updated_at = $3
        WHERE id = $1
    `

	_, err := r.db.ExecContext(ctx, query, deviceID, userID, time.Now())
	return err
}

// UpdateLastAccess actualiza el último acceso de un dispositivo
func (r *DeviceRepository) UpdateLastAccess(ctx context.Context, deviceID uuid.UUID, lastAccess time.Time) error {
	query := `
        UPDATE notification_service.devices 
        SET last_access = $2, updated_at = $3
        WHERE id = $1
    `

	_, err := r.db.ExecContext(ctx, query, deviceID, lastAccess, time.Now())
	return err
}

// MarkAsVerified marca un dispositivo como verificado
func (r *DeviceRepository) MarkAsVerified(ctx context.Context, deviceID uuid.UUID) error {
	query := `
        UPDATE notification_service.devices 
        SET verified = true, updated_at = $2
        WHERE id = $1
    `

	_, err := r.db.ExecContext(ctx, query, deviceID, time.Now())
	return err
}

// GetInactiveDevices obtiene los dispositivos inactivos
func (r *DeviceRepository) GetInactiveDevices(ctx context.Context, threshold time.Time) ([]*entity.Device, error) {
	query := `
        SELECT id, user_id, model, device_identifier, verified, last_access, created_at, updated_at 
        FROM notification_service.devices 
        WHERE last_access < $1
    `

	rows, err := r.db.QueryContext(ctx, query, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*entity.Device

	for rows.Next() {
		var device entity.Device
		var userID sql.NullInt64
		var model sql.NullString

		err := rows.Scan(
			&device.ID,
			&userID,
			&model,
			&device.DeviceIdentifier,
			&device.Verified,
			&device.LastAccess,
			&device.CreateTime,
			&device.UpdateTime,
		)

		if err != nil {
			return nil, err
		}

		if userID.Valid {
			uintUserID := uint(userID.Int64)
			device.UserID = &uintUserID
		}

		if model.Valid {
			device.Model = &model.String
		}

		devices = append(devices, &device)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return devices, nil
}

// Delete elimina un dispositivo
func (r *DeviceRepository) Delete(ctx context.Context, deviceID uuid.UUID) error {
	// En lugar de borrar físicamente, marcamos como eliminado usando DeleteTime
	query := `
        UPDATE notification_service.devices 
        SET updated_at = $2, delete_time = $2
        WHERE id = $1
    `

	_, err := r.db.ExecContext(ctx, query, deviceID, time.Now())
	return err
}
