package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"notification-service/internal/domain/entity"
	"notification-service/internal/domain/repository"
	"notification-service/pkg/logging"

	"github.com/google/uuid"
)

// Errores comunes del servicio de entregas
var (
	ErrDeliveryNotFound         = errors.New("delivery record not found")
	ErrFailedToCreateDelivery   = errors.New("failed to create delivery record")
	ErrFailedToUpdateDelivery   = errors.New("failed to update delivery record")
	ErrInvalidDeliveryStatus    = errors.New("invalid delivery status")
	ErrDeliveryAlreadyCompleted = errors.New("delivery already completed")
)

// DeliveryService define las operaciones para gestionar el seguimiento de entregas de notificaciones
type DeliveryService struct {
	deliveryRepo     repository.DeliveryRepository
	notificationRepo repository.NotificationRepository
	deviceRepo       repository.DeviceRepository
	logger           *logging.Logger
}

// NewDeliveryService crea una nueva instancia de DeliveryService
func NewDeliveryService(
	deliveryRepo repository.DeliveryRepository,
	notificationRepo repository.NotificationRepository,
	deviceRepo repository.DeviceRepository,
	logger *logging.Logger,
) *DeliveryService {
	return &DeliveryService{
		deliveryRepo:     deliveryRepo,
		notificationRepo: notificationRepo,
		deviceRepo:       deviceRepo,
		logger:           logger,
	}
}

// CreateDeliveryRecord crea un nuevo registro de entrega para una notificación y dispositivo
func (s *DeliveryService) CreateDeliveryRecord(
	ctx context.Context,
	notificationID uuid.UUID,
	deviceID uuid.UUID,
	channel entity.TokenType,
) (*entity.DeliveryTracking, error) {
	// Verificar que la notificación existe
	notification, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		s.logger.Error("Error getting notification: %v, notificationID: %s", err, notificationID)
		return nil, fmt.Errorf("error getting notification: %w", err)
	}

	// Verificar que el dispositivo existe
	device, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		s.logger.Error("Error getting device: %v, deviceID: %s", err, deviceID)
		return nil, fmt.Errorf("error getting device: %w", err)
	}

	// Crear registro de entrega
	delivery := entity.NewDeliveryTracking(notification.ID, device.ID, channel)

	if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
		s.logger.Error("Error creating delivery record: %v, notificationID: %s, deviceID: %s",
			err, notificationID, deviceID)
		return nil, ErrFailedToCreateDelivery
	}

	s.logger.Info("Delivery record created: deliveryID=%s, notificationID=%s, deviceID=%s",
		delivery.ID, notificationID, deviceID)

	return delivery, nil
}

// MarkAsSent marca un registro de entrega como enviado
func (s *DeliveryService) MarkAsSent(ctx context.Context, deliveryID uuid.UUID) error {
	// Verificar que el registro existe
	delivery, err := s.deliveryRepo.GetByID(ctx, deliveryID)
	if err != nil {
		s.logger.Error("Error getting delivery record: %v, deliveryID: %s", err, deliveryID)
		return ErrDeliveryNotFound
	}

	// Verificar que no esté ya marcado como entregado
	if delivery.Status == entity.DeliveryStatusDelivered {
		s.logger.Warn("Delivery already completed, deliveryID: %s", deliveryID)
		return ErrDeliveryAlreadyCompleted
	}

	// Marcar como enviado
	if err := s.deliveryRepo.MarkAsSent(ctx, deliveryID); err != nil {
		s.logger.Error("Error marking delivery as sent: %v, deliveryID: %s", err, deliveryID)
		return ErrFailedToUpdateDelivery
	}

	s.logger.Info("Delivery marked as sent, deliveryID: %s", deliveryID)
	return nil
}

// MarkAsDelivered marca un registro de entrega como entregado (confirmado por el dispositivo)
func (s *DeliveryService) MarkAsDelivered(ctx context.Context, deliveryID uuid.UUID) error {
	// Verificar que el registro existe
	delivery, err := s.deliveryRepo.GetByID(ctx, deliveryID)
	if err != nil {
		s.logger.Error("Error getting delivery record: %v, deliveryID: %s", err, deliveryID)
		return ErrDeliveryNotFound
	}

	// Verificar que esté marcado como enviado
	if delivery.Status != entity.DeliveryStatusSent {
		s.logger.Warn("Cannot mark as delivered: not in sent state, deliveryID: %s, currentStatus: %s",
			deliveryID, delivery.Status)
		return fmt.Errorf("delivery must be in sent state to mark as delivered: %w", ErrInvalidDeliveryStatus)
	}

	// Marcar como entregado
	if err := s.deliveryRepo.MarkAsDelivered(ctx, deliveryID); err != nil {
		s.logger.Error("Error marking delivery as delivered: %v, deliveryID: %s", err, deliveryID)
		return ErrFailedToUpdateDelivery
	}

	s.logger.Info("Delivery marked as delivered, deliveryID: %s", deliveryID)
	return nil
}

// MarkAsFailed marca un registro de entrega como fallido
func (s *DeliveryService) MarkAsFailed(ctx context.Context, deliveryID uuid.UUID, errorMsg string) error {
	// Verificar que el registro existe
	delivery, err := s.deliveryRepo.GetByID(ctx, deliveryID)
	if err != nil {
		s.logger.Error("Error getting delivery record: %v, deliveryID: %s", err, deliveryID)
		return ErrDeliveryNotFound
	}

	// Verificar que no esté ya marcado como entregado
	if delivery.Status == entity.DeliveryStatusDelivered {
		s.logger.Warn("Cannot mark as failed: already delivered, deliveryID: %s", deliveryID)
		return ErrDeliveryAlreadyCompleted
	}

	// Marcar como fallido
	if err := s.deliveryRepo.MarkAsFailed(ctx, deliveryID, errorMsg); err != nil {
		s.logger.Error("Error marking delivery as failed: %v, deliveryID: %s", err, deliveryID)
		return ErrFailedToUpdateDelivery
	}

	s.logger.Info("Delivery marked as failed, deliveryID: %s, error: %s", deliveryID, errorMsg)
	return nil
}

// ConfirmDelivery confirma la entrega de una notificación a un dispositivo
func (s *DeliveryService) ConfirmDelivery(ctx context.Context, notificationID uuid.UUID, deviceID uuid.UUID) error {
	// Buscar registros de entrega para esta notificación y dispositivo
	deliveries, err := s.deliveryRepo.GetByNotificationID(ctx, notificationID)
	if err != nil {
		s.logger.Error("Error getting delivery records: %v, notificationID: %s", err, notificationID)
		return err
	}

	// Buscar el registro específico para este dispositivo
	var found bool
	for _, delivery := range deliveries {
		if delivery.DeviceID == deviceID {
			// Marcar como entregado
			if err := s.deliveryRepo.MarkAsDelivered(ctx, delivery.ID); err != nil {
				s.logger.Error("Error marking delivery as delivered: %v, deliveryID: %s", err, delivery.ID)
				return ErrFailedToUpdateDelivery
			}
			found = true
			break
		}
	}

	if !found {
		s.logger.Warn("No delivery record found for notification and device, notificationID: %s, deviceID: %s",
			notificationID, deviceID)

		// Si no se encontró, crear un nuevo registro y marcarlo como entregado
		delivery := entity.NewDeliveryTracking(notificationID, deviceID, entity.TokenTypeWebSocket)
		if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
			s.logger.Error("Error creating delivery record: %v", err)
			return ErrFailedToCreateDelivery
		}

		delivery.MarkAsDelivered()
		if err := s.deliveryRepo.MarkAsDelivered(ctx, delivery.ID); err != nil {
			s.logger.Error("Error marking new delivery as delivered: %v", err)
			return ErrFailedToUpdateDelivery
		}

		s.logger.Info("Created and confirmed delivery for notificationID: %s, deviceID: %s",
			notificationID, deviceID)
		return nil
	}

	s.logger.Info("Delivery confirmed, notificationID: %s, deviceID: %s", notificationID, deviceID)
	return nil
}

// GetDeliveryStatus obtiene el estado de entrega de una notificación
func (s *DeliveryService) GetDeliveryStatus(ctx context.Context, notificationID uuid.UUID) ([]*entity.DeliveryTracking, error) {
	deliveries, err := s.deliveryRepo.GetByNotificationID(ctx, notificationID)
	if err != nil {
		s.logger.Error("Error getting delivery status: %v, notificationID: %s", err, notificationID)
		return nil, err
	}

	return deliveries, nil
}

// GetPendingForRetry obtiene entregas pendientes de reintento
func (s *DeliveryService) GetPendingForRetry(ctx context.Context, maxRetries int) ([]*entity.DeliveryTracking, error) {
	return s.deliveryRepo.GetPendingForRetry(ctx, maxRetries)
}

// GetFailedDeliveries obtiene entregas fallidas en un período de tiempo
func (s *DeliveryService) GetFailedDeliveries(ctx context.Context, startTime, endTime time.Time) ([]*entity.DeliveryTracking, error) {
	return s.deliveryRepo.GetFailedByTimeRange(ctx, startTime, endTime)
}

// GetDeviceDeliveries obtiene el historial de entregas para un dispositivo
func (s *DeliveryService) GetDeviceDeliveries(ctx context.Context, deviceID uuid.UUID) ([]*entity.DeliveryTracking, error) {
	return s.deliveryRepo.GetByDeviceID(ctx, deviceID)
}

// UpdateStatus actualiza el estado de un registro de entrega
func (s *DeliveryService) UpdateStatus(ctx context.Context, deliveryID uuid.UUID, status entity.DeliveryStatus) error {
	// Verificar que el registro existe
	delivery, err := s.deliveryRepo.GetByID(ctx, deliveryID)
	if err != nil {
		s.logger.Error("Error getting delivery record: %v, deliveryID: %s", err, deliveryID)
		return ErrDeliveryNotFound
	}

	// Validar la transición de estado
	if !isValidStatusTransition(delivery.Status, status) {
		s.logger.Warn("Invalid status transition, deliveryID: %s, currentStatus: %s, newStatus: %s",
			deliveryID, delivery.Status, status)
		return ErrInvalidDeliveryStatus
	}

	// Actualizar estado
	if err := s.deliveryRepo.UpdateStatus(ctx, deliveryID, status); err != nil {
		s.logger.Error("Error updating delivery status: %v, deliveryID: %s, status: %s",
			err, deliveryID, status)
		return ErrFailedToUpdateDelivery
	}

	s.logger.Info("Delivery status updated, deliveryID: %s, oldStatus: %s, newStatus: %s",
		deliveryID, delivery.Status, status)
	return nil
}

// RetryFailedDeliveries reintenta enviar notificaciones que fallaron
func (s *DeliveryService) RetryFailedDeliveries(ctx context.Context, maxRetries int) error {
	// Obtener entregas fallidas pendientes de reintento
	failedDeliveries, err := s.deliveryRepo.GetPendingForRetry(ctx, maxRetries)
	if err != nil {
		s.logger.Error("Failed to get pending deliveries for retry: %v", err)
		return err
	}

	s.logger.Info("Found %d failed deliveries to retry", len(failedDeliveries))

	// Aquí implementarías la lógica para reintentar enviar estas notificaciones
	// (Por ejemplo, enviarlas a través de un canal diferente)

	return nil
}

// Función auxiliar para validar transiciones de estado
func isValidStatusTransition(from, to entity.DeliveryStatus) bool {
	switch from {
	case entity.DeliveryStatusPending:
		// Desde "pending" se puede ir a cualquier estado
		return true
	case entity.DeliveryStatusSent:
		// Desde "sent" se puede ir a "delivered" o "failed"
		return to == entity.DeliveryStatusDelivered || to == entity.DeliveryStatusFailed
	case entity.DeliveryStatusDelivered:
		// Desde "delivered" no se puede cambiar a ningún otro estado
		return false
	case entity.DeliveryStatusFailed:
		// Desde "failed" solo se puede reintentar (volver a "pending")
		return to == entity.DeliveryStatusPending
	case entity.DeliveryStatusExpired:
		// Desde "expired" no se puede cambiar a ningún otro estado
		return false
	default:
		return false
	}
}
