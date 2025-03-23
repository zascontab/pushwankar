package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"notification-service/internal/domain/entity"
	"notification-service/internal/domain/repository"
	"notification-service/pkg/logging"

	"github.com/google/uuid"
)

// PushService define la interfaz para enviar notificaciones push
type PushService interface {
	// SendNotification envía una notificación a un dispositivo específico
	SendNotification(ctx context.Context, deviceID uuid.UUID, notification *entity.Notification) (string, error)

	// SendBatchNotification envía una notificación a múltiples dispositivos
	SendBatchNotification(ctx context.Context, deviceIDs []uuid.UUID, notification *entity.Notification) (map[uuid.UUID]string, map[uuid.UUID]error)

	// SendUserNotification envía una notificación a todos los dispositivos de un usuario
	SendUserNotification(ctx context.Context, userID string, notification *entity.Notification) (map[uuid.UUID]string, map[uuid.UUID]error)
}

// PushAdapter define la interfaz que deben implementar los adaptadores específicos de plataforma
type PushAdapter interface {
	// Send envía una notificación a través de la plataforma específica
	Send(ctx context.Context, token string, notification *entity.Notification) (string, error)
}

// PushServiceImpl implementa PushService
type PushServiceImpl struct {
	deviceRepo   repository.DeviceRepository
	tokenRepo    repository.TokenRepository
	deliveryRepo repository.DeliveryRepository
	wsAdapter    PushAdapter // Para WebSocket
	fcmAdapter   PushAdapter // Para FCM (Android)
	apnsAdapter  PushAdapter // Para APNS (iOS)
	logger       logging.Logger
}

// NewPushService crea una nueva instancia de PushServiceImpl
func NewPushService(
	deviceRepo repository.DeviceRepository,
	tokenRepo repository.TokenRepository,
	deliveryRepo repository.DeliveryRepository,
	wsAdapter PushAdapter,
	fcmAdapter PushAdapter,
	apnsAdapter PushAdapter,
	logger logging.Logger,
) PushService {
	return &PushServiceImpl{
		deviceRepo:   deviceRepo,
		tokenRepo:    tokenRepo,
		deliveryRepo: deliveryRepo,
		wsAdapter:    wsAdapter,
		fcmAdapter:   fcmAdapter,
		apnsAdapter:  apnsAdapter,
		logger:       logger,
	}
}

// SendNotification envía una notificación a un dispositivo específico
func (s *PushServiceImpl) SendNotification(ctx context.Context, deviceID uuid.UUID, notification *entity.Notification) (string, error) {
	// nose usa devices revisar urgente
	// Obtener el dispositivo
	devices, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		s.logger.Error("Error getting device", "deviceID", deviceID, "error", err, "devices", devices)
		return "", fmt.Errorf("error getting device: %w", err)
	}

	// Obtener todos los tokens para el dispositivo
	tokens, err := s.tokenRepo.GetAllForDevice(ctx, deviceID)
	if err != nil {
		s.logger.Error("Error getting tokens", "deviceID", deviceID, "error", err)
		return "", fmt.Errorf("error getting tokens: %w", err)
	}

	if len(tokens) == 0 {
		s.logger.Warn("No tokens found for device", "deviceID", deviceID)
		return "", errors.New("no tokens found for device")
	}

	// Intentar enviar por cada tipo de token disponible
	for _, token := range tokens {
		// Crear registro de entrega
		delivery := entity.NewDeliveryTracking(notification.ID, deviceID, token.TokenType)
		if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
			s.logger.Error("Error creating delivery tracking", "deviceID", deviceID, "error", err)
			continue
		}

		var adapter PushAdapter
		switch token.TokenType {
		case entity.TokenTypeWebSocket:
			adapter = s.wsAdapter
		case entity.TokenTypeFCM:
			adapter = s.fcmAdapter
		case entity.TokenTypeAPNS:
			adapter = s.apnsAdapter
		default:
			s.logger.Warn("Unsupported token type", "type", token.TokenType, "deviceID", deviceID)
			s.deliveryRepo.MarkAsFailed(ctx, delivery.ID, "Unsupported token type")
			continue
		}

		if adapter == nil {
			s.logger.Warn("Adapter not configured", "type", token.TokenType, "deviceID", deviceID)
			s.deliveryRepo.MarkAsFailed(ctx, delivery.ID, "Adapter not configured")
			continue
		}

		// Enviar notificación
		messageID, err := adapter.Send(ctx, token.Token, notification)
		if err != nil {
			s.logger.Error("Error sending notification",
				"deviceID", deviceID,
				"tokenType", token.TokenType,
				"error", err,
			)
			s.deliveryRepo.MarkAsFailed(ctx, delivery.ID, err.Error())
			continue
		}

		// Marcar como enviado
		s.deliveryRepo.MarkAsSent(ctx, delivery.ID)
		s.logger.Info("Notification sent",
			"deviceID", deviceID,
			"tokenType", token.TokenType,
			"messageID", messageID,
		)

		// Devolver el ID del mensaje y salir al primer éxito
		return messageID, nil
	}

	// Si llegamos aquí, es que ningún envío tuvo éxito
	return "", errors.New("failed to send notification through any channel")
}

// SendBatchNotification envía una notificación a múltiples dispositivos
func (s *PushServiceImpl) SendBatchNotification(
	ctx context.Context,
	deviceIDs []uuid.UUID,
	notification *entity.Notification,
) (map[uuid.UUID]string, map[uuid.UUID]error) {
	results := make(map[uuid.UUID]string)
	errors := make(map[uuid.UUID]error)

	var wg sync.WaitGroup
	var mutex sync.Mutex

	for _, deviceID := range deviceIDs {
		wg.Add(1)
		go func(id uuid.UUID) {
			defer wg.Done()

			// Crear un contexto con timeout para cada envío
			sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			messageID, err := s.SendNotification(sendCtx, id, notification)

			mutex.Lock()
			defer mutex.Unlock()

			if err != nil {
				errors[id] = err
			} else {
				results[id] = messageID
			}
		}(deviceID)
	}

	wg.Wait()
	return results, errors
}

// SendUserNotification envía una notificación a todos los dispositivos de un usuario
func (s *PushServiceImpl) SendUserNotification(
	ctx context.Context,
	userID string,
	notification *entity.Notification,
) (map[uuid.UUID]string, map[uuid.UUID]error) {
	// Convertir userID a uint
	var userIDUint uint
	fmt.Sscanf(userID, "%d", &userIDUint)

	// Obtener todos los dispositivos del usuario
	devices, err := s.deviceRepo.GetByUserID(ctx, userIDUint)
	if err != nil {
		s.logger.Error("Error getting user devices", "userID", userID, "error", err)
		return nil, map[uuid.UUID]error{uuid.Nil: fmt.Errorf("error getting user devices: %w", err)}
	}

	if len(devices) == 0 {
		s.logger.Warn("No devices found for user", "userID", userID)
		return nil, map[uuid.UUID]error{uuid.Nil: errors.New("no devices found for user")}
	}

	// Extraer los IDs de dispositivo
	deviceIDs := make([]uuid.UUID, 0, len(devices))
	for _, device := range devices {
		deviceIDs = append(deviceIDs, device.ID)
	}

	// Enviar a todos los dispositivos
	return s.SendBatchNotification(ctx, deviceIDs, notification)
}

// SendNotification envía una notificación a un dispositivo específico
func (s *PushServiceImpl) SendNotificationX(ctx context.Context, deviceID uuid.UUID, notification *entity.Notification) (string, error) {
	// Obtener el dispositivo (simplemente para verificar que existe)
	device, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		s.logger.Error("Error getting device", "deviceID", deviceID, "error", err)
		return "", fmt.Errorf("error getting device: %w", err)
	}

	// Verificar que el dispositivo existe
	if device == nil {
		s.logger.Error("Device not found", "deviceID", deviceID)
		return "", errors.New("device not found")
	}

	// Obtener todos los tokens para el dispositivo
	tokens, err := s.tokenRepo.GetAllForDevice(ctx, deviceID)
	if err != nil {
		s.logger.Error("Error getting tokens", "deviceID", deviceID, "error", err)
		return "", fmt.Errorf("error getting tokens: %w", err)
	}

	if len(tokens) == 0 {
		s.logger.Warn("No tokens found for device", "deviceID", deviceID)
		return "", errors.New("no tokens found for device")
	}

	// Intentar enviar por cada tipo de token disponible
	for _, token := range tokens {
		// Crear registro de entrega
		delivery := entity.NewDeliveryTracking(notification.ID, deviceID, token.TokenType)
		if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
			s.logger.Error("Error creating delivery tracking", "deviceID", deviceID, "error", err)
			continue
		}

		var adapter PushAdapter
		switch token.TokenType {
		case entity.TokenTypeWebSocket:
			adapter = s.wsAdapter
		case entity.TokenTypeFCM:
			adapter = s.fcmAdapter
		case entity.TokenTypeAPNS:
			adapter = s.apnsAdapter
		default:
			s.logger.Warn("Unsupported token type", "type", token.TokenType, "deviceID", deviceID)
			s.deliveryRepo.MarkAsFailed(ctx, delivery.ID, "Unsupported token type")
			continue
		}

		if adapter == nil {
			s.logger.Warn("Adapter not configured", "type", token.TokenType, "deviceID", deviceID)
			s.deliveryRepo.MarkAsFailed(ctx, delivery.ID, "Adapter not configured")
			continue
		}

		// Enviar notificación
		messageID, err := adapter.Send(ctx, token.Token, notification)
		if err != nil {
			s.logger.Error("Error sending notification",
				"deviceID", deviceID,
				"tokenType", token.TokenType,
				"error", err,
			)
			s.deliveryRepo.MarkAsFailed(ctx, delivery.ID, err.Error())
			continue
		}

		// Marcar como enviado
		s.deliveryRepo.MarkAsSent(ctx, delivery.ID)
		s.logger.Info("Notification sent",
			"deviceID", deviceID,
			"tokenType", token.TokenType,
			"messageID", messageID,
		)

		// Devolver el ID del mensaje y salir al primer éxito
		return messageID, nil
	}

	// Si llegamos aquí, es que ningún envío tuvo éxito
	return "", errors.New("failed to send notification through any channel")
}
