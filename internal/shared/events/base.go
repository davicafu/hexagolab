package events

import (
	"encoding/json"
	"time"
)

// Base de todos los eventos de integración
type IntegrationEvent struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"` // contenido específico del evento
}
