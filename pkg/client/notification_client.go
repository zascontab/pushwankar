package client

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "notification-service/pkg/proto"
)

// NotificationClient encapsula la comunicación con el servicio de notificaciones
type NotificationClient struct {
	conn   *grpc.ClientConn
	client pb.NotificationServiceClient
}

// NotificationRequest representa una solicitud para enviar una notificación
type NotificationRequest struct {
	UserID           string            // ID del usuario
	Title            string            // Título de la notificación
	Message          string            // Mensaje de la notificación
	Data             map[string]string // Datos adicionales (opcional)
	NotificationType string            // Tipo: normal, urgent, system, message
	SenderID         string            // ID del remitente (opcional)
	Priority         int               // Prioridad: 0-normal, 1-alta
	Expiry           int64             // Tiempo de expiración en segundos (opcional)
}

// DeviceRegistrationRequest representa una solicitud para registrar un dispositivo
type DeviceRegistrationRequest struct {
	DeviceIdentifier string // Identificador único del dispositivo
	DeviceModel      string // Modelo del dispositivo (opcional)
	UserID           string // ID del usuario (opcional)
}

// DeviceLinkRequest representa una solicitud para vincular un dispositivo a un usuario
type DeviceLinkRequest struct {
	DeviceID string // ID del dispositivo
	UserID   string // ID del usuario
	Token    string // Token temporal para autorización
}

// TokenUpdateRequest representa una solicitud para actualizar un token de notificación
type TokenUpdateRequest struct {
	DeviceID  string // ID del dispositivo
	Token     string // Valor del token
	TokenType string // Tipo de token: websocket, apns, fcm
}

// DeliveryStatusResponse representa el estado de entrega de una notificación
type DeliveryStatusResponse struct {
	NotificationID string         // ID de la notificación
	Deliveries     []DeliveryInfo // Información de entregas
	Success        bool           // Indica si la operación fue exitosa
	ErrorMessage   string         // Mensaje de error (si hay)
}

// DeliveryInfo representa la información de entrega para un dispositivo
type DeliveryInfo struct {
	DeviceID     string // ID del dispositivo
	Status       string // Estado: pending, sent, delivered, failed
	SentAt       int64  // Timestamp de envío
	DeliveredAt  int64  // Timestamp de entrega
	FailedAt     int64  // Timestamp de fallo
	RetryCount   int    // Número de reintentos
	ErrorMessage string // Mensaje de error (si hay)
}

// NewNotificationClient crea un nuevo cliente para el servicio de notificaciones
func NewNotificationClient(serverAddress string) (*NotificationClient, error) {
	// Configurar opciones de conexión
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	}

	// Establecer conexión con timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, serverAddress, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to notification service: %w", err)
	}

	client := pb.NewNotificationServiceClient(conn)
	return &NotificationClient{
		conn:   conn,
		client: client,
	}, nil
}

// Close cierra la conexión con el servidor gRPC
func (c *NotificationClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// SendNotification envía una notificación push
func (c *NotificationClient) SendNotification(ctx context.Context, req *NotificationRequest) (string, error) {
	// Crear solicitud gRPC
	request := &pb.SendNotificationRequest{
		UserId:           req.UserID,
		Title:            req.Title,
		Message:          req.Message,
		NotificationType: req.NotificationType,
		SenderId:         req.SenderID,
		Priority:         int32(req.Priority),
		Expiry:           req.Expiry,
	}

	// Convertir Data a formato requerido
	if req.Data != nil {
		request.Data = make(map[string]string)
		for k, v := range req.Data {
			request.Data[k] = v
		}
	}

	// Configurar backoff para reintentos
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxElapsedTime = 10 * time.Second

	var response *pb.SendNotificationResponse
	var notificationID string

	// Intentar enviar con reintentos
	operation := func() error {
		// Crear contexto con timeout para cada intento
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var err error
		response, err = c.client.SendNotification(reqCtx, request)
		if err != nil {
			return fmt.Errorf("error calling SendNotification: %w", err)
		}

		if !response.Success {
			return fmt.Errorf("notification delivery failed: %s", response.ErrorMessage)
		}

		notificationID = response.NotificationId
		return nil
	}

	// Ejecutar con reintentos
	err := backoff.Retry(operation, expBackoff)
	if err != nil {
		return "", err
	}

	return notificationID, nil
}

// RegisterDevice registra un dispositivo en el servicio de notificaciones
func (c *NotificationClient) RegisterDevice(ctx context.Context, req *DeviceRegistrationRequest) (string, string, error) {
	// Crear solicitud gRPC
	request := &pb.RegisterDeviceRequest{
		DeviceIdentifier: req.DeviceIdentifier,
		DeviceModel:      req.DeviceModel,
		UserId:           req.UserID,
	}

	// Configurar backoff para reintentos
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxElapsedTime = 10 * time.Second

	var response *pb.RegisterDeviceResponse
	var deviceID, token string

	// Intentar registrar con reintentos
	operation := func() error {
		// Crear contexto con timeout para cada intento
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var err error
		response, err = c.client.RegisterDevice(reqCtx, request)
		if err != nil {
			return fmt.Errorf("error calling RegisterDevice: %w", err)
		}

		if !response.Success {
			return fmt.Errorf("device registration failed: %s", response.ErrorMessage)
		}

		deviceID = response.DeviceId
		token = response.Token
		return nil
	}

	// Ejecutar con reintentos
	err := backoff.Retry(operation, expBackoff)
	if err != nil {
		return "", "", err
	}

	return deviceID, token, nil
}

// LinkDeviceToUser vincula un dispositivo a un usuario
func (c *NotificationClient) LinkDeviceToUser(ctx context.Context, req *DeviceLinkRequest) (string, error) {
	// Crear solicitud gRPC
	request := &pb.LinkDeviceToUserRequest{
		DeviceId: req.DeviceID,
		UserId:   req.UserID,
		Token:    req.Token,
	}

	// Configurar backoff para reintentos
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxElapsedTime = 10 * time.Second

	var response *pb.LinkDeviceToUserResponse
	var newToken string

	// Intentar vincular con reintentos
	operation := func() error {
		// Crear contexto con timeout para cada intento
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var err error
		response, err = c.client.LinkDeviceToUser(reqCtx, request)
		if err != nil {
			return fmt.Errorf("error calling LinkDeviceToUser: %w", err)
		}

		if !response.Success {
			return fmt.Errorf("device linking failed: %s", response.ErrorMessage)
		}

		newToken = response.NewToken
		return nil
	}

	// Ejecutar con reintentos
	err := backoff.Retry(operation, expBackoff)
	if err != nil {
		return "", err
	}

	return newToken, nil
}

// UpdateDeviceToken actualiza un token de notificación para un dispositivo
func (c *NotificationClient) UpdateDeviceToken(ctx context.Context, req *TokenUpdateRequest) error {
	// Crear solicitud gRPC
	request := &pb.UpdateDeviceTokenRequest{
		DeviceId:  req.DeviceID,
		Token:     req.Token,
		TokenType: req.TokenType,
	}

	// Configurar backoff para reintentos
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxElapsedTime = 10 * time.Second

	// Intentar actualizar con reintentos
	operation := func() error {
		// Crear contexto con timeout para cada intento
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		response, err := c.client.UpdateDeviceToken(reqCtx, request)
		if err != nil {
			return fmt.Errorf("error calling UpdateDeviceToken: %w", err)
		}

		if !response.Success {
			return fmt.Errorf("token update failed: %s", response.ErrorMessage)
		}

		return nil
	}

	// Ejecutar con reintentos
	return backoff.Retry(operation, expBackoff)
}

// VerifyDeviceToken verifica si un token es válido
func (c *NotificationClient) VerifyDeviceToken(ctx context.Context, token string) (bool, string, string, error) {
	// Crear solicitud gRPC
	request := &pb.VerifyDeviceTokenRequest{
		Token: token,
	}

	// Configurar backoff para reintentos
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxElapsedTime = 10 * time.Second

	var response *pb.VerifyDeviceTokenResponse
	var isValid bool
	var deviceID, userID string

	// Intentar verificar con reintentos
	operation := func() error {
		// Crear contexto con timeout para cada intento
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var err error
		response, err = c.client.VerifyDeviceToken(reqCtx, request)
		if err != nil {
			return fmt.Errorf("error calling VerifyDeviceToken: %w", err)
		}

		isValid = response.IsValid
		deviceID = response.DeviceId
		userID = response.UserId
		return nil
	}

	// Ejecutar con reintentos
	err := backoff.Retry(operation, expBackoff)
	if err != nil {
		return false, "", "", err
	}

	if !isValid {
		return false, "", "", fmt.Errorf("invalid token: %s", response.ErrorMessage)
	}

	return isValid, deviceID, userID, nil
}

// GetDeliveryStatus obtiene el estado de entrega de una notificación
func (c *NotificationClient) GetDeliveryStatus(ctx context.Context, notificationID string) (*DeliveryStatusResponse, error) {
	// Crear solicitud gRPC
	request := &pb.GetDeliveryStatusRequest{
		NotificationId: notificationID,
	}

	// Configurar backoff para reintentos
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxElapsedTime = 10 * time.Second

	var response *pb.GetDeliveryStatusResponse

	// Intentar obtener estado con reintentos
	operation := func() error {
		// Crear contexto con timeout para cada intento
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var err error
		response, err = c.client.GetDeliveryStatus(reqCtx, request)
		if err != nil {
			return fmt.Errorf("error calling GetDeliveryStatus: %w", err)
		}

		if !response.Success {
			return fmt.Errorf("get delivery status failed: %s", response.ErrorMessage)
		}

		return nil
	}

	// Ejecutar con reintentos
	err := backoff.Retry(operation, expBackoff)
	if err != nil {
		return nil, err
	}

	// Convertir la respuesta al formato requerido
	result := &DeliveryStatusResponse{
		NotificationID: response.NotificationId,
		Success:        response.Success,
		ErrorMessage:   response.ErrorMessage,
		Deliveries:     make([]DeliveryInfo, 0, len(response.Deliveries)),
	}

	for _, delivery := range response.Deliveries {
		info := DeliveryInfo{
			DeviceID:     delivery.DeviceId,
			Status:       delivery.Status,
			SentAt:       delivery.SentAt,
			DeliveredAt:  delivery.DeliveredAt,
			FailedAt:     delivery.FailedAt,
			RetryCount:   int(delivery.RetryCount),
			ErrorMessage: delivery.ErrorMessage,
		}
		result.Deliveries = append(result.Deliveries, info)
	}

	return result, nil
}
