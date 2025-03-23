package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"notification-service/internal/domain/entity"
	"notification-service/internal/usecase"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// NotificationHandler maneja las peticiones HTTP relacionadas con notificaciones
type NotificationHandler struct {
	notificationService *usecase.NotificationService
}

// NewNotificationHandler crea un nuevo NotificationHandler
func NewNotificationHandler(notificationService *usecase.NotificationService) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
	}
}

// SendNotification envía una notificación a un usuario
func (h *NotificationHandler) SendNotification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID           string                 `json:"user_id"`
		Title            string                 `json:"title"`
		Message          string                 `json:"message"`
		Data             map[string]interface{} `json:"data"`
		NotificationType string                 `json:"notification_type"`
	}

	// Decodificar el cuerpo de la petición
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validar campos requeridos
	if req.UserID == "" || req.Title == "" || req.Message == "" {
		respondWithError(w, http.StatusBadRequest, "user_id, title and message are required")
		return
	}

	// Determinar el tipo de notificación
	notificationType := entity.NotificationTypeNormal
	if req.NotificationType != "" {
		switch req.NotificationType {
		case "urgent":
			notificationType = entity.NotificationTypeUrgent
		case "system":
			notificationType = entity.NotificationTypeSystem
		case "message":
			notificationType = entity.NotificationTypeMessage
		}
	}

	// Enviar notificación
	notificationID, err := h.notificationService.SendNotification(
		r.Context(),
		req.UserID,
		req.Title,
		req.Message,
		req.Data,
		notificationType,
	)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Responder con éxito
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"notification_id": notificationID,
		"status":          "success",
	})
}

// GetNotification obtiene detalles de una notificación
func (h *NotificationHandler) GetNotification(w http.ResponseWriter, r *http.Request) {
	// Obtener ID de la notificación de la URL
	vars := mux.Vars(r)
	notificationIDStr := vars["id"]

	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid notification ID")
		return
	}

	// Obtener la notificación
	notification, err := h.notificationService.GetNotification(r.Context(), notificationID)
	if err != nil {
		if err == usecase.ErrNotificationNotFound {
			respondWithError(w, http.StatusNotFound, "Notification not found")
		} else {
			respondWithError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Obtener el estado de entrega
	deliveryStatus, err := h.notificationService.GetDeliveryStatus(r.Context(), notificationID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Construir respuesta
	type DeliveryInfo struct {
		DeviceID    string `json:"device_id"`
		Status      string `json:"status"`
		SentAt      int64  `json:"sent_at,omitempty"`
		DeliveredAt int64  `json:"delivered_at,omitempty"`
		FailedAt    int64  `json:"failed_at,omitempty"`
		RetryCount  int    `json:"retry_count"`
	}

	deliveryInfo := make([]DeliveryInfo, 0, len(deliveryStatus))
	for _, delivery := range deliveryStatus {
		info := DeliveryInfo{
			DeviceID:   delivery.DeviceID.String(),
			Status:     string(delivery.Status),
			RetryCount: delivery.RetryCount,
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

		deliveryInfo = append(deliveryInfo, info)
	}

	// Convertir datos a map[string]interface{}
	dataMap, _ := notification.GetDataMap()

	// Responder con la notificación y su estado de entrega
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"id":         notification.ID.String(),
		"user_id":    notification.UserID,
		"title":      notification.Title,
		"message":    notification.Message,
		"data":       dataMap,
		"type":       notification.NotificationType,
		"created_at": notification.CreatedAt.Unix(),
		"deliveries": deliveryInfo,
	})
}

// GetUserNotifications obtiene las notificaciones de un usuario
func (h *NotificationHandler) GetUserNotifications(w http.ResponseWriter, r *http.Request) {
	// Obtener ID del usuario de la URL
	vars := mux.Vars(r)
	userID := vars["user_id"]

	// Parámetros de paginación
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20 // valor por defecto
	offset := 0 // valor por defecto

	// Convertir limit y offset a enteros
	if limitStr != "" {
		if i, err := parseInt(limitStr); err == nil && i > 0 {
			limit = i
		}
	}

	if offsetStr != "" {
		if i, err := parseInt(offsetStr); err == nil && i >= 0 {
			offset = i
		}
	}

	// Obtener notificaciones
	notifications, err := h.notificationService.GetUserNotifications(r.Context(), userID, limit, offset)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Contar notificaciones no leídas
	unreadCount, err := h.notificationService.CountUnreadNotifications(r.Context(), userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Preparar respuesta
	result := make([]map[string]interface{}, 0, len(notifications))
	for _, notification := range notifications {
		dataMap, _ := notification.GetDataMap()

		notificationData := map[string]interface{}{
			"id":         notification.ID.String(),
			"title":      notification.Title,
			"message":    notification.Message,
			"data":       dataMap,
			"type":       notification.NotificationType,
			"created_at": notification.CreatedAt.Unix(),
		}

		result = append(result, notificationData)
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"notifications": result,
		"unread_count":  unreadCount,
		"total":         len(notifications),
		"limit":         limit,
		"offset":        offset,
	})
}

// ConfirmDelivery confirma la entrega de una notificación
func (h *NotificationHandler) ConfirmDelivery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NotificationID string `json:"notification_id"`
		DeviceID       string `json:"device_id"`
	}

	// Decodificar el cuerpo de la petición
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validar campos requeridos
	if req.NotificationID == "" || req.DeviceID == "" {
		respondWithError(w, http.StatusBadRequest, "notification_id and device_id are required")
		return
	}

	// Convertir strings a UUID
	notificationID, err := uuid.Parse(req.NotificationID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid notification ID")
		return
	}

	deviceID, err := uuid.Parse(req.DeviceID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	// Confirmar entrega
	err = h.notificationService.ConfirmDelivery(r.Context(), notificationID, deviceID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"status": "success",
	})
}

func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// GetDeliveryStatus obtiene el estado de entrega de una notificación
func (h *NotificationHandler) GetDeliveryStatus(w http.ResponseWriter, r *http.Request) {
	// Obtener notificationID de los parámetros de consulta
	notificationIDStr := r.URL.Query().Get("notification_id")
	if notificationIDStr == "" {
		respondWithError(w, http.StatusBadRequest, "notification_id is required")
		return
	}

	// Convertir a UUID
	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid notification_id")
		return
	}

	// Obtener estado de entrega
	deliveries, err := h.notificationService.GetDeliveryStatus(r.Context(), notificationID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Preparar respuesta
	type DeliveryInfo struct {
		DeviceID     string `json:"device_id"`
		Status       string `json:"status"`
		SentAt       int64  `json:"sent_at,omitempty"`
		DeliveredAt  int64  `json:"delivered_at,omitempty"`
		FailedAt     int64  `json:"failed_at,omitempty"`
		RetryCount   int    `json:"retry_count"`
		ErrorMessage string `json:"error_message,omitempty"`
	}

	deliveryInfos := make([]DeliveryInfo, 0, len(deliveries))
	for _, delivery := range deliveries {
		info := DeliveryInfo{
			DeviceID:     delivery.DeviceID.String(),
			Status:       string(delivery.Status),
			RetryCount:   delivery.RetryCount,
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

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"notification_id": notificationID.String(),
		"deliveries":      deliveryInfos,
	})
}

// SendHybridNotification envía una notificación a través de múltiples canales (websocket, fcm, apns)
func (h *NotificationHandler) SendHybridNotification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID           string                 `json:"user_id"`
		DeviceIDs        []string               `json:"device_ids,omitempty"` // Opcional, si solo se desea enviar a dispositivos específicos
		Title            string                 `json:"title"`
		Message          string                 `json:"message"`
		Data             map[string]interface{} `json:"data,omitempty"`
		NotificationType string                 `json:"notification_type,omitempty"` // normal, urgent, system, message
		Priority         int                    `json:"priority,omitempty"`          // 0=normal, 1=alta
		Channels         []string               `json:"channels,omitempty"`          // websocket, fcm, apns - si no se especifica, usa todos
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validar campos obligatorios
	if req.UserID == "" || req.Title == "" || req.Message == "" {
		respondWithError(w, http.StatusBadRequest, "user_id, title and message are required")
		return
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

	// Crear una lista de UUIDs de dispositivos si se proporcionaron
	var deviceIDs []uuid.UUID
	if len(req.DeviceIDs) > 0 {
		for _, idStr := range req.DeviceIDs {
			id, err := uuid.Parse(idStr)
			if err != nil {
				respondWithError(w, http.StatusBadRequest, "Invalid device ID: "+idStr)
				return
			}
			deviceIDs = append(deviceIDs, id)
		}
	}

	// Enviar notificación
	var notificationID string
	var err error

	// Si se especificaron dispositivos, enviar solo a esos
	if len(deviceIDs) > 0 {
		notificationID, err = h.notificationService.SendNotificationToDevices(
			r.Context(),
			req.UserID,
			deviceIDs,
			req.Title,
			req.Message,
			req.Data,
			notificationType,
			req.Priority,
			req.Channels,
		)
	} else {
		// Enviar a todos los dispositivos del usuario
		notificationID, err = h.notificationService.SendNotification(
			r.Context(),
			req.UserID,
			req.Title,
			req.Message,
			req.Data,
			notificationType,
		)
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"notification_id": notificationID,
		"status":          "success",
	})
}
