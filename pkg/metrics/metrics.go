package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Métricas de notificaciones
	NotificationsSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notifications_sent_total",
			Help: "Total number of notifications sent",
		},
		[]string{"channel", "type", "status"},
	)

	NotificationsDelivered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notifications_delivered_total",
			Help: "Total number of notifications confirmed as delivered",
		},
		[]string{"channel", "type", "status"},
	)

	NotificationsFailed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notifications_failed_total",
			Help: "Total number of notifications that failed to deliver",
		},
		[]string{"channel", "type", "error_type"},
	)

	NotificationsRetried = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notifications_retried_total",
			Help: "Total number of notification delivery retries",
		},
		[]string{"channel", "type"},
	)

	// Histogramas para medir latencia
	NotificationLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "notification_latency_seconds",
			Help:    "Time taken to send notification",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // From 1ms to ~1s
		},
		[]string{"channel", "type"},
	)

	NotificationEndToEndLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "notification_e2e_latency_seconds",
			Help:    "Time taken from notification request to delivery confirmation",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 12), // From 10ms to ~40s
		},
		[]string{"channel", "type"},
	)

	// Métricas de WebSocket
	WebSocketConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "websocket_connections_active",
			Help: "Current number of active WebSocket connections",
		},
	)

	WebSocketMessagesReceived = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "websocket_messages_received_total",
			Help: "Total number of WebSocket messages received",
		},
	)

	WebSocketMessagesSent = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "websocket_messages_sent_total",
			Help: "Total number of WebSocket messages sent",
		},
	)

	WebSocketErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "websocket_errors_total",
			Help: "Total number of WebSocket errors",
		},
		[]string{"type"},
	)

	// Métricas de tokens
	TokensGenerated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tokens_generated_total",
			Help: "Total number of tokens generated",
		},
		[]string{"type"},
	)

	TokensVerified = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tokens_verified_total",
			Help: "Total number of token verifications",
		},
		[]string{"result"},
	)

	TokensRevoked = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tokens_revoked_total",
			Help: "Total number of tokens revoked",
		},
	)

	// Métricas de rate-limiting (throttling)
	RateLimiterBlocked = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limiter_blocked_total",
			Help: "Total number of requests blocked by rate limiter",
		},
		[]string{"type", "id"},
	)

	RateLimiterAllowed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limiter_allowed_total",
			Help: "Total number of requests allowed by rate limiter",
		},
		[]string{"type", "id"},
	)

	// Métricas de sistema
	ProcessingErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "processing_errors_total",
			Help: "Total number of errors during message processing",
		},
		[]string{"component", "type"},
	)

	// Métricas de caché
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_type"},
	)

	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_type"},
	)

	// Métricas de base de datos
	DBOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_operations_total",
			Help: "Total number of database operations",
		},
		[]string{"operation", "repository"},
	)

	DBOperationLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_operation_latency_seconds",
			Help:    "Latency of database operations",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"operation", "repository"},
	)

	// Métricas del ciclo de vida de la aplicación
	StartupTime = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "app_startup_timestamp_seconds",
			Help: "Timestamp when the application started",
		},
	)

	Uptime = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "app_uptime_seconds",
			Help: "Time elapsed since the application started",
		},
	)

	// Métricas específicas para canales externos
	FCMRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fcm_requests_total",
			Help: "Total number of requests sent to FCM",
		},
		[]string{"status"},
	)

	APNSRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "apns_requests_total",
			Help: "Total number of requests sent to APNS",
		},
		[]string{"status"},
	)

	ExternalAPILatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "external_api_latency_seconds",
			Help:    "Latency of requests to external APIs",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
		},
		[]string{"service"},
	)
)

// RecordLatency registra el tiempo transcurrido desde el inicio
func RecordLatency(histogram *prometheus.HistogramVec, labels prometheus.Labels, start time.Time) {
	duration := time.Since(start).Seconds()
	histogram.With(labels).Observe(duration)
}

// SetupMetrics inicializa las métricas cuando se inicia la aplicación
func SetupMetrics() {
	// Registrar tiempo de inicio
	StartupTime.Set(float64(time.Now().Unix()))

	// Iniciar goroutine para actualizar el tiempo de actividad
	go updateUptime()
}

// updateUptime actualiza periódicamente la métrica de tiempo de actividad
func updateUptime() {
	startTime := time.Now()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		uptime := time.Since(startTime).Seconds()
		Uptime.Set(uptime)
	}
}

// DBMetricsMiddleware es un middleware para medir operaciones de base de datos
type DBMetricsMiddleware struct {
	Repository string
}

// RecordOperation registra una operación de base de datos
func (m *DBMetricsMiddleware) RecordOperation(operation string, f func() error) error {
	start := time.Now()
	err := f()
	duration := time.Since(start).Seconds()

	labels := prometheus.Labels{"operation": operation, "repository": m.Repository}
	DBOperations.With(labels).Inc()
	DBOperationLatency.With(labels).Observe(duration)

	return err
}

// CacheMetricsMiddleware es un middleware para medir operaciones de caché
type CacheMetricsMiddleware struct {
	CacheType string
}

// RecordHit registra un acierto en la caché
func (m *CacheMetricsMiddleware) RecordHit() {
	CacheHits.WithLabelValues(m.CacheType).Inc()
}

// RecordMiss registra un fallo en la caché
func (m *CacheMetricsMiddleware) RecordMiss() {
	CacheMisses.WithLabelValues(m.CacheType).Inc()
}
