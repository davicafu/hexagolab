package persistence

import (
	"context"

	"github.com/davicafu/hexagolab/shared/domain" // Asumiendo que OutboxEvent está aquí
	"github.com/google/uuid"
)

// OutboxRepository define el contrato para acceder a la tabla outbox.
// Es la única dependencia que tendrá el worker.
type OutboxRepository interface {
	FetchPendingOutbox(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error
}
