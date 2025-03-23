package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"notification-service/config"
	httpHandlers "notification-service/internal/handler/http"
	"notification-service/internal/infrastructure/client/business"
	"notification-service/internal/infrastructure/repository/postgres"
	"notification-service/internal/infrastructure/websocket"
	"notification-service/internal/usecase"
	"notification-service/pkg/logging"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Cargar configuración
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Configurar logger
	logger := logging.NewLogger(
		logging.WithLevel(logging.ParseLevel(cfg.Logging.Level)),
		logging.WithPrefix("notification-service"),
	)

	logger.Info("Starting notification service with configuration: %+v", cfg)

	// Conectar a la base de datos
	dbConn, err := sql.Open("postgres", cfg.Database.GetDatabaseDSN())
	if err != nil {
		logger.Fatal("Failed to connect to database: %v", err)
	}
	defer dbConn.Close()

	// Probar conexión
	if err := dbConn.Ping(); err != nil {
		logger.Fatal("Failed to ping database: %v", err)
	}
	logger.Info("Successfully connected to database")

	// Crear repositorios
	deviceRepo := postgres.NewDeviceRepository(dbConn)
	tokenRepo := postgres.NewTokenRepository(dbConn)
	notificationRepo := postgres.NewNotificationRepository(dbConn)
	deliveryRepo := postgres.NewDeliveryRepository(dbConn)

	// Crear cliente para comunicación con el servicio de negocio
	businessClient, err := business.NewBusinessClient(cfg.BusinessService.GRPCAddress)
	if err != nil {
		logger.Fatal("Failed to create business client: %v", err)
	}
	defer businessClient.Close()

	// Crear servicios de dominio
	tokenService := usecase.NewTokenService(
		tokenRepo,
		deviceRepo,
		cfg.JWT.Secret,
		cfg.JWT.TokenExpiry,
		cfg.JWT.TemporaryTokenExpiry,
	)

	deviceService := usecase.NewDeviceService(deviceRepo, tokenRepo)

	// Crear servicio de entrega
	deliveryService := usecase.NewDeliveryService(
		deliveryRepo,
		notificationRepo,
		deviceRepo,
		logger,
	)

	// Crear websocket manager
	wsManager := websocket.NewWebSocketManager(
		tokenService,
		deviceService,
		deliveryService,
	)

	// Iniciar el websocket manager
	wsManager.Start()

	// Ahora podemos crear el servicio de notificaciones
	notificationService := usecase.NewNotificationService(notificationRepo, deliveryRepo, deviceRepo, tokenRepo, wsManager, logger)

	// Crear handlers HTTP
	notificationHandler := httpHandlers.NewNotificationHandler(notificationService)
	deviceHandler := httpHandlers.NewDeviceHandler(deviceService, tokenService)
	healthHandler := httpHandlers.NewHealthHandler()

	// Crear router
	router := mux.NewRouter()

	// Rutas API (cambiando el prefijo para coincidir con el módulo original)
	apiRouter := router.PathPrefix("/api").Subrouter()

	// En tu archivo main.go, reemplaza la configuración actual del router por esta:

	// Rutas de notificaciones
	apiRouter.HandleFunc("/notifications/send", notificationHandler.SendNotification).Methods("POST")
	apiRouter.HandleFunc("/notifications/{id}", notificationHandler.GetNotification).Methods("GET")
	apiRouter.HandleFunc("/notifications/delivery-status", notificationHandler.GetDeliveryStatus).Methods("GET")
	apiRouter.HandleFunc("/notifications/confirm", notificationHandler.ConfirmDelivery).Methods("POST")
	apiRouter.HandleFunc("/notifications/send-hybrid", notificationHandler.SendHybridNotification).Methods("POST")

	// Rutas de usuarios y sus notificaciones
	apiRouter.HandleFunc("/users/{user_id}/notifications", notificationHandler.GetUserNotifications).Methods("GET")

	// Rutas de dispositivos
	apiRouter.HandleFunc("/devices/register", deviceHandler.RegisterDevice).Methods("POST")
	apiRouter.HandleFunc("/devices/register-without-user", deviceHandler.RegisterDeviceWithoutUser).Methods("POST")
	apiRouter.HandleFunc("/devices/{id}", deviceHandler.GetDevice).Methods("GET")
	apiRouter.HandleFunc("/devices/user", deviceHandler.GetUserDevices).Methods("GET")
	apiRouter.HandleFunc("/devices/link", deviceHandler.LinkDeviceToUser).Methods("POST")
	apiRouter.HandleFunc("/devices/update-token", deviceHandler.UpdateToken).Methods("POST")
	apiRouter.HandleFunc("/devices/renew-token", deviceHandler.RenewToken).Methods("POST")
	apiRouter.HandleFunc("/devices/sync-tokens", deviceHandler.SyncTokens).Methods("POST")
	apiRouter.HandleFunc("/devices/update-apns-token", deviceHandler.UpdateAPNSToken).Methods("POST")
	apiRouter.HandleFunc("/devices/update-fcm-token", deviceHandler.UpdateFCMToken).Methods("POST")

	// Ruta de WebSocket
	router.HandleFunc("/ws", wsManager.HandleConnection)

	// Rutas de health y métricas
	router.HandleFunc("/health", healthHandler.Check).Methods("GET")

	if cfg.Monitoring.MetricsEnabled {
		router.Handle("/metrics", promhttp.Handler())
	}
	/* // Crear router
	router := mux.NewRouter()

	// Rutas API
	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	// Rutas de notificaciones
	apiRouter.HandleFunc("/notifications/send", notificationHandler.SendNotification).Methods("POST")
	apiRouter.HandleFunc("/notifications/{id}", notificationHandler.GetNotification).Methods("GET")
	apiRouter.HandleFunc("/users/{user_id}/notifications", notificationHandler.GetUserNotifications).Methods("GET")
	apiRouter.HandleFunc("/notifications/confirm", notificationHandler.ConfirmDelivery).Methods("POST")

	// Rutas de dispositivos
	apiRouter.HandleFunc("/devices/register", deviceHandler.RegisterDevice).Methods("POST")
	apiRouter.HandleFunc("/devices/{id}", deviceHandler.GetDevice).Methods("GET")
	apiRouter.HandleFunc("/devices/link", deviceHandler.LinkDeviceToUser).Methods("POST")
	apiRouter.HandleFunc("/devices/token", deviceHandler.UpdateToken).Methods("POST")

	// Ruta de WebSocket
	router.HandleFunc("/ws", wsManager.HandleConnection)

	// Rutas de health y métricas
	router.HandleFunc("/health", healthHandler.Check).Methods("GET")

	if cfg.Monitoring.MetricsEnabled {
		router.Handle("/metrics", promhttp.Handler())
	} */

	// Configurar middleware
	router.Use(createLoggingMiddleware(logger))

	// Configurar servidor HTTP
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// Iniciar el servidor en una goroutine
	go func() {
		logger.Info("Starting server on port %d", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server: %v", err)
		}
	}()

	// Configurar grácilmente el cierre
	gracefulShutdown(srv, wsManager, cfg.Server.ShutdownTimeout, logger)
}

// Middleware para loggear peticiones
func createLoggingMiddleware(logger *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("%s %s %s", r.Method, r.RequestURI, time.Since(start))
		})
	}
}

// Manejo de cierre gracioso
func gracefulShutdown(srv *http.Server, wsManager *websocket.WebSocketManager, timeout time.Duration, logger *logging.Logger) {
	// Canal para recibir señales de sistema
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Esperar señal
	<-stop
	logger.Info("Shutting down gracefully...")

	// Crear contexto con timeout para shutdown
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Primero cerrar el servidor HTTP
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error: %v", err)
	}

	// Luego cerrar el WebSocket manager
	wsManager.Shutdown()

	logger.Info("Server gracefully stopped")
}
