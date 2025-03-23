package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config contiene todas las configuraciones del servicio
type Config struct {
	Server          ServerConfig
	Database        DatabaseConfig
	JWT             JWTConfig
	BusinessService BusinessServiceConfig
	WebSocket       WebSocketConfig
	Monitoring      MonitoringConfig
	Logging         LoggingConfig
}

// ServerConfig contiene la configuración del servidor HTTP
type ServerConfig struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// DatabaseConfig contiene la configuración de la base de datos
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	Schema   string
	SSLMode  string
}

// JWTConfig contiene la configuración de JWT
type JWTConfig struct {
	Secret               string
	TokenExpiry          time.Duration
	TemporaryTokenExpiry time.Duration
}

// BusinessServiceConfig contiene la configuración para comunicarse con el servicio de negocio
type BusinessServiceConfig struct {
	GRPCAddress string
	Timeout     time.Duration
}

// WebSocketConfig contiene la configuración del servidor WebSocket
type WebSocketConfig struct {
	Path              string
	PingInterval      time.Duration
	PongWait          time.Duration
	MaxMessageSize    int64
	WriteWait         time.Duration
	MessageBufferSize int
}

// MonitoringConfig contiene la configuración de monitoreo
type MonitoringConfig struct {
	MetricsEnabled bool
	MetricsPort    int
}

// LoggingConfig contiene la configuración de logging
type LoggingConfig struct {
	Level     string
	UseColors bool
}

// LoadConfig carga la configuración desde variables de entorno o archivo .env
func LoadConfig() (*Config, error) {
	// Cargar variables de entorno del archivo .env si existe
	_ = godotenv.Load() // No importa si falla (en producción no se usa .env)

	config := &Config{
		Server: ServerConfig{
			Port:            getEnvAsInt("SERVER_PORT", 8080),
			ReadTimeout:     getEnvAsDuration("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    getEnvAsDuration("SERVER_WRITE_TIMEOUT", 10*time.Second),
			ShutdownTimeout: getEnvAsDuration("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			Name:     getEnv("DB_NAME", "notification"),
			Schema:   getEnv("DB_SCHEMA", "notification_service"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			Secret:               getEnv("JWT_SECRET", "your-secret-key"),
			TokenExpiry:          getEnvAsDuration("JWT_TOKEN_EXPIRY", 24*time.Hour),
			TemporaryTokenExpiry: getEnvAsDuration("JWT_TEMP_TOKEN_EXPIRY", 30*time.Minute),
		},
		BusinessService: BusinessServiceConfig{
			GRPCAddress: getEnv("BUSINESS_SERVICE_GRPC_ADDRESS", "localhost:50051"),
			Timeout:     getEnvAsDuration("BUSINESS_SERVICE_TIMEOUT", 5*time.Second),
		},
		WebSocket: WebSocketConfig{
			Path:              getEnv("WS_PATH", "/ws"),
			PingInterval:      getEnvAsDuration("WS_PING_INTERVAL", 54*time.Second),
			PongWait:          getEnvAsDuration("WS_PONG_WAIT", 60*time.Second),
			MaxMessageSize:    getEnvAsInt64("WS_MAX_MESSAGE_SIZE", 4096),
			WriteWait:         getEnvAsDuration("WS_WRITE_WAIT", 10*time.Second),
			MessageBufferSize: getEnvAsInt("WS_MESSAGE_BUFFER_SIZE", 256),
		},
		Monitoring: MonitoringConfig{
			MetricsEnabled: getEnvAsBool("METRICS_ENABLED", true),
			MetricsPort:    getEnvAsInt("METRICS_PORT", 9090),
		},
		Logging: LoggingConfig{
			Level:     getEnv("LOG_LEVEL", "INFO"),
			UseColors: getEnvAsBool("LOG_USE_COLORS", true),
		},
	}

	return config, nil
}

// GetDatabaseDSN devuelve la cadena de conexión a la base de datos
func (c *DatabaseConfig) GetDatabaseDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s search_path=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode, c.Schema,
	)
}

// Funciones auxiliares para obtener variables de entorno

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := getEnv(key, "")
	if value, err := time.ParseDuration(valueStr); err == nil {
		return value
	}
	return defaultValue
}
