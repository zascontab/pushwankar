package http

import (
	"encoding/json"
	"net/http"
	"notification-service/internal/domain/entity"
	"notification-service/internal/usecase"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// DeviceHandler maneja las peticiones HTTP relacionadas con dispositivos
type DeviceHandler struct {
	deviceService *usecase.DeviceService
	tokenService  *usecase.TokenService
}

// NewDeviceHandler crea un nuevo DeviceHandler
func NewDeviceHandler(deviceService *usecase.DeviceService, tokenService *usecase.TokenService) *DeviceHandler {
	return &DeviceHandler{
		deviceService: deviceService,
		tokenService:  tokenService,
	}
}

// RegisterDevice registra un nuevo dispositivo
func (h *DeviceHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceIdentifier string `json:"device_identifier"`
		UserID           string `json:"user_id,omitempty"`
		Model            string `json:"model,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validar campos obligatorios
	if req.DeviceIdentifier == "" {
		respondWithError(w, http.StatusBadRequest, "device_identifier is required")
		return
	}

	var device *entity.Device
	var err error
	var token string

	// Si se proporciona un usuario, asociar el dispositivo al usuario
	if req.UserID != "" {
		userID, err := strconv.ParseUint(req.UserID, 10, 32)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid user_id")
			return
		}

		model := req.Model
		uintUserID := uint(userID)

		device, err = h.deviceService.RegisterDeviceWithUser(r.Context(), req.DeviceIdentifier, uintUserID, &model)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Generar token permanente
		token, err = h.tokenService.GeneratePermanentToken(req.UserID, device.ID)
	} else {
		// Registrar dispositivo sin usuario
		model := req.Model
		device, err = h.deviceService.RegisterDeviceWithoutUser(r.Context(), req.DeviceIdentifier, &model)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Generar token temporal
		token, err = h.tokenService.GenerateTemporaryToken(device.DeviceIdentifier)
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	response := map[string]interface{}{
		"device_id":   device.ID,
		"token":       token,
		"is_verified": device.Verified,
	}

	respondWithJSON(w, http.StatusOK, response)
}

// RegisterDeviceWithoutUser registra un nuevo dispositivo sin usuario asociado
// Esta es una ruta específica para mantener compatibilidad con el módulo original
func (h *DeviceHandler) RegisterDeviceWithoutUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceIdentifier string `json:"device_identifier"`
		Model            string `json:"model,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validar campos obligatorios
	if req.DeviceIdentifier == "" {
		respondWithError(w, http.StatusBadRequest, "device_identifier is required")
		return
	}

	// Registrar dispositivo sin usuario
	model := req.Model
	device, err := h.deviceService.RegisterDeviceWithoutUser(r.Context(), req.DeviceIdentifier, &model)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Generar token temporal
	token, err := h.tokenService.GenerateTemporaryToken(device.DeviceIdentifier)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	response := map[string]interface{}{
		"device_id":   device.ID,
		"token":       token,
		"is_verified": device.Verified,
	}

	respondWithJSON(w, http.StatusOK, response)
}

// GetDevice obtiene información de un dispositivo
func (h *DeviceHandler) GetDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID, err := uuid.Parse(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	device, err := h.deviceService.GetDevice(r.Context(), deviceID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Device not found")
		return
	}

	respondWithJSON(w, http.StatusOK, device)
}

// GetUserDevices obtiene todos los dispositivos de un usuario
func (h *DeviceHandler) GetUserDevices(w http.ResponseWriter, r *http.Request) {
	// Obtener user_id de los parámetros de consulta
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		respondWithError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	// Convertir a uint
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user_id")
		return
	}

	// Obtener dispositivos
	devices, err := h.deviceService.GetUserDevices(r.Context(), uint(userID))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"devices": devices,
	})
}

// LinkDeviceToUser vincula un dispositivo a un usuario
func (h *DeviceHandler) LinkDeviceToUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
		UserID   string `json:"user_id"`
		Token    string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validar campos obligatorios
	if req.DeviceID == "" || req.UserID == "" {
		respondWithError(w, http.StatusBadRequest, "device_id and user_id are required")
		return
	}

	// Verificar token temporal si se proporciona
	if req.Token != "" {
		isValid := h.tokenService.VerifyTemporaryToken(req.Token, req.DeviceID)
		if !isValid {
			respondWithError(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}
	}

	// Convertir IDs a formatos apropiados
	deviceID, err := uuid.Parse(req.DeviceID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	userID, err := strconv.ParseUint(req.UserID, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Vincular dispositivo a usuario
	err = h.deviceService.LinkDeviceToUser(r.Context(), deviceID, uint(userID))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Generar nuevo token permanente
	newToken, err := h.tokenService.GeneratePermanentToken(req.UserID, deviceID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"token": newToken,
	})
}

// UpdateToken actualiza un token de notificación
func (h *DeviceHandler) UpdateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID  string `json:"device_id"`
		Token     string `json:"token"`
		TokenType string `json:"token_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validar campos obligatorios
	if req.DeviceID == "" || req.Token == "" {
		respondWithError(w, http.StatusBadRequest, "device_id and token are required")
		return
	}

	// Determinar tipo de token
	tokenType := entity.TokenTypeWebSocket
	if req.TokenType != "" {
		switch req.TokenType {
		case "apns":
			tokenType = entity.TokenTypeAPNS
		case "fcm":
			tokenType = entity.TokenTypeFCM
		}
	}

	// Convertir DeviceID a UUID
	deviceID, err := uuid.Parse(req.DeviceID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	// Actualizar token
	err = h.tokenService.SaveToken(r.Context(), deviceID, req.Token, tokenType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"status": "success",
	})
}

// RenewToken renueva un token de autenticación
func (h *DeviceHandler) RenewToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token          string `json:"token"`
		DeviceID       string `json:"device_id,omitempty"`
		ForceTemporary bool   `json:"force_temporary,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validar campos obligatorios
	if req.Token == "" {
		respondWithError(w, http.StatusBadRequest, "token is required")
		return
	}

	// Verificar token actual
	claims, err := h.tokenService.VerifyToken(r.Context(), req.Token)
	if err != nil {
		// Si el token está expirado pero podemos extraer el deviceIdentifier, generar uno nuevo
		deviceIdentifier, extractErr := h.tokenService.ExtractDeviceIdentifierFromToken(req.Token)
		if extractErr != nil {
			respondWithError(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		// Si forzamos token temporal o no hay ID de usuario en las claims
		if req.ForceTemporary || (claims == nil || claims.UserID == "") {
			// Generar token temporal
			newToken, err := h.tokenService.GenerateTemporaryToken(deviceIdentifier)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
				return
			}

			respondWithJSON(w, http.StatusOK, map[string]string{
				"token":      newToken,
				"token_type": "temporary",
			})
			return
		}

		// Si hay claims con ID de usuario, generar token permanente
		if claims != nil && claims.UserID != "" && claims.DeviceID != "" {
			deviceID, err := uuid.Parse(claims.DeviceID)
			if err != nil {
				respondWithError(w, http.StatusBadRequest, "Invalid device ID in token")
				return
			}

			newToken, err := h.tokenService.GeneratePermanentToken(claims.UserID, deviceID)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
				return
			}

			respondWithJSON(w, http.StatusOK, map[string]string{
				"token":      newToken,
				"token_type": "permanent",
			})
			return
		}

		respondWithError(w, http.StatusBadRequest, "Cannot renew token")
		return
	}

	// Si el token es válido, simplemente devolvemos el mismo token
	tokenType := "permanent"
	if claims.IsTemporary {
		tokenType = "temporary"
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"token":      req.Token,
		"token_type": tokenType,
	})
}

// SyncTokens sincroniza varios tokens para un mismo dispositivo
func (h *DeviceHandler) SyncTokens(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID  string `json:"device_id"`
		WSToken   string `json:"ws_token,omitempty"`
		FCMToken  string `json:"fcm_token,omitempty"`
		APNSToken string `json:"apns_token,omitempty"`
		AuthToken string `json:"auth_token"` // Token de autenticación para validar la solicitud
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validar campos obligatorios
	if req.DeviceID == "" || req.AuthToken == "" {
		respondWithError(w, http.StatusBadRequest, "device_id and auth_token are required")
		return
	}

	// Verificar token de autenticación
	_, err := h.tokenService.VerifyToken(r.Context(), req.AuthToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid authentication token")
		return
	}

	// Convertir DeviceID a UUID
	deviceID, err := uuid.Parse(req.DeviceID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	// Actualizar cada tipo de token si está presente
	var updatedTokens []string

	if req.WSToken != "" {
		err = h.tokenService.SaveToken(r.Context(), deviceID, req.WSToken, entity.TokenTypeWebSocket)
		if err == nil {
			updatedTokens = append(updatedTokens, "websocket")
		}
	}

	if req.FCMToken != "" {
		err = h.tokenService.SaveToken(r.Context(), deviceID, req.FCMToken, entity.TokenTypeFCM)
		if err == nil {
			updatedTokens = append(updatedTokens, "fcm")
		}
	}

	if req.APNSToken != "" {
		err = h.tokenService.SaveToken(r.Context(), deviceID, req.APNSToken, entity.TokenTypeAPNS)
		if err == nil {
			updatedTokens = append(updatedTokens, "apns")
		}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "success",
		"updated_tokens": updatedTokens,
	})
}

// UpdateAPNSToken actualiza específicamente un token APNS
func (h *DeviceHandler) UpdateAPNSToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID  string `json:"device_id"`
		Token     string `json:"token"`
		AuthToken string `json:"auth_token,omitempty"` // Token de autenticación opcional
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validar campos obligatorios
	if req.DeviceID == "" || req.Token == "" {
		respondWithError(w, http.StatusBadRequest, "device_id and token are required")
		return
	}

	// Si se proporciona un token de autenticación, verificarlo
	if req.AuthToken != "" {
		_, err := h.tokenService.VerifyToken(r.Context(), req.AuthToken)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "Invalid authentication token")
			return
		}
	}

	// Convertir DeviceID a UUID
	deviceID, err := uuid.Parse(req.DeviceID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	// Actualizar token APNS
	err = h.tokenService.SaveToken(r.Context(), deviceID, req.Token, entity.TokenTypeAPNS)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"status": "success",
	})
}

// UpdateFCMToken actualiza específicamente un token FCM
func (h *DeviceHandler) UpdateFCMToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID  string `json:"device_id"`
		Token     string `json:"token"`
		AuthToken string `json:"auth_token,omitempty"` // Token de autenticación opcional
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validar campos obligatorios
	if req.DeviceID == "" || req.Token == "" {
		respondWithError(w, http.StatusBadRequest, "device_id and token are required")
		return
	}

	// Si se proporciona un token de autenticación, verificarlo
	if req.AuthToken != "" {
		_, err := h.tokenService.VerifyToken(r.Context(), req.AuthToken)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "Invalid authentication token")
			return
		}
	}

	// Convertir DeviceID a UUID
	deviceID, err := uuid.Parse(req.DeviceID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	// Actualizar token FCM
	err = h.tokenService.SaveToken(r.Context(), deviceID, req.Token, entity.TokenTypeFCM)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"status": "success",
	})
}
