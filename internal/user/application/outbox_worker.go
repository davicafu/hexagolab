package application

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/davicafu/hexagolab/internal/user/domain"
)

// OutboxWorker procesa eventos pendientes en la tabla outbox y los publica en Kafka.
type OutboxWorker struct {
	repo      domain.UserRepository
	publisher domain.EventPublisher
	interval  time.Duration
	batchSize int
}

// NewOutboxWorker crea un nuevo worker
func NewOutboxWorker(repo domain.UserRepository, publisher domain.EventPublisher, interval time.Duration, batchSize int) *OutboxWorker {
	return &OutboxWorker{
		repo:      repo,
		publisher: publisher,
		interval:  interval,
		batchSize: batchSize,
	}
}

// Start inicia el worker en background. Cancela con ctx.
func (w *OutboxWorker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("🛑 Outbox worker detenido")
				return
			case <-ticker.C:
				w.ProcessBatch(ctx)
			}
		}
	}()
}

func (w *OutboxWorker) ProcessBatch(ctx context.Context) {
	events, err := w.repo.FetchPendingOutbox(ctx, w.batchSize)
	if err != nil {
		log.Printf("⚠️ Error al obtener eventos pendientes: %v", err)
		return
	}

	for _, evt := range events {
		payloadBytes, _ := json.Marshal(evt.Payload)

		// Publicar con reintentos
		const maxRetries = 3
		var lastErr error
		for i := 0; i < maxRetries; i++ {
			if err := w.publisher.Publish(ctx, evt.EventType, payloadBytes); err != nil {
				lastErr = err
				time.Sleep(50 * time.Millisecond) // backoff corto
				continue
			}
			lastErr = nil
			break
		}

		if lastErr != nil {
			log.Printf("⚠️ No se pudo publicar evento %s: %v", evt.ID, lastErr)
			continue
		}

		// Marcar como procesado
		if err := w.repo.MarkOutboxProcessed(ctx, evt.ID); err != nil {
			log.Printf("⚠️ No se pudo marcar evento %s como procesado: %v", evt.ID, err)
		} else {
			log.Printf("✅ Evento %s publicado y marcado como procesado", evt.ID)
		}
	}
}
