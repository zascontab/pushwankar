package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"notification-service/internal/domain/entity"
	"notification-service/internal/domain/repository"
	"notification-service/pkg/utils"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

// Errores comunes del servicio de tokens
var (
	ErrTokenNotFound         = errors.New("token not found")
	ErrInvalidToken          = errors.New("invalid token")
	ErrTokenExpired          = errors.New("token has expired")
	ErrFailedToGenerateToken = errors.New("failed to generate token")
	ErrFailedToSaveToken     = errors.New("failed to save token")
	ErrInvalidSigningMethod  = errors.New("invalid signing method")
)

// Claims define los datos que se almacenan en un JWT
type Claims struct {
	DeviceID         string `json:"device_id,omitempty"`
	DeviceIdentifier string `json:"device_identifier,omitempty"`
	UserID           string `json:"user_id,omitempty"`
	IsTemporary      bool   `json:"temp,omitempty"`
	jwt.RegisteredClaims
}

// TokenService define las operaciones de negocio para gestionar tokens
type TokenService struct {
	tokenRepo   repository.TokenRepository
	deviceRepo  repository.DeviceRepository
	jwtSecret   []byte
	tokenExpiry time.Duration
	tempExpiry  time.Duration
}

// NewTokenService crea una nueva instancia del servicio de tokens
func NewTokenService(
	tokenRepo repository.TokenRepository,
	deviceRepo repository.DeviceRepository,
	jwtSecret string,
	tokenExpiry time.Duration,
	tempExpiry time.Duration,
) *TokenService {
	return &TokenService{
		tokenRepo:   tokenRepo,
		deviceRepo:  deviceRepo,
		jwtSecret:   []byte(jwtSecret),
		tokenExpiry: tokenExpiry,
		tempExpiry:  tempExpiry,
	}
}

// GenerateTemporaryToken genera un token temporal para un dispositivo
func (s *TokenService) GenerateTemporaryToken(deviceIdentifier string) (string, error) {
	// Crear claims
	claims := Claims{
		DeviceIdentifier: deviceIdentifier,
		IsTemporary:      true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tempExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// Crear token con claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Firmar token
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", ErrFailedToGenerateToken
	}

	return tokenString, nil
}

// GeneratePermanentToken genera un token permanente para un usuario y dispositivo
func (s *TokenService) GeneratePermanentToken(userID string, deviceID uuid.UUID) (string, error) {
	// Verificar si el dispositivo existe
	device, err := s.deviceRepo.GetByID(context.Background(), deviceID)
	if err != nil {
		return "", ErrDeviceNotFound
	}

	// Crear claims
	claims := Claims{
		DeviceID:         deviceID.String(),
		DeviceIdentifier: device.DeviceIdentifier,
		UserID:           userID,
		IsTemporary:      false,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// Crear token con claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Firmar token
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", ErrFailedToGenerateToken
	}

	return tokenString, nil
}

// VerifyToken verifica un token y devuelve los datos del dispositivo
func (s *TokenService) VerifyToken(ctx context.Context, tokenString string) (*Claims, error) {
	// Parsear token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validar método de firma
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidSigningMethod
		}
		return s.jwtSecret, nil
	})

	// Manejar errores específicos
	if err != nil {
		if strings.Contains(err.Error(), "token is expired") {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	// Obtener claims
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// ExtractDeviceIdentifierFromToken extrae el identificador del dispositivo de un token expirado
func (s *TokenService) ExtractDeviceIdentifierFromToken(tokenString string) (string, error) {
	// Dividir token en partes
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return "", ErrInvalidToken
	}

	// Decodificar la parte de los claims
	claimsJSON, err := utils.DecodeBase64URL(parts[1])
	if err != nil {
		return "", err
	}

	// Extraer deviceIdentifier
	var claims map[string]interface{}
	if err := utils.JSONUnmarshal(claimsJSON, &claims); err != nil {
		return "", err
	}

	// Buscar por deviceIdentifier primero
	if deviceIdentifier, ok := claims["device_identifier"].(string); ok && deviceIdentifier != "" {
		return deviceIdentifier, nil
	}

	// Si no hay deviceIdentifier, buscar por deviceID
	if deviceID, ok := claims["device_id"].(string); ok && deviceID != "" {
		// Si tenemos deviceID, intentar obtener el dispositivo de la base de datos
		deviceUUID, err := uuid.Parse(deviceID)
		if err != nil {
			return "", err
		}

		device, err := s.deviceRepo.GetByID(context.Background(), deviceUUID)
		if err != nil {
			return "", err
		}

		return device.DeviceIdentifier, nil
	}

	return "", errors.New("no device identifier found in token")
}

// VerifyTemporaryToken verifica si un token temporal es válido para un dispositivo
func (s *TokenService) VerifyTemporaryToken(tokenString, deviceIdentifier string) bool {
	claims, err := s.VerifyToken(context.Background(), tokenString)
	if err != nil {
		return false
	}

	return claims.IsTemporary && claims.DeviceIdentifier == deviceIdentifier
}

// RevokeToken revoca un token específico
func (s *TokenService) RevokeToken(ctx context.Context, tokenID uuid.UUID) error {
	return s.tokenRepo.Revoke(ctx, tokenID)
}

// RevokeAllForDevice revoca todos los tokens de un dispositivo
func (s *TokenService) RevokeAllForDevice(ctx context.Context, deviceID uuid.UUID) error {
	return s.tokenRepo.RevokeAllForDevice(ctx, deviceID)
}

// SaveToken guarda un token en el sistema
func (s *TokenService) SaveToken(ctx context.Context, deviceID uuid.UUID, tokenValue string, tokenType entity.TokenType) error {
	token := entity.NewNotificationToken(deviceID, tokenValue, tokenType)
	return s.tokenRepo.Create(ctx, token)
}

// GetToken obtiene un token por ID de dispositivo y tipo
func (s *TokenService) GetToken(ctx context.Context, deviceID uuid.UUID, tokenType entity.TokenType) (*entity.NotificationToken, error) {
	return s.tokenRepo.GetByDeviceAndType(ctx, deviceID, tokenType)
}

// CleanupExpiredTokens limpia los tokens expirados
func (s *TokenService) CleanupExpiredTokens(ctx context.Context) error {
	return s.tokenRepo.CleanupExpired(ctx)
}
