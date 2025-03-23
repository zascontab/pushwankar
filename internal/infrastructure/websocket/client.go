package websocket

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Tiempo máximo para escribir un mensaje al cliente
	writeWait = 10 * time.Second

	// Tiempo máximo para leer el siguiente pong del cliente
	pongWait = 60 * time.Second

	// Enviar pings al cliente con esta periodicidad
	pingPeriod = (pongWait * 9) / 10

	// Tamaño máximo del mensaje
	maxMessageSize = 4096
)

// ClientMessage representa un mensaje del cliente
type ClientMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Client representa una conexión WebSocket con un cliente
type Client struct {
	hub               *Hub
	conn              *websocket.Conn
	send              chan []byte
	userID            string
	deviceID          uuid.UUID
	deviceIdentifier  string
	token             string
	lastActivity      time.Time
	connectionHandler ConnectionHandler
}

// ConnectionHandler define las operaciones para manejar eventos de conexión
type ConnectionHandler interface {
	OnConnect(client *Client)
	OnDisconnect(client *Client)
	OnMessage(client *Client, messageType int, message []byte)
	OnError(client *Client, err error)
}

// NewClient crea un nuevo cliente WebSocket
func NewClient(
	hub *Hub,
	conn *websocket.Conn,
	userID string,
	deviceID uuid.UUID,
	deviceIdentifier string,
	token string,
	handler ConnectionHandler,
) *Client {
	return &Client{
		hub:               hub,
		conn:              conn,
		send:              make(chan []byte, 256),
		userID:            userID,
		deviceID:          deviceID,
		deviceIdentifier:  deviceIdentifier,
		token:             token,
		lastActivity:      time.Now(),
		connectionHandler: handler,
	}
}

// readPump bombea mensajes desde la conexión WebSocket al hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
		if c.connectionHandler != nil {
			c.connectionHandler.OnDisconnect(c)
		}
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.lastActivity = time.Now()
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				if c.connectionHandler != nil {
					c.connectionHandler.OnError(c, err)
				}
			}
			break
		}

		c.lastActivity = time.Now()

		if c.connectionHandler != nil {
			c.connectionHandler.OnMessage(c, messageType, message)
		}
	}
}

// writePump bombea mensajes desde el hub a la conexión WebSocket
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	if c.connectionHandler != nil {
		c.connectionHandler.OnConnect(c)
	}

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// El hub cerró el canal
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Agregar todos los mensajes pendientes
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Send envía un mensaje al cliente
func (c *Client) Send(message []byte) bool {
	select {
	case c.send <- message:
		return true
	default:
		return false
	}
}

// Close cierra la conexión del cliente
func (c *Client) Close() {
	c.conn.Close()
}

// IsActive verifica si el cliente está activo
func (c *Client) IsActive() bool {
	return time.Since(c.lastActivity) <= pongWait
}

// GetLastActivity devuelve la última vez que el cliente estuvo activo
func (c *Client) GetLastActivity() time.Time {
	return c.lastActivity
}

// UpdateLastActivity actualiza la última actividad del cliente
func (c *Client) UpdateLastActivity() {
	c.lastActivity = time.Now()
}
