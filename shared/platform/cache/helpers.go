package cache

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// AsyncCacheSet actualiza caché en background sin bloquear
func AsyncCacheSet(ctx context.Context, cache Cache, key string, value interface{}, ttl int, log *zap.Logger) {
	if cache == nil {
		return
	}

	go func() {
		// Usamos context.Background() deliberadamente. Esta es una operación de "dispara y olvida".
		// Queremos que la actualización de la caché tenga éxito incluso si el contexto de la
		// petición original ya ha sido cancelado.
		cacheCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		if err := cache.Set(cacheCtx, key, value, ttl); err != nil {
			log.Warn("Cache update failed",
				zap.String("key", key),
				zap.Error(err))
		}
	}()
}

// AsyncCacheDelete elimina de caché en background
func AsyncCacheDelete(ctx context.Context, cache Cache, key string, log *zap.Logger) {
	if cache == nil {
		return
	}

	go func() {
		if err := cache.Delete(ctx, key); err != nil {
			log.Warn("Cache deletion failed",
				zap.String("key", key),
				zap.Error(err))
		}
	}()
}
