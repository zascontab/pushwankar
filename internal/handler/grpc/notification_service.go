package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"notification-service/internal/domain/entity"
	"notification-service/internal/usecase"
	"notification-service/pkg/logging"
	pb "notification-service/pkg/proto"
)

// NotificationServer implementa la interfaz gRPC NotificationService
type NotificationServer struct {
	pb.UnimplementedNotificationServiceServer
	notificationService *usecase.NotificationService
	deviceService       *usecase.DeviceService
	tokenService        *usecase.TokenService
	pushService         usecase.PushService
	logger              logging.Logger
}

// NewNotificationServer crea una nueva instancia del servidor gRPC
func NewNotificationServer(
	notificationService *usecase.NotificationService,
	deviceService *usecase.DeviceService,
	tokenService *usecase.TokenService,
	pushService usecase.PushService,
	logger logging.Logger,
) *NotificationServer {
	return &NotificationServer{
		notificationService: notificationService,
		deviceService:       deviceService,
		tokenService:        tokenService,
		pushService:         pushService,
		logger:              logger,
	}
}

// SendNotification implementa el método RPC SendNotification
func (s *NotificationServer) SendNotification(
	ctx context.Context,
	req *pb.SendNotificationRequest,
) (*pb.SendNotificationResponse, error) {
	// Validar la solicitud
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.Title == "" || req.Message == "" {
		return nil, status.Error(codes.InvalidArgument, "title and message are required")
	}

	// Determinar el tipo de notificación
	notificationType := entity.NotificationTypeNormal
	switch req.NotificationType {
	case "urgent":
		notificationType = entity.NotificationTypeUrgent
	case "system":
		notificationType = entity.NotificationTypeSystem
	case "message":
		notificationType = entity.NotificationTypeMessage
	}

	// Convertir los datos del mapa a un mapa real
	data := make(map[string]interface{})
	for k, v := range req.Data {
		data[k] = v
	}

	// Crear y guardar la notificación
	notification, err := entity.NewNotification(req.UserId, req.Title, req.Message, data, notificationType)
	if err != nil {
		s.logger.Error("Error creating notification", "error", err)
		return nil, status.Error(codes.Internal, "error creating notification")
	}

	// Establecer propiedades adicionales
	if req.Priority > 0 {
		notification.SetPriority(int(req.Priority))
	}
	if req.SenderId != "" {
		notification.SetSender(req.SenderId)
	}
	if req.Expiry > 0 {
		expiryTime := time.Duration(req.Expiry) * time.Second
		notification.SetExpiry(expiryTime)
	}

	// Guardar la notificación
	if err := s.notificationService.SaveNotification(ctx, notification); err != nil {
		s.logger.Error("Error saving notification", "error", err)
		return nil, status.Error(codes.Internal, "error saving notification")
	}

	// Enviar la notificación a todos los dispositivos del usuario
	results, errors := s.pushService.SendUserNotification(ctx, req.UserId, notification)

	// Preparar la respuesta
	response := &pb.SendNotificationResponse{
		NotificationId: notification.ID.String(),
		Success:        len(results) > 0,
	}

	// Si no se pudo entregar a ningún dispositivo, incluir mensaje de error
	if len(results) == 0 && len(errors) > 0 {
		var errorMsg string
		for _, err := range errors {
			errorMsg = err.Error()
			break
		}
		response.ErrorMessage = errorMsg
	}

	return response, nil
}

// VerifyDeviceToken implementa el método RPC VerifyDeviceToken
func (s *NotificationServer) VerifyDeviceToken(
	ctx context.Context,
	req *pb.VerifyDeviceTokenRequest,
) (*pb.VerifyDeviceTokenResponse, error) {
	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	// Verificar el token
	claims, err := s.tokenService.VerifyToken(ctx, req.Token)
	if err != nil {
		if errors.Is(err, usecase.ErrTokenExpired) {
			return &pb.VerifyDeviceTokenResponse{
				IsValid:      false,
				ErrorMessage: "token has expired",
			}, nil
		}
		return &pb.VerifyDeviceTokenResponse{
			IsValid:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	return &pb.VerifyDeviceTokenResponse{
		IsValid:     true,
		DeviceId:    claims.DeviceID,
		UserId:      claims.UserID,
		IsTemporary: claims.IsTemporary,
	}, nil
}

// RegisterDevice implementa el método RPC RegisterDevice
func (s *NotificationServer) RegisterDevice(
	ctx context.Context,
	req *pb.RegisterDeviceRequest,
) (*pb.RegisterDeviceResponse, error) {
	if req.DeviceIdentifier == "" {
		return nil, status.Error(codes.InvalidArgument, "device_identifier is required")
	}

	var err error
	var device *entity.Device

	// Registrar el dispositivo con o sin usuario
	if req.UserId != "" {
		// Convertir UserId a uint
		var userID uint
		fmt.Sscanf(req.UserId, "%d", &userID)

		// Registrar dispositivo con usuario
		device, err = s.deviceService.RegisterDeviceWithUser(
			ctx,
			req.DeviceIdentifier,
			userID,
			&req.DeviceModel,
		)
	} else {
		// Registrar dispositivo sin usuario
		device, err = s.deviceService.RegisterDeviceWithoutUser(
			ctx,
			req.DeviceIdentifier,
			&req.DeviceModel,
		)
	}

	if err != nil {
		s.logger.Error("Error registering device", "error", err)
		return &pb.RegisterDeviceResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	// Generar token según corresponda
	var token string
	if req.UserId != "" {
		// Convertir UserId a uint
		var userID uint
		fmt.Sscanf(req.UserId, "%d", &userID)

		// Generar token permanente
		token, err = s.tokenService.GeneratePermanentToken(req.UserId, device.ID)
	} else {
		// Generar token temporal
		token, err = s.tokenService.GenerateTemporaryToken(req.DeviceIdentifier)
	}

	if err != nil {
		s.logger.Error("Error generating token", "error", err)
		return &pb.RegisterDeviceResponse{
			DeviceId:     device.ID.String(),
			Success:      false,
			ErrorMessage: "device registered but token generation failed: " + err.Error(),
		}, nil
	}

	return &pb.RegisterDeviceResponse{
		DeviceId: device.ID.String(),
		Token:    token,
		Success:  true,
	}, nil
}

// LinkDeviceToUser implementa el método RPC LinkDeviceToUser
func (s *NotificationServer) LinkDeviceToUser(
	ctx context.Context,
	req *pb.LinkDeviceToUserRequest,
) (*pb.LinkDeviceToUserResponse, error) {
	if req.DeviceId == "" || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id and user_id are required")
	}

	// Verificar token temporal
	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	// Convertir DeviceId a UUID
	deviceID, err := uuid.Parse(req.DeviceId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid device_id format")
	}

	// Obtener el dispositivo para verificar su identificador
	device, err := s.deviceService.GetDevice(ctx, deviceID)
	if err != nil {
		s.logger.Error("Error getting device", "deviceID", deviceID, "error", err)
		return &pb.LinkDeviceToUserResponse{
			Success:      false,
			ErrorMessage: "error getting device: " + err.Error(),
		}, nil
	}

	// Verificar que el token sea válido para este dispositivo
	if !s.tokenService.VerifyTemporaryToken(req.Token, device.DeviceIdentifier) {
		return &pb.LinkDeviceToUserResponse{
			Success:      false,
			ErrorMessage: "invalid temporary token",
		}, nil
	}

	// Convertir UserID a uint
	var userID uint
	fmt.Sscanf(req.UserId, "%d", &userID)

	// Vincular dispositivo a usuario
	if err := s.deviceService.LinkDeviceToUser(ctx, deviceID, userID); err != nil {
		s.logger.Error("Error linking device to user", "error", err)
		return &pb.LinkDeviceToUserResponse{
			Success:      false,
			ErrorMessage: "error linking device to user: " + err.Error(),
		}, nil
	}

	// Generar nuevo token permanente
	newToken, err := s.tokenService.GeneratePermanentToken(req.UserId, deviceID)
	if err != nil {
		s.logger.Error("Error generating permanent token", "error", err)
		return &pb.LinkDeviceToUserResponse{
			Success:      true,
			ErrorMessage: "device linked but token generation failed: " + err.Error(),
		}, nil
	}

	return &pb.LinkDeviceToUserResponse{
		NewToken: newToken,
		Success:  true,
	}, nil
}

// UpdateDeviceToken implementa el método RPC UpdateDeviceToken
func (s *NotificationServer) UpdateDeviceToken(
	ctx context.Context,
	req *pb.UpdateDeviceTokenRequest,
) (*pb.UpdateDeviceTokenResponse, error) {
	if req.DeviceId == "" || req.Token == "" || req.TokenType == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id, token and token_type are required")
	}

	// Convertir DeviceId a UUID
	deviceID, err := uuid.Parse(req.DeviceId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid device_id format")
	}

	// Determinar el tipo de token
	var tokenType entity.TokenType
	switch req.TokenType {
	case "websocket":
		tokenType = entity.TokenTypeWebSocket
	case "apns":
		tokenType = entity.TokenTypeAPNS
	case "fcm":
		tokenType = entity.TokenTypeFCM
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid token_type")
	}

	// Actualizar o crear token
	err = s.tokenService.SaveToken(ctx, deviceID, req.Token, tokenType)
	if err != nil {
		s.logger.Error("Error saving token", "error", err)
		return &pb.UpdateDeviceTokenResponse{
			Success:      false,
			ErrorMessage: "error saving token: " + err.Error(),
		}, nil
	}

	return &pb.UpdateDeviceTokenResponse{
		Success: true,
	}, nil
}

// GetDeliveryStatus implementa el método RPC GetDeliveryStatus
func (s *NotificationServer) GetDeliveryStatus(
	ctx context.Context,
	req *pb.GetDeliveryStatusRequest,
) (*pb.GetDeliveryStatusResponse, error) {
	if req.NotificationId == "" {
		return nil, status.Error(codes.InvalidArgument, "notification_id is required")
	}

	// Convertir NotificationId a UUID
	notificationID, err := uuid.Parse(req.NotificationId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid notification_id format")
	}

	// Obtener el estado de entrega
	deliveries, err := s.notificationService.GetDeliveryStatus(ctx, notificationID)
	if err != nil {
		s.logger.Error("Error getting delivery status", "error", err)
		return &pb.GetDeliveryStatusResponse{
			NotificationId: req.NotificationId,
			Success:        false,
			ErrorMessage:   "error getting delivery status: " + err.Error(),
		}, nil
	}

	// Convertir a la estructura de la respuesta
	deliveryInfos := make([]*pb.DeliveryInfo, 0, len(deliveries))
	for _, delivery := range deliveries {
		info := &pb.DeliveryInfo{
			DeviceId:     delivery.DeviceID.String(),
			Status:       string(delivery.Status),
			RetryCount:   int32(delivery.RetryCount),
			ErrorMessage: delivery.ErrorMessage,
		}

		if delivery.SentAt != nil {
			info.SentAt = delivery.SentAt.Unix()
		}
		if delivery.DeliveredAt != nil {
			info.DeliveredAt = delivery.DeliveredAt.Unix()
		}
		if delivery.FailedAt != nil {
			info.FailedAt = delivery.FailedAt.Unix()
		}

		deliveryInfos = append(deliveryInfos, info)
	}

	return &pb.GetDeliveryStatusResponse{
		NotificationId: req.NotificationId,
		Deliveries:     deliveryInfos,
		Success:        true,
	}, nil
}

// StartGRPCServer inicia el servidor gRPC
func StartGRPCServer(port int, server *NotificationServer, logger logging.Logger) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterNotificationServiceServer(s, server)

	logger.Info("Starting gRPC server", "port", port)
	return s.Serve(lis)
}
