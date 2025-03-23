package cache

import (
	"sync"
	"time"
)

// Item representa un elemento en la caché con su tiempo de expiración
type Item struct {
	Value      interface{}
	Expiration int64
}

// InMemoryCache es una implementación simple de caché en memoria con expiración
type InMemoryCache struct {
	items             map[string]Item
	mu                sync.RWMutex
	defaultExpiration time.Duration
	cleanupInterval   time.Duration
	stopCleanup       chan bool
}

// NewInMemoryCache crea una nueva instancia de caché
func NewInMemoryCache(defaultExpiration, cleanupInterval time.Duration) *InMemoryCache {
	cache := &InMemoryCache{
		items:             make(map[string]Item),
		defaultExpiration: defaultExpiration,
		cleanupInterval:   cleanupInterval,
		stopCleanup:       make(chan bool),
	}

	// Iniciar rutina de limpieza si el intervalo es mayor a 0
	if cleanupInterval > 0 {
		go cache.startCleanupTimer()
	}

	return cache
}

// Set almacena un valor en la caché con el tiempo de expiración predeterminado
func (c *InMemoryCache) Set(key string, value interface{}) {
	c.SetWithExpiration(key, value, c.defaultExpiration)
}

// SetWithExpiration almacena un valor en la caché con un tiempo de expiración específico
func (c *InMemoryCache) SetWithExpiration(key string, value interface{}, expiration time.Duration) {
	var expirationTime int64

	if expiration == 0 {
		// Usar el tiempo predeterminado
		expiration = c.defaultExpiration
	}

	if expiration > 0 {
		expirationTime = time.Now().Add(expiration).UnixNano()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = Item{
		Value:      value,
		Expiration: expirationTime,
	}
}

// Get obtiene un valor de la caché
func (c *InMemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.items[key]
	if !found {
		return nil, false
	}

	// Verificar si el elemento ha expirado
	if item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
		return nil, false
	}

	return item.Value, true
}

// Delete elimina un elemento de la caché
func (c *InMemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Flush elimina todos los elementos de la caché
func (c *InMemoryCache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]Item)
}

// startCleanupTimer inicia la rutina de limpieza periódica
func (c *InMemoryCache) startCleanupTimer() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-c.stopCleanup:
			return
		}
	}
}

// deleteExpired elimina los elementos expirados de la caché
func (c *InMemoryCache) deleteExpired() {
	now := time.Now().UnixNano()

	c.mu.Lock()
	defer c.mu.Unlock()

	for key, item := range c.items {
		if item.Expiration > 0 && now > item.Expiration {
			delete(c.items, key)
		}
	}
}

// StopCleanup detiene la rutina de limpieza
func (c *InMemoryCache) StopCleanup() {
	if c.cleanupInterval > 0 {
		c.stopCleanup <- true
	}
}
