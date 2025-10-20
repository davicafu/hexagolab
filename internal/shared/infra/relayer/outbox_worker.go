package relayer

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	sharedDomain "github.com/davicafu/hexagolab/internal/shared/domain"
	sharedDomainEvents "github.com/davicafu/hexagolab/internal/shared/domain/events"
	sharedBus "github.com/davicafu/hexagolab/internal/shared/infra/platform/bus"
	"go.uber.org/zap"
)

// Worker procesa eventos pendientes de la tabla outbox de forma genérica.
type Worker struct {
	repo          sharedDomain.OutboxRepository
	publisher     sharedBus.EventBus
	eventRegistry map[string]sharedDomainEvents.EventMetadata
	interval      time.Duration
	batchSize     int
	log           *zap.Logger
}

func NewOutboxWorker(
	repo sharedDomain.OutboxRepository,
	publisher sharedBus.EventBus,
	registry map[string]sharedDomainEvents.EventMetadata,
	interval time.Duration,
	batchSize int,
	log *zap.Logger,
) *Worker {
	return &Worker{
		repo:          repo,
		publisher:     publisher,
		eventRegistry: registry,
		interval:      interval,
		batchSize:     batchSize,
		log:           log,
	}
}

// Start inicia el bucle de polling del worker.
func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.log.Info("🚀 Outbox worker iniciado", zap.Duration("interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			w.log.Info("🛑 Outbox worker detenido.")
			return
		case <-ticker.C:
			w.log.Info("🔄 Ejecutando polling de outbox")
			w.ProcessBatch(ctx)
		}
	}
}

func (w *Worker) ProcessBatch(ctx context.Context) {
	events, err := w.repo.FetchPendingOutbox(ctx, w.batchSize)
	if err != nil {
		w.log.Warn("⚠️ Error al obtener eventos pendientes", zap.Error(err))
		return
	}
	if len(events) > 0 {
		w.log.Info(fmt.Sprintf("📬 %d eventos encontrados para procesar", len(events)))
	}

	for _, evt := range events {
		w.publishAndMark(ctx, evt)
	}
}

func (w *Worker) publishAndMark(ctx context.Context, evt sharedDomain.OutboxEvent) {
	// 1. Usar el registro para decodificar el payload al tipo de evento correcto
	metadata, ok := w.eventRegistry[evt.EventType]
	if !ok {
		w.log.Error("Tipo de evento desconocido en registro", zap.String("event_type", evt.EventType))
		// Opcional: Marcar como procesado para no reintentar indefinidamente
		// w.repo.MarkOutboxProcessed(ctx, evt.ID)
		return
	}

	// Creamos una nueva instancia del tipo de evento (ej: &userDomain.User{})
	eventPayload := reflect.New(metadata.Type).Interface()

	payloadBytes, _ := json.Marshal(evt.Payload)
	if err := json.Unmarshal(payloadBytes, eventPayload); err != nil {
		w.log.Error("Error al decodificar payload del evento", zap.String("event_id", evt.ID.String()), zap.Error(err))
		return
	}

	// 2. Publicar el evento fuertemente tipado
	if err := w.publisher.Publish(ctx, eventPayload); err != nil {
		w.log.Warn("⚠️ No se pudo publicar evento",
			zap.String("event_id", evt.ID.String()),
			zap.Error(err),
		)
		return // No lo marcamos como procesado para que se reintente
	}

	// 3. Marcar como procesado en la DB
	if err := w.repo.MarkOutboxProcessed(ctx, evt.ID); err != nil {
		w.log.Warn("⚠️ No se pudo marcar evento como procesado",
			zap.String("event_id", evt.ID.String()),
			zap.Error(err),
		)
	} else {
		w.log.Info("✅ Evento publicado y marcado", zap.String("event_id", evt.ID.String()))
	}
}
