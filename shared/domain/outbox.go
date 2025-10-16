package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// OutboxEvent representa un evento pendiente de publicar en el broker.
type OutboxEvent struct {
	ID            uuid.UUID   `json:"id"`
	AggregateType string      `json:"aggregate_type"` // ej. "user", "car", "task"
	AggregateID   string      `json:"aggregate_id"`
	EventType     string      `json:"event_type"` // ej. "user.updated"
	Payload       interface{} `json:"payload"`    // JSON serializable
	CreatedAt     time.Time   `json:"created_at"`
	Processed     bool        `json:"processed"` // si ya se publicó
}

// OutboxRepository define el contrato para acceder a la tabla outbox.
// Es una interfaz más pequeña que la de un repositorio de dominio completo,
// conteniendo solo los métodos que el worker necesita.
type OutboxRepository interface {
	FetchPendingOutbox(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error
}
