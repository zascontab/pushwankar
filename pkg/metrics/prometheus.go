package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Métricas HTTP
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total de solicitudes HTTP procesadas",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duración de las solicitudes HTTP en segundos",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Métricas de WebSocket
	websocketConnectionsTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "websocket_connections_total",
			Help: "Número actual de conexiones WebSocket activas",
		},
	)

	websocketMessagesSent = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "websocket_messages_sent_total",
			Help: "Total de mensajes WebSocket enviados",
		},
	)

	websocketMessagesReceived = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "websocket_messages_received_total",
			Help: "Total de mensajes WebSocket recibidos",
		},
	)

	// Métricas de notificaciones
	notificationSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_sent_total",
			Help: "Total de notificaciones enviadas",
		},
		[]string{"type"},
	)

	notificationDeliveryTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_delivery_total",
			Help: "Resultados de entrega de notificaciones",
		},
		[]string{"status", "channel"},
	)

	notificationQueueSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "notification_queue_size",
			Help: "Número de notificaciones en la cola de procesamiento",
		},
	)

	notificationRetryTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "notification_retry_total",
			Help: "Total de reintentos de envío de notificaciones",
		},
	)

	// Métricas de tokens
	tokenOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "token_operations_total",
			Help: "Operaciones de gestión de tokens",
		},
		[]string{"operation", "result"},
	)

	// Métricas de base de datos
	dbOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_operations_total",
			Help: "Total de operaciones de base de datos",
		},
		[]string{"operation", "table", "result"},
	)

	dbOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_operation_duration_seconds",
			Help:    "Duración de las operaciones de base de datos en segundos",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "table"},
	)
)

// HTTPMiddleware registra métricas para solicitudes HTTP
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Crear un ResponseWriter personalizado para capturar el código de estado
		rw := NewResponseWriter(w)

		// Llamar al siguiente handler
		next.ServeHTTP(rw, r)

		// Registrar duración
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.StatusCode)
		path := GetNormalizedPath(r.URL.Path)

		httpRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

// ResponseWriter personalizado para capturar el código de estado
type ResponseWriter struct {
	http.ResponseWriter
	StatusCode int
}

// NewResponseWriter crea un nuevo ResponseWriter
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{w, http.StatusOK}
}

// WriteHeader sobrescribe el método original para capturar el código de estado
func (rw *ResponseWriter) WriteHeader(code int) {
	rw.StatusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// GetNormalizedPath normaliza la ruta para evitar cardinalidad alta en las métricas
func GetNormalizedPath(path string) string {
	// Aquí puedes implementar lógica para normalizar rutas con IDs dinámicos
	// Por ejemplo, /users/123 -> /users/:id
	return path
}

// WebSocketConnectionsChange actualiza el contador de conexiones WebSocket
func WebSocketConnectionsChange(delta int) {
	websocketConnectionsTotal.Add(float64(delta))
}

// WebSocketMessageSent incrementa el contador de mensajes WebSocket enviados
func WebSocketMessageSent() {
	websocketMessagesSent.Inc()
}

// WebSocketMessageReceived incrementa el contador de mensajes WebSocket recibidos
func WebSocketMessageReceived() {
	websocketMessagesReceived.Inc()
}

// NotificationSent registra una notificación enviada
func NotificationSent(notificationType string) {
	notificationSentTotal.WithLabelValues(notificationType).Inc()
}

// NotificationDelivery registra un resultado de entrega de notificación
func NotificationDelivery(status, channel string) {
	notificationDeliveryTotal.WithLabelValues(status, channel).Inc()
}

// SetNotificationQueueSize actualiza el tamaño de la cola de notificaciones
func SetNotificationQueueSize(size int) {
	notificationQueueSize.Set(float64(size))
}

// NotificationRetry registra un reintento de envío de notificación
func NotificationRetry() {
	notificationRetryTotal.Inc()
}

// TokenOperation registra una operación sobre tokens
func TokenOperation(operation, result string) {
	tokenOperationsTotal.WithLabelValues(operation, result).Inc()
}

// ObserveDatabaseOperation registra una operación de base de datos con su duración
func ObserveDatabaseOperation(operation, table, result string, duration time.Duration) {
	dbOperationsTotal.WithLabelValues(operation, table, result).Inc()
	dbOperationDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

// TrackDatabaseOperation mide y registra la duración de una operación de base de datos
func TrackDatabaseOperation(operation, table string) func(result string) {
	start := time.Now()
	return func(result string) {
		duration := time.Since(start)
		ObserveDatabaseOperation(operation, table, result, duration)
	}
}
