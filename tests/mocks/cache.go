package mocks

import (
	"context"
	"encoding/json"
	"sync"

	sharedCache "github.com/davicafu/hexagolab/internal/shared/infra/platform/cache"
)

// DummyCache es un mock de caché en memoria, genérico y seguro para concurrencia.
// Puede almacenar cualquier tipo de objeto serializable a JSON.
type DummyCache struct {
	store map[string][]byte // ✅ Almacenamos bytes (JSON), no un tipo concreto.
	mu    sync.RWMutex      // ✅ Usamos RWMutex para mejor rendimiento en lecturas.
}

// Verificación estática para asegurar que implementa la interfaz compartida.
var _ sharedCache.Cache = (*DummyCache)(nil)

func NewDummyCache() *DummyCache {
	return &DummyCache{
		store: make(map[string][]byte),
	}
}

func (c *DummyCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	c.mu.RLock() // Bloqueo de solo lectura
	defer c.mu.RUnlock()

	data, ok := c.store[key]
	if !ok {
		return false, nil // Cache miss
	}

	// ✅ Deserializa los bytes en el puntero 'dest' de cualquier tipo.
	if err := json.Unmarshal(data, dest); err != nil {
		return false, err
	}
	return true, nil // Cache hit
}

func (c *DummyCache) Set(ctx context.Context, key string, val interface{}, ttlSecs int) error {
	c.mu.Lock() // Bloqueo de escritura
	defer c.mu.Unlock()

	// ✅ Serializa el valor (de cualquier tipo) a JSON antes de guardarlo.
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	c.store[key] = data
	return nil
}

func (c *DummyCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock() // Bloqueo de escritura
	defer c.mu.Unlock()
	delete(c.store, key)
	return nil
}
