package application

import (
	"context"
	"encoding/json"

	"time"

	"github.com/davicafu/hexagolab/internal/user/domain"
	"go.uber.org/zap"
)

// OutboxWorker procesa eventos pendientes en la tabla outbox y los publica en Kafka.
type OutboxWorker struct {
	repo      domain.UserRepository
	publisher domain.EventPublisher
	interval  time.Duration
	batchSize int
	ch        chan domain.OutboxEvent
	log       *zap.Logger
}

func NewOutboxWorker(
	repo domain.UserRepository,
	publisher domain.EventPublisher,
	interval time.Duration,
	batchSize int,
	log *zap.Logger,
) *OutboxWorker {

	return &OutboxWorker{
		repo:      repo,
		publisher: publisher,
		interval:  interval,
		batchSize: batchSize,
		ch:        make(chan domain.OutboxEvent, 100),
		log:       log,
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
		for {
			select {
			case <-ctx.Done():
				w.log.Info("🛑 Outbox worker detenido (canal en memoria)")
				return
			case evt := <-w.ch:
				w.log.Info("Procesando evento desde canal en memoria",
					zap.String("event_id", evt.ID.String()),
					zap.String("aggregate_type", evt.AggregateType),
					zap.String("event_type", evt.EventType),
				)
				w.publishAndMark(ctx, evt)
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				w.log.Info("🛑 Outbox worker detenido (polling)")
				return
			case <-ticker.C:
				w.log.Info("🔄 Ejecutando polling de outbox")
				w.ProcessBatch(ctx)
			}
		}
	}()
}

func (w *OutboxWorker) ProcessBatch(ctx context.Context) {
	events, err := w.repo.FetchPendingOutbox(ctx, w.batchSize)
	if err != nil {
		w.log.Warn("⚠️ Error al obtener eventos pendientes", zap.Error(err))
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
		w.log.Warn("⚠️ No se pudo publicar evento",
			zap.String("event_id", evt.ID.String()),
			zap.String("event_type", evt.EventType),
			zap.Error(lastErr),
		)
		return
	}

	if err := w.repo.MarkOutboxProcessed(ctx, evt.ID); err != nil {
		w.log.Warn("⚠️ No se pudo marcar evento %s como procesado",
			zap.String("event_id", evt.ID.String()),
			zap.Error(err),
		)
	} else {
		w.log.Info("✅ Evento %s publicado y marcado como procesado",
			zap.String("event_id", evt.ID.String()),
			zap.String("event_type", evt.EventType),
		)
	}
}
