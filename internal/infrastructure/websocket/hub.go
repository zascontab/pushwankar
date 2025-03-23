package websocket

import (
	"sync"

	"github.com/google/uuid"
)

// Hub mantiene el seguimiento de todas las conexiones activas
type Hub struct {
	// Clientes registrados
	clients map[*Client]bool

	// Mapeo de deviceID a clientes
	deviceClients map[uuid.UUID]map[*Client]bool

	// Mapeo de userID a clientes
	userClients map[string]map[*Client]bool

	// Canal para registrar nuevos clientes
	register chan *Client

	// Canal para dar de baja clientes
	unregister chan *Client

	// Canal para enviar mensajes a todos los clientes
	broadcast chan []byte

	// Canal para cerrar el hub
	shutdown chan struct{}

	// Mutex para deviceClients
	deviceMutex sync.RWMutex

	// Mutex para userClients
	userMutex sync.RWMutex
}

// NewHub crea un nuevo hub
func NewHub() *Hub {
	return &Hub{
		clients:       make(map[*Client]bool),
		deviceClients: make(map[uuid.UUID]map[*Client]bool),
		userClients:   make(map[string]map[*Client]bool),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		broadcast:     make(chan []byte),
		shutdown:      make(chan struct{}),
	}
}

// Run inicia el Hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			// Enviar mensajes a todos los clientes conectados
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					h.unregisterClient(client)
				}
			}

		case <-h.shutdown:
			// Cerrar todas las conexiones
			for client := range h.clients {
				h.unregisterClient(client)
				close(client.send)
			}
			return
		}
	}
}

// registerClient registra un cliente en el hub
func (h *Hub) registerClient(client *Client) {
	// Registrar en el mapa general de clientes
	h.clients[client] = true

	// Registrar por deviceID
	h.deviceMutex.Lock()
	if _, ok := h.deviceClients[client.deviceID]; !ok {
		h.deviceClients[client.deviceID] = make(map[*Client]bool)
	}
	h.deviceClients[client.deviceID][client] = true
	h.deviceMutex.Unlock()

	// Registrar por userID si está disponible
	if client.userID != "" {
		h.userMutex.Lock()
		if _, ok := h.userClients[client.userID]; !ok {
			h.userClients[client.userID] = make(map[*Client]bool)
		}
		h.userClients[client.userID][client] = true
		h.userMutex.Unlock()
	}
}

// unregisterClient elimina un cliente del hub
// unregisterClient elimina un cliente del hub
func (h *Hub) unregisterClient(client *Client) {
	// Eliminar del mapa general de clientes
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
	}

	// Eliminar del mapa deviceClients
	h.deviceMutex.Lock()
	if clients, ok := h.deviceClients[client.deviceID]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.deviceClients, client.deviceID)
		}
	}
	h.deviceMutex.Unlock()

	// Eliminar del mapa userClients
	if client.userID != "" {
		h.userMutex.Lock()
		if clients, ok := h.userClients[client.userID]; ok {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.userClients, client.userID)
			}
		}
		h.userMutex.Unlock()
	}

	// Cerrar el canal del cliente
	close(client.send)
}

// SendToDevice envía un mensaje a todos los clientes de un dispositivo específico
// SendToDevice envía un mensaje a todos los clientes de un dispositivo específico
func (h *Hub) SendToDevice(deviceID uuid.UUID, message []byte) bool {
	h.deviceMutex.RLock()
	clients, exists := h.deviceClients[deviceID]
	h.deviceMutex.RUnlock()

	if !exists || len(clients) == 0 {
		return false
	}

	sentToAny := false
	for client := range clients {
		select {
		case client.send <- message:
			sentToAny = true
		default:
			// Si el buffer está lleno, desregistramos el cliente
			// Usando una función anónima con go para enviar al canal
			go func(c *Client) {
				h.unregister <- c
			}(client)
		}
	}

	return sentToAny
}

// SendToUser envía un mensaje a todos los clientes de un usuario específico
// SendToUser envía un mensaje a todos los clientes de un usuario específico
func (h *Hub) SendToUser(userID string, message []byte) bool {
	h.userMutex.RLock()
	clients, exists := h.userClients[userID]
	h.userMutex.RUnlock()

	if !exists || len(clients) == 0 {
		return false
	}

	sentToAny := false
	for client := range clients {
		select {
		case client.send <- message:
			sentToAny = true
		default:
			// Si el buffer está lleno, desregistramos el cliente
			go func(c *Client) {
				h.unregister <- c
			}(client)
		}
	}

	return sentToAny
}

// BroadcastAll envía un mensaje a todos los clientes conectados
func (h *Hub) BroadcastAll(message []byte) {
	h.broadcast <- message
}

// IsDeviceConnected verifica si un dispositivo tiene alguna conexión activa
func (h *Hub) IsDeviceConnected(deviceID uuid.UUID) bool {
	h.deviceMutex.RLock()
	clients, exists := h.deviceClients[deviceID]
	h.deviceMutex.RUnlock()

	return exists && len(clients) > 0
}

// IsUserConnected verifica si un usuario tiene alguna conexión activa
func (h *Hub) IsUserConnected(userID string) bool {
	h.userMutex.RLock()
	clients, exists := h.userClients[userID]
	h.userMutex.RUnlock()

	return exists && len(clients) > 0
}

// GetConnectedDevices devuelve una lista de todos los dispositivos conectados
func (h *Hub) GetConnectedDevices() []uuid.UUID {
	h.deviceMutex.RLock()
	defer h.deviceMutex.RUnlock()

	devices := make([]uuid.UUID, 0, len(h.deviceClients))
	for deviceID := range h.deviceClients {
		devices = append(devices, deviceID)
	}

	return devices
}

// GetConnectedUsers devuelve una lista de todos los usuarios conectados
func (h *Hub) GetConnectedUsers() []string {
	h.userMutex.RLock()
	defer h.userMutex.RUnlock()

	users := make([]string, 0, len(h.userClients))
	for userID := range h.userClients {
		users = append(users, userID)
	}

	return users
}

// GetClientCount devuelve el número total de clientes conectados
func (h *Hub) GetClientCount() int {
	return len(h.clients)
}

// Shutdown cierra el hub y todas las conexiones
func (h *Hub) Shutdown() {
	close(h.shutdown)
}
