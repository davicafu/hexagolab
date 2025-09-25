package domain

import (
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
