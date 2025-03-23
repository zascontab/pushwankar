package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"notification-service/internal/domain/entity"
	"notification-service/internal/domain/repository"
	"notification-service/pkg/logging"

	"github.com/google/uuid"
)

// Errores comunes del servicio de notificaciones
var (
	ErrNotificationNotFound     = errors.New("notification not found")
	ErrFailedToSaveNotification = errors.New("failed to save notification")
	ErrInvalidNotificationData  = errors.New("invalid notification data")
	ErrDeliveryFailed           = errors.New("failed to deliver notification")
	ErrUserHasNoDevices         = errors.New("user has no registered devices")
)

// NotificationService define las operaciones de negocio para gestionar notificaciones
type NotificationService struct {
	notificationRepo repository.NotificationRepository
	deliveryRepo     repository.DeliveryRepository
	deviceRepo       repository.DeviceRepository
	tokenRepo        repository.TokenRepository
	wsManager        WebSocketManager
	logger           *logging.Logger
}

// WebSocketManager define las operaciones para enviar mensajes WebSocket
type WebSocketManager interface {
	SendMessage(deviceID uuid.UUID, payload []byte) error
	GetConnectedDevices() []uuid.UUID
	IsDeviceConnected(deviceID uuid.UUID) bool

	//
	SendToDevice(deviceID uuid.UUID, payload []byte) bool
	SendToUser(userID string, payload []byte) bool
}

// NewNotificationService crea una nueva instancia del servicio de notificaciones
func NewNotificationService(
	notificationRepo repository.NotificationRepository,
	deliveryRepo repository.DeliveryRepository,
	deviceRepo repository.DeviceRepository,
	tokenRepo repository.TokenRepository,
	wsManager WebSocketManager,
	logger *logging.Logger,
) *NotificationService {
	return &NotificationService{
		notificationRepo: notificationRepo,
		deliveryRepo:     deliveryRepo,
		deviceRepo:       deviceRepo,
		tokenRepo:        tokenRepo,
		wsManager:        wsManager,
		logger:           logger,
	}
}

// SendNotification envía una notificación a un usuario
func (s *NotificationService) SendNotification(
	ctx context.Context,
	userID, title, message string,
	data map[string]interface{},
	notificationType entity.NotificationType,
) (string, error) {
	// Crear notificación
	notification, err := entity.NewNotification(userID, title, message, data, notificationType)
	if err != nil {
		return "", ErrInvalidNotificationData
	}

	// Guardar en repositorio
	if err := s.notificationRepo.Save(ctx, notification); err != nil {
		return "", ErrFailedToSaveNotification
	}

	// Obtener dispositivos del usuario
	var userIDUint uint
	fmt.Sscanf(userID, "%d", &userIDUint)
	devices, err := s.deviceRepo.GetByUserID(ctx, userIDUint)
	if err != nil {
		return "", err
	}

	if len(devices) == 0 {
		return notification.ID.String(), ErrUserHasNoDevices
	}

	// Intentar enviar a cada dispositivo
	var deliveryErrors []error
	deliveredToAny := false

	for _, device := range devices {
		// Primero intentar WebSocket si está conectado
		if s.wsManager.IsDeviceConnected(device.ID) {
			// Crear registro de entrega
			delivery := entity.NewDeliveryTracking(notification.ID, device.ID, entity.TokenTypeWebSocket)
			if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
				deliveryErrors = append(deliveryErrors, err)
				continue
			}

			// Preparar payload
			payload, err := s.prepareNotificationPayload(notification)
			if err != nil {
				deliveryErrors = append(deliveryErrors, err)
				s.deliveryRepo.MarkAsFailed(ctx, delivery.ID, "Failed to prepare payload")
				continue
			}

			// Enviar por WebSocket
			if err := s.wsManager.SendMessage(device.ID, payload); err != nil {
				deliveryErrors = append(deliveryErrors, err)
				s.deliveryRepo.MarkAsFailed(ctx, delivery.ID, err.Error())
				continue
			}

			// Marcar como enviado
			s.deliveryRepo.MarkAsSent(ctx, delivery.ID)
			deliveredToAny = true
			continue
		}

		// Si WebSocket no está disponible, intentar otros canales (implementar después)
		// ...
	}

	if !deliveredToAny && len(deliveryErrors) > 0 {
		return notification.ID.String(), ErrDeliveryFailed
	}

	return notification.ID.String(), nil
}

// prepareNotificationPayload prepara el payload para enviar
func (s *NotificationService) prepareNotificationPayload(notification *entity.Notification) ([]byte, error) {
	dataMap, err := notification.GetDataMap()
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"type":            "notification",
		"notification_id": notification.ID.String(),
		"title":           notification.Title,
		"message":         notification.Message,
		"data":            dataMap,
		"timestamp":       notification.CreatedAt.Unix(),
	}

	return json.Marshal(payload)
}

// GetNotification obtiene una notificación por su ID
func (s *NotificationService) GetNotification(ctx context.Context, id uuid.UUID) (*entity.Notification, error) {
	return s.notificationRepo.GetByID(ctx, id)
}

// GetDeliveryStatus obtiene el estado de entrega de una notificación
func (s *NotificationService) GetDeliveryStatus(ctx context.Context, notificationID uuid.UUID) ([]*entity.DeliveryTracking, error) {
	return s.deliveryRepo.GetByNotificationID(ctx, notificationID)
}

// ResendFailedNotifications reintenta enviar notificaciones fallidas
func (s *NotificationService) ResendFailedNotifications(ctx context.Context, maxRetries int) error {
	// Obtener entregas pendientes para reintento
	deliveries, err := s.deliveryRepo.GetPendingForRetry(ctx, maxRetries)
	if err != nil {
		return err
	}

	for _, delivery := range deliveries {
		// Obtener la notificación
		notification, err := s.notificationRepo.GetByID(ctx, delivery.NotificationID)
		if err != nil {
			continue
		}

		// Verificar si la notificación ha expirado
		if notification.IsExpired() {
			s.deliveryRepo.UpdateStatus(ctx, delivery.ID, entity.DeliveryStatusExpired)
			continue
		}

		// Preparar payload
		payload, err := s.prepareNotificationPayload(notification)
		if err != nil {
			s.deliveryRepo.MarkAsFailed(ctx, delivery.ID, "Failed to prepare payload")
			continue
		}

		// Intentar reenviar según el canal
		switch delivery.Channel {
		case entity.TokenTypeWebSocket:
			if s.wsManager.IsDeviceConnected(delivery.DeviceID) {
				if err := s.wsManager.SendMessage(delivery.DeviceID, payload); err != nil {
					s.deliveryRepo.MarkAsFailed(ctx, delivery.ID, err.Error())
					continue
				}
				s.deliveryRepo.MarkAsSent(ctx, delivery.ID)
			} else {
				s.deliveryRepo.MarkAsFailed(ctx, delivery.ID, "Device not connected")
			}
		// Implementar otros canales como FCM, APNS, etc.
		default:
			s.deliveryRepo.MarkAsFailed(ctx, delivery.ID, "Unsupported channel")
		}
	}

	return nil
}

// ConfirmDelivery confirma la entrega de una notificación a un dispositivo
func (s *NotificationService) ConfirmDelivery(ctx context.Context, notificationID, deviceID uuid.UUID) error {
	// Buscar el registro de entrega
	deliveries, err := s.deliveryRepo.GetByNotificationID(ctx, notificationID)
	if err != nil {
		return err
	}

	for _, delivery := range deliveries {
		if delivery.DeviceID == deviceID {
			return s.deliveryRepo.MarkAsDelivered(ctx, delivery.ID)
		}
	}

	return errors.New("delivery record not found")
}

// GetUserNotifications obtiene las notificaciones de un usuario
func (s *NotificationService) GetUserNotifications(ctx context.Context, userID string, limit, offset int) ([]*entity.Notification, error) {
	return s.notificationRepo.GetByUserID(ctx, userID, limit, offset)
}

// CountUnreadNotifications cuenta las notificaciones no leídas de un usuario
func (s *NotificationService) CountUnreadNotifications(ctx context.Context, userID string) (int, error) {
	return s.notificationRepo.CountUnreadByUser(ctx, userID)
}

// Añadir estos dos nuevos métodos a la implementación de NotificationService

// SaveNotification guarda una notificación en el repositorio
func (s *NotificationService) SaveNotification(ctx context.Context, notification *entity.Notification) error {
	// Guardar en repositorio
	if err := s.notificationRepo.Save(ctx, notification); err != nil {
		return ErrFailedToSaveNotification
	}

	return nil
}

// SendNotificationToDevices envía una notificación a dispositivos específicos de un usuario
func (s *NotificationService) SendNotificationToDevices(
	ctx context.Context,
	userID string,
	deviceIDs []uuid.UUID,
	title, message string,
	data map[string]interface{},
	notificationType entity.NotificationType,
	priority int,
	channels []string,
) (string, error) {
	// Crear notificación
	notification, err := entity.NewNotification(userID, title, message, data, notificationType)
	if err != nil {
		return "", ErrInvalidNotificationData
	}

	// Establecer prioridad si se especificó
	if priority > 0 {
		notification.SetPriority(priority)
	}

	// Guardar en repositorio
	if err := s.notificationRepo.Save(ctx, notification); err != nil {
		return "", ErrFailedToSaveNotification
	}

	// Si no se proporcionaron deviceIDs, error
	if len(deviceIDs) == 0 {
		return notification.ID.String(), errors.New("no devices specified")
	}

	// Determinar qué canales usar
	useWebSocket := true
	useFCM := true
	useAPNS := true

	if len(channels) > 0 {
		// Reiniciar flags si se especificaron canales
		useWebSocket = false
		useFCM = false
		useAPNS = false

		// Establecer flags según los canales especificados
		for _, channel := range channels {
			switch channel {
			case "websocket":
				useWebSocket = true
			case "fcm":
				useFCM = true
			case "apns":
				useAPNS = true
			}
		}
	}

	// Intentar enviar a cada dispositivo por los canales apropiados
	var deliveryErrors []error
	deliveredToAny := false

	for _, deviceID := range deviceIDs {
		// Verificar si el dispositivo existe
		_, err := s.deviceRepo.GetByID(ctx, deviceID)
		if err != nil {
			s.logger.Warn("Device not found: %s", deviceID)
			continue
		}

		// Intentar WebSocket primero si está habilitado
		if useWebSocket && s.wsManager.IsDeviceConnected(deviceID) {
			// Crear registro de entrega
			delivery := entity.NewDeliveryTracking(notification.ID, deviceID, entity.TokenTypeWebSocket)
			if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
				deliveryErrors = append(deliveryErrors, err)
				continue
			}

			// Preparar payload
			payload, err := s.prepareNotificationPayload(notification)
			if err != nil {
				deliveryErrors = append(deliveryErrors, err)
				s.deliveryRepo.MarkAsFailed(ctx, delivery.ID, "Failed to prepare payload")
				continue
			}

			// Enviar por WebSocket
			if err := s.wsManager.SendMessage(deviceID, payload); err != nil {
				deliveryErrors = append(deliveryErrors, err)
				s.deliveryRepo.MarkAsFailed(ctx, delivery.ID, err.Error())
				continue
			}

			// Marcar como enviado
			s.deliveryRepo.MarkAsSent(ctx, delivery.ID)
			deliveredToAny = true
			continue
		}

		// Intentar FCM si está habilitado
		if useFCM {
			// Obtener token FCM
			fcmToken, err := s.tokenRepo.GetByDeviceAndType(ctx, deviceID, entity.TokenTypeFCM)
			if err == nil && fcmToken != nil {
				// Crear registro de entrega
				delivery := entity.NewDeliveryTracking(notification.ID, deviceID, entity.TokenTypeFCM)
				if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
					deliveryErrors = append(deliveryErrors, err)
				} else {
					// Aquí se implementaría el envío FCM
					// Como no tenemos el servicio FCM implementado, marcamos como pendiente
					s.logger.Info("FCM delivery would be attempted here for device %s", deviceID)

					// En una implementación real, aquí llamaríamos al servicio FCM
					// y marcaríamos como enviado o fallido según corresponda

					deliveredToAny = true
				}
			}
		}

		// Intentar APNS si está habilitado
		if useAPNS {
			// Obtener token APNS
			apnsToken, err := s.tokenRepo.GetByDeviceAndType(ctx, deviceID, entity.TokenTypeAPNS)
			if err == nil && apnsToken != nil {
				// Crear registro de entrega
				delivery := entity.NewDeliveryTracking(notification.ID, deviceID, entity.TokenTypeAPNS)
				if err := s.deliveryRepo.Create(ctx, delivery); err != nil {
					deliveryErrors = append(deliveryErrors, err)
				} else {
					// Aquí se implementaría el envío APNS
					// Como no tenemos el servicio APNS implementado, marcamos como pendiente
					s.logger.Info("APNS delivery would be attempted here for device %s", deviceID)

					// En una implementación real, aquí llamaríamos al servicio APNS
					// y marcaríamos como enviado o fallido según corresponda

					deliveredToAny = true
				}
			}
		}
	}

	if !deliveredToAny && len(deliveryErrors) > 0 {
		return notification.ID.String(), ErrDeliveryFailed
	}

	return notification.ID.String(), nil
}
