package websocket

import (
	"sync"
	"time"

	"notification-service/pkg/logging"
)

// ConnectionCleaner se encarga de limpiar las conexiones WebSocket inactivas
type ConnectionCleaner struct {
	hub            *Hub
	interval       time.Duration
	inactivityTime time.Duration
	stopCh         chan struct{}
	wg             sync.WaitGroup
	logger         *logging.Logger
	clientsMutex   sync.RWMutex // Mutex adicional para acceder al mapa clients
}

// NewConnectionCleaner crea una nueva instancia de ConnectionCleaner
func NewConnectionCleaner(hub *Hub, interval, inactivityTime time.Duration, logger *logging.Logger) *ConnectionCleaner {
	return &ConnectionCleaner{
		hub:            hub,
		interval:       interval,
		inactivityTime: inactivityTime,
		stopCh:         make(chan struct{}),
		logger:         logger,
	}
}

// Start inicia el proceso de limpieza de conexiones
func (c *ConnectionCleaner) Start() {
	c.wg.Add(1)
	go c.run()
}

// Stop detiene el proceso de limpieza de conexiones
func (c *ConnectionCleaner) Stop() {
	close(c.stopCh)
	c.wg.Wait()
}

// run ejecuta el proceso de limpieza periódicamente
func (c *ConnectionCleaner) run() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.cleanInactiveConnections()
		}
	}
}

// cleanInactiveConnections limpia las conexiones inactivas
func (c *ConnectionCleaner) cleanInactiveConnections() {
	now := time.Now()
	inactiveThreshold := now.Add(-c.inactivityTime)

	// Crear lista temporal para no modificar el mapa mientras lo recorremos
	var inactiveClients []*Client

	// Usar una copia segura del mapa de clientes para no bloquear el hub
	c.clientsMutex.Lock()
	clientsCopy := make([]*Client, 0, len(c.hub.clients))
	for client := range c.hub.clients {
		clientsCopy = append(clientsCopy, client)
	}
	c.clientsMutex.Unlock()

	// Verificar inactividad sin bloquear el mapa completo
	for _, client := range clientsCopy {
		if client.GetLastActivity().Before(inactiveThreshold) {
			inactiveClients = append(inactiveClients, client)
		}
	}

	// Cerrar las conexiones inactivas
	for _, client := range inactiveClients {
		c.logger.Info("Closing inactive WebSocket connection: DeviceID=%s, UserID=%s, LastActivity=%s",
			client.deviceID, client.userID, client.GetLastActivity().Format(time.RFC3339))

		// Usar el canal de unregister para que el hub maneje correctamente la desconexión
		c.hub.unregister <- client
	}

	if len(inactiveClients) > 0 {
		c.logger.Info("Cleaned %d inactive WebSocket connections", len(inactiveClients))
	}
}

// También se necesitan estos métodos en el cliente para que funcione el connection cleaner:

/* // GetLastActivity devuelve la última vez que el cliente estuvo activo
func (c *Client) GetLastActivity() time.Time {
	return c.lastActivity
}

// UpdateLastActivity actualiza la última actividad del cliente
func (c *Client) UpdateLastActivity() {
	c.lastActivity = time.Now()
}
*/
