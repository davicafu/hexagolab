package events

import (
	"github.com/google/uuid"
)

type TaskCreated struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	AssigneeID  uuid.UUID `json:"assigneeId"`
}

type TaskUpdated struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
}
