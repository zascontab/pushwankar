package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"notification-service/internal/usecase"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Configurar según las necesidades
	},
}

// ConnectionHandlerImpl implementa ConnectionHandler
type ConnectionHandlerImpl struct {
	tokenService    *usecase.TokenService
	deviceService   *usecase.DeviceService
	deliveryService *usecase.DeliveryService
}

// OnConnect se llama cuando un cliente se conecta
func (h *ConnectionHandlerImpl) OnConnect(client *Client) {
	// Actualizar último acceso del dispositivo
	if client.deviceID != uuid.Nil {
		go h.deviceService.UpdateDeviceLastAccess(context.Background(), client.deviceID)
	}
}

// OnDisconnect se llama cuando un cliente se desconecta
func (h *ConnectionHandlerImpl) OnDisconnect(client *Client) {
	// No necesitamos hacer nada especial al desconectar
}

// OnMessage se llama cuando se recibe un mensaje de un cliente
func (h *ConnectionHandlerImpl) OnMessage(client *Client, messageType int, message []byte) {
	if messageType != websocket.TextMessage {
		return
	}

	// Actualizar último acceso
	client.UpdateLastActivity()

	// Procesar el mensaje
	var clientMsg ClientMessage
	if err := json.Unmarshal(message, &clientMsg); err != nil {
		h.OnError(client, err)
		return
	}

	// Manejar según el tipo de mensaje
	switch clientMsg.Type {
	case "ping":
		// Simplemente responder con un pong
		pongMsg := map[string]string{"type": "pong", "timestamp": time.Now().Format(time.RFC3339)}
		pongJSON, _ := json.Marshal(pongMsg)
		client.Send(pongJSON)

	case "ack":
		// Procesar confirmación de entrega
		var ackData struct {
			NotificationID string `json:"notification_id"`
		}
		if err := json.Unmarshal(clientMsg.Payload, &ackData); err != nil {
			h.OnError(client, err)
			return
		}

		if ackData.NotificationID != "" {
			notificationID, err := uuid.Parse(ackData.NotificationID)
			if err == nil {
				go h.deliveryService.ConfirmDelivery(context.Background(), notificationID, client.deviceID)
			}
		}

	case "token_refresh":
		// Manejar renovación de token
		var tokenData struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(clientMsg.Payload, &tokenData); err != nil {
			h.OnError(client, err)
			return
		}

		claims, err := h.tokenService.VerifyToken(context.Background(), tokenData.Token)
		if err != nil {
			// Si el token expiró, generar uno nuevo
			if errors.Is(err, usecase.ErrTokenExpired) {
				deviceIdentifier, extractErr := h.tokenService.ExtractDeviceIdentifierFromToken(tokenData.Token)
				if extractErr == nil && deviceIdentifier == client.deviceIdentifier {
					// Generar nuevo token temporal o permanente según corresponda
					var newToken string
					if claims != nil && claims.IsTemporary {
						newToken, _ = h.tokenService.GenerateTemporaryToken(client.deviceIdentifier)
					} else if client.userID != "" {
						newToken, _ = h.tokenService.GeneratePermanentToken(client.userID, client.deviceID)
					}

					if newToken != "" {
						response := map[string]interface{}{
							"type":    "token_refresh_response",
							"token":   newToken,
							"success": true,
						}
						respJSON, _ := json.Marshal(response)
						client.Send(respJSON)

						// Actualizar el token del cliente
						client.token = newToken
					}
				}
			}
		}
	}
}

// OnError se llama cuando ocurre un error en la conexión
func (h *ConnectionHandlerImpl) OnError(client *Client, err error) {
	// Podríamos registrar el error
}

// WebSocketManager implementa el gestor de websockets
type WebSocketManager struct {
	hub               *Hub
	tokenService      *usecase.TokenService
	deviceService     *usecase.DeviceService
	deliveryService   *usecase.DeliveryService
	connectionHandler ConnectionHandler
}

// NewWebSocketManager crea un nuevo WebSocketManager
func NewWebSocketManager(
	tokenService *usecase.TokenService,
	deviceService *usecase.DeviceService,
	deliveryService *usecase.DeliveryService,

) *WebSocketManager {
	hub := NewHub()
	connectionHandler := &ConnectionHandlerImpl{
		tokenService:    tokenService,
		deviceService:   deviceService,
		deliveryService: deliveryService,
	}

	return &WebSocketManager{
		hub:               hub,
		tokenService:      tokenService,
		deviceService:     deviceService,
		deliveryService:   deliveryService,
		connectionHandler: connectionHandler,
	}
}

// Start inicia el WebSocketManager
func (m *WebSocketManager) Start() {
	go m.hub.Run()
}

// HandleConnection maneja una nueva conexión WebSocket
func (m *WebSocketManager) HandleConnection(w http.ResponseWriter, r *http.Request) {
	// Obtener token
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	// Verificar token
	claims, er := m.tokenService.VerifyToken(r.Context(), token)
	if er != nil {
		// Si el token está expirado pero tenemos deviceIdentifier, generar uno nuevo
		if errors.Is(er, usecase.ErrTokenExpired) {
			deviceIdentifier, extractErr := m.tokenService.ExtractDeviceIdentifierFromToken(token)
			if extractErr == nil {
				newToken, genErr := m.tokenService.GenerateTemporaryToken(deviceIdentifier)
				if genErr == nil {
					// Enviar el nuevo token en la respuesta
					w.Header().Set("X-New-Token", newToken)
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Access-Control-Expose-Headers", "X-New-Token")
					w.WriteHeader(http.StatusUnauthorized)

					response := map[string]interface{}{
						"error":     "token_expired",
						"message":   "Please reconnect with the new token",
						"new_token": newToken,
					}
					json.NewEncoder(w).Encode(response)
					return
				}
			}
		}

		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	var deviceID uuid.UUID
	var err error

	// Obtener deviceID del token
	if claims.DeviceID != "" {
		deviceID, err = uuid.Parse(claims.DeviceID)
		if err != nil {
			http.Error(w, "Invalid device ID", http.StatusBadRequest)
			return
		}
	}

	// Actualizar dispositivo
	// En una implementación real, obtendríamos el dispositivo de la BD y lo actualizaríamos

	// Actualizar la conexión a WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// Crear nuevo cliente
	client := NewClient(
		m.hub,
		conn,
		claims.UserID,
		deviceID,
		claims.DeviceIdentifier,
		token,
		m.connectionHandler,
	)

	// Registrar cliente en el hub
	m.hub.register <- client

	// Iniciar el bombeo de mensajes
	go client.writePump()
	go client.readPump()
}

// SendToDevice envía un mensaje a un dispositivo
/* func (m *WebSocketManager) SendToDevice(deviceID uuid.UUID, payload []byte) error {
	if !m.hub.SendToDevice(deviceID, payload) {
		return errors.New("device not connected")
	}
	return nil
} */

// SendToUser envía un mensaje a un usuario
/* func (m *WebSocketManager) SendToUser(userID string, payload []byte) error {
	if !m.hub.SendToUser(userID, payload) {
		return errors.New("user not connected")
	}
	return nil
}

// IsDeviceConnected verifica si un dispositivo está conectado
func (m *WebSocketManager) IsDeviceConnected(deviceID uuid.UUID) bool {
	return m.hub.IsDeviceConnected(deviceID)
}

// IsUserConnected verifica si un usuario está conectado
func (m *WebSocketManager) IsUserConnected(userID string) bool {
	return m.hub.IsUserConnected(userID)
}

// GetConnectedDevices devuelve todos los dispositivos conectados
func (m *WebSocketManager) GetConnectedDevices() []uuid.UUID {
	return m.hub.GetConnectedDevices()
}

// GetConnectedUsers devuelve todos los usuarios conectados
func (m *WebSocketManager) GetConnectedUsers() []string {
	return m.hub.GetConnectedUsers()
} */

// Shutdown cierra el WebSocketManager
func (m *WebSocketManager) Shutdown() {
	m.hub.Shutdown()
}

// Adaptar el WebSocketManager para que implemente la interfaz usecase.WebSocketManager
// Añadir estos métodos a internal/infrastructure/websocket/server.go

// SendToDevice envía un mensaje a un dispositivo
// Este método satisface la interfaz usecase.WebSocketManager
func (m *WebSocketManager) SendToDevice(deviceID uuid.UUID, payload []byte) bool {
	return m.hub.SendToDevice(deviceID, payload)
}

// SendToUser envía un mensaje a un usuario
// Este método satisface la interfaz usecase.WebSocketManager
func (m *WebSocketManager) SendToUser(userID string, payload []byte) bool {
	return m.hub.SendToUser(userID, payload)
}

// GetConnectedDevices devuelve todos los dispositivos conectados
// Este método satisface la interfaz usecase.WebSocketManager
func (m *WebSocketManager) GetConnectedDevices() []uuid.UUID {
	return m.hub.GetConnectedDevices()
}

// IsDeviceConnected verifica si un dispositivo está conectado
// Este método satisface la interfaz usecase.WebSocketManager
func (m *WebSocketManager) IsDeviceConnected(deviceID uuid.UUID) bool {
	return m.hub.IsDeviceConnected(deviceID)
}

func (m *WebSocketManager) SendMessage(deviceID uuid.UUID, payload []byte) error {
	success := m.hub.SendToDevice(deviceID, payload)
	if !success {
		return fmt.Errorf("failed to send message to device %s", deviceID)
	}
	return nil
}
