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
	ch        chan domain.OutboxEvent
}

func NewOutboxWorker(repo domain.UserRepository, publisher domain.EventPublisher, interval time.Duration, batchSize int) *OutboxWorker {
	return &OutboxWorker{
		repo:      repo,
		publisher: publisher,
		interval:  interval,
		batchSize: batchSize,
		ch:        make(chan domain.OutboxEvent, 100), // buffer configurable
	}
}

// Enqueue: se llama cuando guardas un evento en la DB
func (w *OutboxWorker) Enqueue(evt domain.OutboxEvent) {
	// Intentar enviar al canal para publicarlo "en caliente"
	select {
	case w.ch <- evt:
	default:
		// si está lleno, no importa: se procesará en el siguiente ciclo de polling
	}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	go func() {
		// Loop para consumir el canal en memoria
		for {
			select {
			case <-ctx.Done():
				log.Println("🛑 Outbox worker detenido (canal en memoria)")
				return
			case evt := <-w.ch:
				log.Printf("📥 Procesando evento %s desde canal en memoria", evt.ID)
				w.publishAndMark(ctx, evt)
			}
		}
	}()

	go func() {
		// Loop para polling periódico
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("🛑 Outbox worker detenido (polling)")
				return
			case <-ticker.C:
				log.Println("🔄 Ejecutando polling de outbox")
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
		w.publishAndMark(ctx, evt)
	}
}

func (w *OutboxWorker) publishAndMark(ctx context.Context, evt domain.OutboxEvent) {
	payloadBytes, _ := json.Marshal(evt.Payload)

	const maxRetries = 3
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := w.publisher.Publish(ctx, evt.EventType, payloadBytes); err != nil {
			lastErr = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		lastErr = nil
		break
	}

	if lastErr != nil {
		log.Printf("⚠️ No se pudo publicar evento %s: %v", evt.ID, lastErr)
		return
	}

	if err := w.repo.MarkOutboxProcessed(ctx, evt.ID); err != nil {
		log.Printf("⚠️ No se pudo marcar evento %s como procesado: %v", evt.ID, err)
	} else {
		log.Printf("✅ Evento %s publicado y marcado como procesado", evt.ID)
	}
}
