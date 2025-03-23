package http

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthHandler maneja las peticiones HTTP relacionadas con el estado del servicio
type HealthHandler struct {
	startTime time.Time
}

// NewHealthHandler crea un nuevo HealthHandler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{
		startTime: time.Now(),
	}
}

// Check proporciona información sobre el estado del servicio
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "ok",
		"uptime":    time.Since(h.startTime).String(),
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "notification-service",
		"version":   "1.0.0", // Puedes obtener esto de una variable de entorno o un archivo de versión
	}

	respondWithJSON(w, http.StatusOK, status)
}

// Helpers para responder con JSON (si no están ya definidos en otro lugar)
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}
