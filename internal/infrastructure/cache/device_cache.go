package cache

import (
	"notification-service/internal/domain/entity"
	"time"

	"github.com/google/uuid"
)

// DeviceCache es una implementación de caché para dispositivos
type DeviceCache struct {
	cache *Cache
}

// NewDeviceCache crea una nueva instancia de DeviceCache
func NewDeviceCache(defaultExpiration, cleanupInterval time.Duration) *DeviceCache {
	return &DeviceCache{
		cache: NewCache(defaultExpiration, cleanupInterval),
	}
}

// Set almacena un dispositivo en la caché
func (c *DeviceCache) Set(deviceID uuid.UUID, device *entity.Device) {
	c.cache.Set(deviceID.String(), device)
}

// Get obtiene un dispositivo de la caché
func (c *DeviceCache) Get(deviceID uuid.UUID) (*entity.Device, bool) {
	value, found := c.cache.Get(deviceID.String())
	if !found {
		return nil, false
	}

	device, ok := value.(*entity.Device)
	if !ok {
		return nil, false
	}

	return device, true
}

// GetByIdentifier obtiene un dispositivo por su identificador
func (c *DeviceCache) GetByIdentifier(identifier string) (*entity.Device, bool) {
	// Esta es una operación costosa ya que tenemos que escanear todos los elementos
	// Una mejor opción sería tener un índice secundario, pero para mantener la simplicidad
	// lo haremos así por ahora
	c.cache.mu.RLock()
	defer c.cache.mu.RUnlock()

	now := time.Now().UnixNano()
	for _, item := range c.cache.items {
		// Verificar expiración
		if item.Expiration > 0 && now > item.Expiration {
			continue
		}

		device, ok := item.Value.(*entity.Device)
		if !ok {
			continue
		}

		if device.DeviceIdentifier == identifier {
			return device, true
		}
	}

	return nil, false
}

// Delete elimina un dispositivo de la caché
func (c *DeviceCache) Delete(deviceID uuid.UUID) {
	c.cache.Delete(deviceID.String())
}

// UserDeviceCache es una implementación de caché para los dispositivos de un usuario
type UserDeviceCache struct {
	cache *Cache
}

// NewUserDeviceCache crea una nueva instancia de UserDeviceCache
func NewUserDeviceCache(defaultExpiration, cleanupInterval time.Duration) *UserDeviceCache {
	return &UserDeviceCache{
		cache: NewCache(defaultExpiration, cleanupInterval),
	}
}

// Set almacena los dispositivos de un usuario en la caché
func (c *UserDeviceCache) Set(userID string, devices []*entity.Device) {
	c.cache.Set(userID, devices)
}

// Get obtiene los dispositivos de un usuario de la caché
func (c *UserDeviceCache) Get(userID string) ([]*entity.Device, bool) {
	value, found := c.cache.Get(userID)
	if !found {
		return nil, false
	}

	devices, ok := value.([]*entity.Device)
	if !ok {
		return nil, false
	}

	return devices, true
}

// Delete elimina los dispositivos de un usuario de la caché
func (c *UserDeviceCache) Delete(userID string) {
	c.cache.Delete(userID)
}

// TokenCache es una implementación de caché para tokens
type TokenCache struct {
	cache *Cache
}

// NewTokenCache crea una nueva instancia de TokenCache
func NewTokenCache(defaultExpiration, cleanupInterval time.Duration) *TokenCache {
	return &TokenCache{
		cache: NewCache(defaultExpiration, cleanupInterval),
	}
}

// Set almacena un token en la caché
func (c *TokenCache) Set(deviceID uuid.UUID, tokenType entity.TokenType, token *entity.NotificationToken) {
	key := deviceID.String() + ":" + string(tokenType)
	c.cache.Set(key, token)
}

// Get obtiene un token de la caché
func (c *TokenCache) Get(deviceID uuid.UUID, tokenType entity.TokenType) (*entity.NotificationToken, bool) {
	key := deviceID.String() + ":" + string(tokenType)
	value, found := c.cache.Get(key)
	if !found {
		return nil, false
	}

	token, ok := value.(*entity.NotificationToken)
	if !ok {
		return nil, false
	}

	return token, true
}

// Delete elimina un token de la caché
func (c *TokenCache) Delete(deviceID uuid.UUID, tokenType entity.TokenType) {
	key := deviceID.String() + ":" + string(tokenType)
	c.cache.Delete(key)
}

// NotificationCache es una implementación de caché para notificaciones
type NotificationCache struct {
	cache *Cache
}

// NewNotificationCache crea una nueva instancia de NotificationCache
func NewNotificationCache(defaultExpiration, cleanupInterval time.Duration) *NotificationCache {
	return &NotificationCache{
		cache: NewCache(defaultExpiration, cleanupInterval),
	}
}

// Set almacena una notificación en la caché
func (c *NotificationCache) Set(notificationID uuid.UUID, notification *entity.Notification) {
	c.cache.Set(notificationID.String(), notification)
}

// Get obtiene una notificación de la caché
func (c *NotificationCache) Get(notificationID uuid.UUID) (*entity.Notification, bool) {
	value, found := c.cache.Get(notificationID.String())
	if !found {
		return nil, false
	}

	notification, ok := value.(*entity.Notification)
	if !ok {
		return nil, false
	}

	return notification, true
}

// Delete elimina una notificación de la caché
func (c *NotificationCache) Delete(notificationID uuid.UUID) {
	c.cache.Delete(notificationID.String())
}
