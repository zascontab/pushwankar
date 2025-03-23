package events

import (
	"encoding/json"
	"sync"
	"time"

	"notification-service/pkg/logging"

	"github.com/google/uuid"
)

// EventType representa el tipo de evento
type EventType string

const (
	// Eventos de conexión
	EventClientConnected    EventType = "client.connected"
	EventClientDisconnected EventType = "client.disconnected"

	// Eventos de notificación
	EventNotificationSent      EventType = "notification.sent"
	EventNotificationDelivered EventType = "notification.delivered"
	EventNotificationFailed    EventType = "notification.failed"
	EventNotificationRetrying  EventType = "notification.retrying"

	// Eventos de sistema
	EventSystemStarted  EventType = "system.started"
	EventSystemStopping EventType = "system.stopping"
	EventSystemError    EventType = "system.error"

	// Eventos de throttling
	EventThrottlingApplied EventType = "throttling.applied"
)

// Event representa un evento del sistema
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// EventHandler es una función que maneja eventos
type EventHandler func(event Event)

// EventManager gestiona los eventos del sistema
type EventManager struct {
	handlers     map[EventType][]EventHandler
	mu           sync.RWMutex
	logger       *logging.Logger
	enabledTypes map[EventType]bool
}

// NewEventManager crea una nueva instancia de EventManager
func NewEventManager(logger *logging.Logger) *EventManager {
	return &EventManager{
		handlers:     make(map[EventType][]EventHandler),
		enabledTypes: make(map[EventType]bool),
		logger:       logger,
	}
}

// Subscribe registra un manejador para un tipo de evento específico
func (m *EventManager) Subscribe(eventType EventType, handler EventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.handlers[eventType]; !exists {
		m.handlers[eventType] = []EventHandler{}
	}
	m.handlers[eventType] = append(m.handlers[eventType], handler)
}

// Unsubscribe elimina un manejador para un tipo de evento específico
func (m *EventManager) Unsubscribe(eventType EventType, handler EventHandler) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if handlers, exists := m.handlers[eventType]; exists {
		for i, h := range handlers {
			// Comparar referencias de función (esto es aproximado, ya que Go no garantiza la comparación de funciones)
			if &h == &handler {
				// Eliminar el handler usando técnica de reordenamiento
				m.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
				return true
			}
		}
	}
	return false
}

// EmitEvent emite un evento
func (m *EventManager) EmitEvent(eventType EventType, data map[string]interface{}) {
	// Verificar si el tipo de evento está habilitado
	m.mu.RLock()
	enabled, exists := m.enabledTypes[eventType]
	m.mu.RUnlock()

	if exists && !enabled {
		return
	}

	event := Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}

	// Notificar a todos los manejadores suscritos
	m.notify(event)

	// Registrar en el log según el tipo de evento
	m.logEvent(event)
}

// EnableEventType habilita un tipo de evento
func (m *EventManager) EnableEventType(eventType EventType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabledTypes[eventType] = true
}

// DisableEventType deshabilita un tipo de evento
func (m *EventManager) DisableEventType(eventType EventType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabledTypes[eventType] = false
}

// notify notifica a todos los manejadores suscritos
func (m *EventManager) notify(event Event) {
	m.mu.RLock()
	handlers, exists := m.handlers[event.Type]
	m.mu.RUnlock()

	if exists {
		// Llamar a los manejadores en goroutines separadas para no bloquear
		for _, handler := range handlers {
			go handler(event)
		}
	}
}

// logEvent registra el evento en el log
func (m *EventManager) logEvent(event Event) {
	// Convertir datos a JSON para el log
	dataJSON, _ := json.Marshal(event.Data)

	switch event.Type {
	case EventSystemError:
		m.logger.Error("Event: %s, Data: %s", event.Type, string(dataJSON))
	case EventNotificationFailed:
		m.logger.Warn("Event: %s, Data: %s", event.Type, string(dataJSON))
	default:
		m.logger.Info("Event: %s, Data: %s", event.Type, string(dataJSON))
	}
}

// NewMetricsEventHandler crea un manejador de eventos para métricas
func NewMetricsEventHandler() EventHandler {
	// Aquí podríamos integrar con Prometheus, StatsD, etc.
	return func(event Event) {
		// Por ejemplo, para Prometheus:
		// según el tipo de evento, incrementar contadores, actualizar histogramas, etc.
		switch event.Type {
		case EventNotificationSent:
			// prometheus.NotificationsSentCounter.Inc()
		case EventNotificationDelivered:
			// prometheus.NotificationsDeliveredCounter.Inc()
		case EventNotificationFailed:
			// prometheus.NotificationsFailedCounter.Inc()
		case EventClientConnected:
			// prometheus.ConnectedClientsGauge.Inc()
		case EventClientDisconnected:
			// prometheus.ConnectedClientsGauge.Dec()
		}
	}
}

// NewAlertEventHandler crea un manejador de eventos para alertas
func NewAlertEventHandler(logger *logging.Logger) EventHandler {
	return func(event Event) {
		// Aquí se podría integrar con sistemas de alerta como PagerDuty, Slack, etc.
		switch event.Type {
		case EventSystemError:
			logger.Error("ALERT - System error: %v", event.Data["error"])
			// Enviar alerta a equipo de operaciones
		case EventNotificationFailed:
			if count, ok := event.Data["retry_count"].(int); ok && count >= 3 {
				logger.Warn("ALERT - Notification failed after %d retries: %v", count, event.Data["error"])
				// Enviar alerta a equipo de soporte
			}
		case EventThrottlingApplied:
			logger.Warn("ALERT - Rate limiting applied: %v", event.Data)
			// Enviar alerta de posible abuso
		}
	}
}

// EventStorage es una interfaz para almacenar eventos
type EventStorage interface {
	Store(event Event) error
	GetByType(eventType EventType, limit, offset int) ([]Event, error)
	GetByTimeRange(start, end time.Time, limit, offset int) ([]Event, error)
}

// InMemoryEventStorage implementa EventStorage en memoria
type InMemoryEventStorage struct {
	events []Event
	mu     sync.RWMutex
	maxLen int
}

// NewInMemoryEventStorage crea un nuevo InMemoryEventStorage
func NewInMemoryEventStorage(maxLen int) *InMemoryEventStorage {
	return &InMemoryEventStorage{
		events: make([]Event, 0, maxLen),
		maxLen: maxLen,
	}
}

// Store almacena un evento
func (s *InMemoryEventStorage) Store(event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Si alcanzamos el tamaño máximo, eliminar el evento más antiguo
	if len(s.events) >= s.maxLen {
		s.events = s.events[1:]
	}

	// Agregar el nuevo evento
	s.events = append(s.events, event)
	return nil
}

// GetByType obtiene eventos por tipo
func (s *InMemoryEventStorage) GetByType(eventType EventType, limit, offset int) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Event
	count := 0

	// Recorrer los eventos en orden inverso (más recientes primero)
	for i := len(s.events) - 1; i >= 0; i-- {
		if s.events[i].Type == eventType {
			if count >= offset {
				result = append(result, s.events[i])
			}
			count++
			if len(result) >= limit {
				break
			}
		}
	}

	return result, nil
}

// GetByTimeRange obtiene eventos por rango de tiempo
func (s *InMemoryEventStorage) GetByTimeRange(start, end time.Time, limit, offset int) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Event
	count := 0

	// Recorrer los eventos en orden inverso (más recientes primero)
	for i := len(s.events) - 1; i >= 0; i-- {
		event := s.events[i]
		if (event.Timestamp.Equal(start) || event.Timestamp.After(start)) &&
			(event.Timestamp.Equal(end) || event.Timestamp.Before(end)) {
			if count >= offset {
				result = append(result, event)
			}
			count++
			if len(result) >= limit {
				break
			}
		}
	}

	return result, nil
}
