package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"notification-service/internal/domain/entity"
	"notification-service/pkg/logging"
)

const (
	fcmSendURL = "https://fcm.googleapis.com/fcm/send"
	fcmTimeout = 10 * time.Second
)

// FCMAdapter es un adaptador para enviar notificaciones a través de Firebase Cloud Messaging
type FCMAdapter struct {
	apiKey     string
	httpClient *http.Client
	logger     logging.Logger
}

// FCMNotification representa una notificación FCM
type FCMNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Sound string `json:"sound,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

// FCMPayload representa el payload de una notificación FCM
type FCMPayload struct {
	To           string                 `json:"to"`
	Notification FCMNotification        `json:"notification"`
	Data         map[string]interface{} `json:"data,omitempty"`
	Priority     string                 `json:"priority,omitempty"` // "normal" o "high"
	TimeToLive   int                    `json:"time_to_live,omitempty"`
}

// FCMResponse representa la respuesta de FCM
type FCMResponse struct {
	MulticastID  int64    `json:"multicast_id"`
	Success      int      `json:"success"`
	Failure      int      `json:"failure"`
	CanonicalIDs int      `json:"canonical_ids"`
	Results      []Result `json:"results"`
}

// Result representa el resultado de enviar un mensaje a FCM
type Result struct {
	MessageID      string `json:"message_id"`
	RegistrationID string `json:"registration_id"`
	Error          string `json:"error"`
}

// NewFCMAdapter crea una nueva instancia de FCMAdapter
func NewFCMAdapter(apiKey string, logger logging.Logger) *FCMAdapter {
	return &FCMAdapter{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: fcmTimeout,
		},
		logger: logger,
	}
}

// Send envía una notificación a través de FCM
func (a *FCMAdapter) Send(ctx context.Context, token string, notification *entity.Notification) (string, error) {
	// Convertir los datos de notification.Data a un mapa
	var dataMap map[string]interface{}
	if notification.Data != nil && len(notification.Data) > 0 {
		if err := json.Unmarshal(notification.Data, &dataMap); err != nil {
			return "", fmt.Errorf("error unmarshalling notification data: %w", err)
		}
	}

	// Si no hay un mapa de datos, crear uno vacío
	if dataMap == nil {
		dataMap = make(map[string]interface{})
	}

	// Añadir notification_id a los datos para rastreo
	dataMap["notification_id"] = notification.ID.String()

	// Determinar la prioridad FCM basada en la prioridad de la notificación
	priority := "normal"
	if notification.Priority > 0 {
		priority = "high"
	}

	// Crear el payload de FCM
	payload := FCMPayload{
		To: token,
		Notification: FCMNotification{
			Title: notification.Title,
			Body:  notification.Message,
			Sound: "default",
		},
		Data:     dataMap,
		Priority: priority,
	}

	// Si la notificación tiene fecha de expiración, establecer TimeToLive
	if notification.ExpiresAt != nil {
		// Calcular tiempo a vivir en segundos desde ahora hasta expiración
		ttl := int(notification.ExpiresAt.Sub(time.Now()).Seconds())
		if ttl > 0 {
			payload.TimeToLive = ttl
		}
	}

	// Convertir el payload a JSON
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshalling FCM payload: %w", err)
	}

	// Crear la solicitud HTTP
	req, err := http.NewRequestWithContext(ctx, "POST", fcmSendURL, bytes.NewBuffer(payloadJSON))
	if err != nil {
		return "", fmt.Errorf("error creating FCM request: %w", err)
	}

	// Establecer las cabeceras
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "key="+a.apiKey)

	// Enviar la solicitud
	resp, err := a.httpClient.Do(req)
	if err != nil {
		a.logger.Error("Error sending FCM notification", "error", err, "token", token)
		return "", fmt.Errorf("error sending FCM request: %w", err)
	}
	defer resp.Body.Close()

	// Comprobar el código de estado
	if resp.StatusCode != http.StatusOK {
		a.logger.Error("FCM server returned error", "status", resp.StatusCode, "token", token)
		return "", fmt.Errorf("FCM server returned error, status: %d", resp.StatusCode)
	}

	// Decodificar la respuesta
	var fcmResponse FCMResponse
	if err := json.NewDecoder(resp.Body).Decode(&fcmResponse); err != nil {
		a.logger.Error("Error decoding FCM response", "error", err, "token", token)
		return "", fmt.Errorf("error decoding FCM response: %w", err)
	}

	// Comprobar si hubo éxito
	if fcmResponse.Success == 0 {
		if len(fcmResponse.Results) > 0 && fcmResponse.Results[0].Error != "" {
			a.logger.Error("FCM notification failed", "error", fcmResponse.Results[0].Error, "token", token)
			return "", fmt.Errorf("FCM notification failed: %s", fcmResponse.Results[0].Error)
		}
		return "", errors.New("FCM notification failed without specific error")
	}

	// Obtener el ID del mensaje
	messageID := ""
	if len(fcmResponse.Results) > 0 && fcmResponse.Results[0].MessageID != "" {
		messageID = fcmResponse.Results[0].MessageID
		a.logger.Info("FCM notification sent", "messageID", messageID, "token", token)
	}

	// Comprobar si el token ha cambiado
	if len(fcmResponse.Results) > 0 && fcmResponse.Results[0].RegistrationID != "" {
		a.logger.Info("FCM token has changed",
			"oldToken", token,
			"newToken", fcmResponse.Results[0].RegistrationID,
		)
		// Aquí se podría implementar una función para actualizar el token en la base de datos
	}

	return messageID, nil
}
