package persistence

import (
	context "context"

	"github.com/davicafu/hexagolab/shared/domain"
	"github.com/google/uuid"
)

type OutboxRepository interface {
	FetchPendingOutbox(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error
}
