package utils

import (
	"context"
	"time"
)

// Retry ejecuta una función con reintentos configurables
func Retry(ctx context.Context, attempts int, delay time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		select {
		case <-time.After(delay):
			// espera antes del siguiente intento
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}
