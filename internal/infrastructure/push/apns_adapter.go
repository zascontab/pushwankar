package push

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"notification-service/internal/domain/entity"
	"notification-service/pkg/logging"
)

const (
	// URLs para desarrollo y producción
	apnsProductionHost  = "https://api.push.apple.com/3/device"
	apnsDevelopmentHost = "https://api.development.push.apple.com/3/device"
	apnsTimeout         = 10 * time.Second
)

// APNSAdapter es un adaptador para enviar notificaciones a través de Apple Push Notification Service
type APNSAdapter struct {
	certificate  tls.Certificate
	bundleID     string
	isProduction bool
	httpClient   *http.Client
	logger       logging.Logger
}

// APNSPayload representa el payload de una notificación APNS
type APNSPayload struct {
	Aps    APSPayload             `json:"aps"`
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// APSPayload representa el contenido del campo 'aps' en una notificación APNS
type APSPayload struct {
	Alert            APSAlert `json:"alert"`
	Badge            int      `json:"badge,omitempty"`
	Sound            string   `json:"sound,omitempty"`
	ContentAvailable int      `json:"content-available,omitempty"`
	MutableContent   int      `json:"mutable-content,omitempty"`
	Category         string   `json:"category,omitempty"`
	ThreadID         string   `json:"thread-id,omitempty"`
}

// APSAlert representa el contenido del campo 'alert' en una notificación APNS
type APSAlert struct {
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	TitleLocKey  string   `json:"title-loc-key,omitempty"`
	TitleLocArgs []string `json:"title-loc-args,omitempty"`
	ActionLocKey string   `json:"action-loc-key,omitempty"`
	LocKey       string   `json:"loc-key,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	LaunchImage  string   `json:"launch-image,omitempty"`
}

// NewAPNSAdapter crea una nueva instancia de APNSAdapter
func NewAPNSAdapter(
	certificatePath string,
	certificatePassword string,
	bundleID string,
	isProduction bool,
	logger logging.Logger,
) (*APNSAdapter, error) {
	// Cargar el certificado de cliente
	cert, err := tls.LoadX509KeyPair(certificatePath, certificatePath)
	if err != nil {
		return nil, fmt.Errorf("error loading APNS certificate: %w", err)
	}

	// Configurar el cliente HTTP con TLS personalizado
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   apnsTimeout,
	}

	return &APNSAdapter{
		certificate:  cert,
		bundleID:     bundleID,
		isProduction: isProduction,
		httpClient:   httpClient,
		logger:       logger,
	}, nil
}

// Send envía una notificación a través de APNS
func (a *APNSAdapter) Send(ctx context.Context, token string, notification *entity.Notification) (string, error) {
	// Determinar el host APNS según el entorno
	host := apnsDevelopmentHost
	if a.isProduction {
		host = apnsProductionHost
	}

	// Construir la URL completa
	url := fmt.Sprintf("%s/%s", host, token)

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

	// Crear el payload de APNS
	payload := APNSPayload{
		Aps: APSPayload{
			Alert: APSAlert{
				Title: notification.Title,
				Body:  notification.Message,
			},
			Sound: "default",
			Badge: 1,
		},
		Custom: dataMap,
	}

	// Para notificaciones silenciosas o de alta prioridad
	if notification.NotificationType == entity.NotificationTypeSystem {
		payload.Aps.ContentAvailable = 1
	}

	// Convertir el payload a JSON
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshalling APNS payload: %w", err)
	}

	// Crear la solicitud HTTP
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payloadJSON))
	if err != nil {
		return "", fmt.Errorf("error creating APNS request: %w", err)
	}

	// Establecer las cabeceras
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apns-topic", a.bundleID)

	// Establecer el ID de la notificación como identificador único
	apnsID := notification.ID.String()
	req.Header.Set("apns-id", apnsID)

	// Establecer la prioridad según el tipo de notificación
	apnsPriority := "5" // Prioridad normal
	if notification.Priority > 0 {
		apnsPriority = "10" // Prioridad alta
	}
	req.Header.Set("apns-priority", apnsPriority)

	// Establecer tiempo de expiración si existe
	if notification.ExpiresAt != nil {
		expiryTime := notification.ExpiresAt.Unix()
		req.Header.Set("apns-expiration", fmt.Sprintf("%d", expiryTime))
	}

	// Enviar la solicitud
	resp, err := a.httpClient.Do(req)
	if err != nil {
		a.logger.Error("Error sending APNS notification", "error", err, "token", token)
		return "", fmt.Errorf("error sending APNS request: %w", err)
	}
	defer resp.Body.Close()

	// Verificar el código de respuesta
	if resp.StatusCode != http.StatusOK {
		var errorResponse struct {
			Reason string `json:"reason"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
			a.logger.Error("Error decoding APNS error response", "status", resp.StatusCode, "error", err)
			return "", fmt.Errorf("APNS server error: status=%d", resp.StatusCode)
		}

		a.logger.Error("APNS notification failed",
			"status", resp.StatusCode,
			"reason", errorResponse.Reason,
			"token", token,
		)

		// Manejo específico de errores
		if errorResponse.Reason == "BadDeviceToken" || errorResponse.Reason == "Unregistered" {
			// Aquí se podría marcar el token como inválido en la base de datos
			return "", fmt.Errorf("invalid device token: %s", errorResponse.Reason)
		}

		return "", fmt.Errorf("APNS notification failed: %s", errorResponse.Reason)
	}

	// Obtener el ID del mensaje desde las cabeceras
	apnsID = resp.Header.Get("apns-id")
	a.logger.Info("APNS notification sent successfully", "apnsID", apnsID, "token", token)

	return apnsID, nil
}
