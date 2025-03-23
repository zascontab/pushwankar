package throttling

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ThrottleStrategy define la estrategia de limitación
type ThrottleStrategy string

const (
	// StrategyAllow permite todas las solicitudes pero puede retrasarlas
	StrategyAllow ThrottleStrategy = "allow"
	// StrategyDrop rechaza las solicitudes que excedan el límite
	StrategyDrop ThrottleStrategy = "drop"
	// StrategyBlock bloquea hasta que se pueda procesar la solicitud
	StrategyBlock ThrottleStrategy = "block"
)

// RateLimiter es una interfaz para el control de tasas
type RateLimiter interface {
	// Allow comprueba si se permite una solicitud y consume un token
	Allow() bool
	// Wait espera hasta que se pueda consumir un token
	Wait() bool
	// Reset restablece el limitador
	Reset()
}

// UserThrottler limita la tasa de notificaciones por usuario
type UserThrottler struct {
	// Limitadores por usuario
	limiters map[string]*rate.Limiter
	// Mutex para operaciones concurrentes
	mu sync.RWMutex
	// Límite de notificaciones por segundo por usuario
	limit rate.Limit
	// Tamaño del bucket de tokens
	burst int
	// Tiempo de expiración para entradas no utilizadas
	expiry time.Duration
	// Mapa para rastrear la última vez que se usó un limitador
	lastUsed map[string]time.Time
	// Canal para detener la limpieza
	stopCleanup chan struct{}
	// WaitGroup para esperar a que la limpieza termine
	wg sync.WaitGroup
	// Estrategia de limitación
	strategy ThrottleStrategy
}

// NewUserThrottler crea un nuevo UserThrottler
func NewUserThrottler(rps float64, burst int, expiry time.Duration, strategy ThrottleStrategy) *UserThrottler {
	t := &UserThrottler{
		limiters:    make(map[string]*rate.Limiter),
		limit:       rate.Limit(rps),
		burst:       burst,
		expiry:      expiry,
		lastUsed:    make(map[string]time.Time),
		stopCleanup: make(chan struct{}),
		strategy:    strategy,
	}

	// Iniciar rutina de limpieza
	t.wg.Add(1)
	go t.cleanup()

	return t
}

// getLimiter obtiene un limitador para un usuario, creándolo si no existe
func (t *UserThrottler) getLimiter(userID string) *rate.Limiter {
	t.mu.RLock()
	limiter, exists := t.limiters[userID]
	t.mu.RUnlock()

	if exists {
		// Actualizar tiempo de último uso
		t.mu.Lock()
		t.lastUsed[userID] = time.Now()
		t.mu.Unlock()
		return limiter
	}

	// Crear nuevo limitador
	t.mu.Lock()
	defer t.mu.Unlock()

	// Verificar nuevamente en caso de carrera
	if limiter, exists = t.limiters[userID]; exists {
		t.lastUsed[userID] = time.Now()
		return limiter
	}

	limiter = rate.NewLimiter(t.limit, t.burst)
	t.limiters[userID] = limiter
	t.lastUsed[userID] = time.Now()

	return limiter
}

// Allow comprueba si se permite una solicitud para el usuario
func (t *UserThrottler) Allow(userID string) bool {
	limiter := t.getLimiter(userID)

	switch t.strategy {
	case StrategyAllow:
		// Intentar reservar pero sin bloquear
		if limiter.Allow() {
			return true
		}
		// Si no hay tokens disponibles, esperamos un poco
		time.Sleep(100 * time.Millisecond)
		return limiter.Allow()
	case StrategyDrop:
		// Simplemente rechazar si no hay tokens
		return limiter.Allow()
	case StrategyBlock:
		// Esperar hasta que haya un token disponible
		r := limiter.Reserve()
		if !r.OK() {
			return false
		}
		time.Sleep(r.Delay())
		return true
	default:
		return limiter.Allow()
	}
}

// Reset restablece el limitador para un usuario
func (t *UserThrottler) Reset(userID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.limiters, userID)
	delete(t.lastUsed, userID)
}

// cleanup elimina limitadores no utilizados
func (t *UserThrottler) cleanup() {
	defer t.wg.Done()

	ticker := time.NewTicker(t.expiry / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.removeExpired()
		case <-t.stopCleanup:
			return
		}
	}
}

// removeExpired elimina limitadores no utilizados
func (t *UserThrottler) removeExpired() {
	now := time.Now()
	expired := []string{}

	t.mu.RLock()
	for userID, lastTime := range t.lastUsed {
		if now.Sub(lastTime) > t.expiry {
			expired = append(expired, userID)
		}
	}
	t.mu.RUnlock()

	// Eliminar entradas expiradas
	if len(expired) > 0 {
		t.mu.Lock()
		for _, userID := range expired {
			delete(t.limiters, userID)
			delete(t.lastUsed, userID)
		}
		t.mu.Unlock()
	}
}

// Stop detiene la rutina de limpieza
func (t *UserThrottler) Stop() {
	close(t.stopCleanup)
	t.wg.Wait()
}

// DeviceThrottler limita la tasa de notificaciones por dispositivo
type DeviceThrottler struct {
	// Limiter interno por dispositivo
	throttler *UserThrottler
}

// NewDeviceThrottler crea un nuevo DeviceThrottler
func NewDeviceThrottler(rps float64, burst int, expiry time.Duration, strategy ThrottleStrategy) *DeviceThrottler {
	return &DeviceThrottler{
		throttler: NewUserThrottler(rps, burst, expiry, strategy),
	}
}

// Allow comprueba si se permite una solicitud para el dispositivo
func (t *DeviceThrottler) Allow(deviceID string) bool {
	return t.throttler.Allow(deviceID)
}

// Reset restablece el limitador para un dispositivo
func (t *DeviceThrottler) Reset(deviceID string) {
	t.throttler.Reset(deviceID)
}

// Stop detiene la rutina de limpieza
func (t *DeviceThrottler) Stop() {
	t.throttler.Stop()
}

// GlobalThrottler limita la tasa global de notificaciones
type GlobalThrottler struct {
	// Limitador único para todo el sistema
	limiter  *rate.Limiter
	strategy ThrottleStrategy
}

// NewGlobalThrottler crea un nuevo GlobalThrottler
func NewGlobalThrottler(rps float64, burst int, strategy ThrottleStrategy) *GlobalThrottler {
	return &GlobalThrottler{
		limiter:  rate.NewLimiter(rate.Limit(rps), burst),
		strategy: strategy,
	}
}

// Allow comprueba si se permite una solicitud a nivel global
func (t *GlobalThrottler) Allow() bool {
	switch t.strategy {
	case StrategyAllow:
		// Intentar reservar pero sin bloquear
		if t.limiter.Allow() {
			return true
		}
		// Si no hay tokens disponibles, esperamos un poco
		time.Sleep(100 * time.Millisecond)
		return t.limiter.Allow()
	case StrategyDrop:
		// Simplemente rechazar si no hay tokens
		return t.limiter.Allow()
	case StrategyBlock:
		// Esperar hasta que haya un token disponible
		r := t.limiter.Reserve()
		if !r.OK() {
			return false
		}
		time.Sleep(r.Delay())
		return true
	default:
		return t.limiter.Allow()
	}
}

// Reset restablece el limitador global
func (t *GlobalThrottler) Reset() {
	// Crear un nuevo limitador con los mismos parámetros
	limit := t.limiter.Limit()
	burst := t.limiter.Burst()
	t.limiter = rate.NewLimiter(limit, burst)
}
