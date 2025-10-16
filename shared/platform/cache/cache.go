package cache

import (
	"context"
)

// Cache define la interfaz para una caché de clave-valor genérica.
type Cache interface {
	// Get intenta poblar 'dest' (que debe ser un puntero) con el valor asociado a la 'key'.
	// Devuelve (true, nil) si hay un 'hit' y 'dest' fue rellenado.
	// Devuelve (false, nil) si es un 'miss'.
	Get(ctx context.Context, key string, dest interface{}) (bool, error)

	// Set serializa y guarda el valor con un TTL (Time To Live) en segundos.
	Set(ctx context.Context, key string, val interface{}, ttlSecs int) error

	// Delete elimina la 'key' de la caché.
	Delete(ctx context.Context, key string) error
}
