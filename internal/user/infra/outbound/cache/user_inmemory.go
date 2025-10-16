package cache

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	// Importamos la interfaz de caché compartida para asegurar la compatibilidad.
	sharedCache "github.com/davicafu/hexagolab/shared/platform/cache"
)

// cacheItem guarda el valor y el tiempo de expiración.
type cacheItem struct {
	value     []byte // Guardamos los bytes para simular la serialización, igual que Redis.
	expiresAt time.Time
}

// InMemoryCache implementa la interfaz de caché usando un mapa en memoria.
type InMemoryCache struct {
	store      map[string]cacheItem
	mu         sync.RWMutex // RWMutex permite múltiples lectores o un solo escritor.
	defaultTTL time.Duration
	stopChan   chan struct{} // Canal para detener la goroutine de limpieza.
}

// Verificación estática: asegura en tiempo de compilación que InMemoryCache implementa la interfaz compartida.
var _ sharedCache.Cache = (*InMemoryCache)(nil)

// NewInMemoryCache crea una nueva instancia de la caché en memoria.
// - defaultTTL: El tiempo de vida por defecto para las claves si no se especifica otro.
// - cleanupInterval: Cada cuánto tiempo se revisarán y eliminarán las claves expiradas.
func NewInMemoryCache(defaultTTL, cleanupInterval time.Duration) *InMemoryCache {
	c := &InMemoryCache{
		store:      make(map[string]cacheItem),
		defaultTTL: defaultTTL,
		stopChan:   make(chan struct{}),
	}

	// Inicia el proceso de limpieza de items expirados en segundo plano.
	go c.cleanupLoop(cleanupInterval)

	return c
}

// Get recupera un valor de la caché. Es seguro para uso concurrente.
func (c *InMemoryCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	c.mu.RLock() // Bloqueo de solo lectura, permite múltiples lectores.
	defer c.mu.RUnlock()

	item, ok := c.store[key]
	if !ok {
		return false, nil // Cache miss: la clave no existe.
	}

	// Comprueba si el item ha expirado.
	if time.Now().UTC().After(item.expiresAt) {
		return false, nil // Expirado, se trata como un cache miss.
	}

	// Deserializa el valor en la estructura de destino.
	if err := json.Unmarshal(item.value, dest); err != nil {
		return false, err
	}

	return true, nil // Cache hit.
}

// Set guarda un valor en la caché. Es seguro para uso concurrente.
func (c *InMemoryCache) Set(ctx context.Context, key string, val interface{}, ttlSecs int) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	c.mu.Lock() // Bloqueo de escritura, bloquea a todos los demás.
	defer c.mu.Unlock()

	ttl := c.defaultTTL
	if ttlSecs > 0 {
		ttl = time.Duration(ttlSecs) * time.Second
	}

	c.store[key] = cacheItem{
		value:     data,
		expiresAt: time.Now().UTC().Add(ttl),
	}

	return nil
}

// Delete elimina un valor de la caché. Es seguro para uso concurrente.
func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock() // Bloqueo de escritura.
	defer c.mu.Unlock()

	delete(c.store, key)
	return nil
}

// Stop detiene la goroutine de limpieza. Deberías llamarlo al apagar la aplicación.
func (c *InMemoryCache) Stop() {
	close(c.stopChan)
}

// cleanupLoop es la goroutine que se ejecuta periódicamente para limpiar claves expiradas.
func (c *InMemoryCache) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Momento de limpiar.
			c.mu.Lock() // Necesitamos un bloqueo de escritura para poder eliminar claves.
			for key, item := range c.store {
				if time.Now().UTC().After(item.expiresAt) {
					delete(c.store, key)
				}
			}
			c.mu.Unlock()
		case <-c.stopChan:
			// Se recibió la señal de parar, terminamos la goroutine.
			return
		}
	}
}
