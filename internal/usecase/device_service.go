package usecase

import (
	"context"
	"errors"
	"time"

	"notification-service/internal/domain/entity"
	"notification-service/internal/domain/repository"

	"github.com/google/uuid"
)

// Errores comunes del servicio de dispositivos
var (
	ErrDeviceNotFound       = errors.New("device not found")
	ErrInvalidDeviceID      = errors.New("invalid device ID")
	ErrDeviceAlreadyExists  = errors.New("device already exists")
	ErrFailedToSaveDevice   = errors.New("failed to save device")
	ErrFailedToUpdateDevice = errors.New("failed to update device")
	ErrInvalidUserID        = errors.New("invalid user ID")
)

// DeviceService define las operaciones de negocio para gestionar dispositivos
type DeviceService struct {
	deviceRepo repository.DeviceRepository
	tokenRepo  repository.TokenRepository
}

// NewDeviceService crea una nueva instancia del servicio de dispositivos
func NewDeviceService(deviceRepo repository.DeviceRepository, tokenRepo repository.TokenRepository) *DeviceService {
	return &DeviceService{
		deviceRepo: deviceRepo,
		tokenRepo:  tokenRepo,
	}
}

// RegisterDeviceWithoutUser registra un nuevo dispositivo sin usuario asociado
func (s *DeviceService) RegisterDeviceWithoutUser(ctx context.Context, deviceIdentifier string, model *string) (*entity.Device, error) {
	// Verificar si ya existe
	existingDevice, err := s.deviceRepo.GetByDeviceIdentifier(ctx, deviceIdentifier)
	if err != nil && !errors.Is(err, ErrDeviceNotFound) {
		return nil, err
	}

	if existingDevice != nil {
		existingDevice.UpdateLastAccess()
		if model != nil {
			existingDevice.Model = model
		}
		if err := s.deviceRepo.Update(ctx, existingDevice); err != nil {
			return nil, ErrFailedToUpdateDevice
		}
		return existingDevice, nil
	}

	// Crear nuevo dispositivo
	newDevice := entity.NewDevice(deviceIdentifier, nil, model)
	if err := s.deviceRepo.Save(ctx, newDevice); err != nil {
		return nil, ErrFailedToSaveDevice
	}

	return newDevice, nil
}

// RegisterDeviceWithUser registra un nuevo dispositivo asociado a un usuario
func (s *DeviceService) RegisterDeviceWithUser(ctx context.Context, deviceIdentifier string, userID uint, model *string) (*entity.Device, error) {
	// Verificar si ya existe
	existingDevice, err := s.deviceRepo.GetByDeviceIdentifier(ctx, deviceIdentifier)
	if err != nil && !errors.Is(err, ErrDeviceNotFound) {
		return nil, err
	}

	if existingDevice != nil {
		existingDevice.UpdateLastAccess()
		existingDevice.LinkToUser(userID)
		if model != nil {
			existingDevice.Model = model
		}
		if err := s.deviceRepo.Update(ctx, existingDevice); err != nil {
			return nil, ErrFailedToUpdateDevice
		}
		return existingDevice, nil
	}

	// Crear nuevo dispositivo
	newDevice := entity.NewDevice(deviceIdentifier, &userID, model)
	if err := s.deviceRepo.Save(ctx, newDevice); err != nil {
		return nil, ErrFailedToSaveDevice
	}

	return newDevice, nil
}

// GetDevice obtiene un dispositivo por su ID
func (s *DeviceService) GetDevice(ctx context.Context, deviceID uuid.UUID) (*entity.Device, error) {
	device, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	return device, nil
}

// LinkDeviceToUser vincula un dispositivo a un usuario
func (s *DeviceService) LinkDeviceToUser(ctx context.Context, deviceID uuid.UUID, userID uint) error {
	// Verificar si el dispositivo existe
	device, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return ErrDeviceNotFound
	}

	// Actualizar la vinculación
	device.LinkToUser(userID)
	if err := s.deviceRepo.Update(ctx, device); err != nil {
		return ErrFailedToUpdateDevice
	}

	return nil
}

// UpdateDeviceLastAccess actualiza el último acceso de un dispositivo
func (s *DeviceService) UpdateDeviceLastAccess(ctx context.Context, deviceID uuid.UUID) error {
	return s.deviceRepo.UpdateLastAccess(ctx, deviceID, time.Now())
}

// GetUserDevices obtiene todos los dispositivos de un usuario
func (s *DeviceService) GetUserDevices(ctx context.Context, userID uint) ([]*entity.Device, error) {
	return s.deviceRepo.GetByUserID(ctx, userID)
}

// CleanupInactiveDevices elimina los dispositivos inactivos
func (s *DeviceService) CleanupInactiveDevices(ctx context.Context, inactivityThreshold time.Duration) error {
	threshold := time.Now().Add(-inactivityThreshold)
	devices, err := s.deviceRepo.GetInactiveDevices(ctx, threshold)
	if err != nil {
		return err
	}

	for _, device := range devices {
		// Revocar tokens antes de eliminar
		if err := s.tokenRepo.RevokeAllForDevice(ctx, device.ID); err != nil {
			continue
		}

		// Eliminar dispositivo
		if err := s.deviceRepo.Delete(ctx, device.ID); err != nil {
			continue
		}
	}

	return nil
}
