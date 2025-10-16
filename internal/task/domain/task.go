package domain

import (
	"time"

	sharedBus "github.com/davicafu/hexagolab/shared/platform/bus"
	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
)

type Task struct {
	ID          uuid.UUID
	Title       string
	Description string
	AssigneeID  uuid.UUID
	Status      TaskStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (t *Task) PartitionKey() string {
	return t.ID.String()
}

// --- Métodos de dominio ---
func (t *Task) Complete() {
	t.Status = TaskCompleted
	t.UpdatedAt = time.Now()
}

func (t *Task) Fail() {
	t.Status = TaskFailed
	t.UpdatedAt = time.Now()
}

func (t *Task) Update(title, description string) {
	t.Title = title
	t.Description = description
	t.UpdatedAt = time.Now()
}

// Verificación estática para asegurar que User implementa la interfaz
var _ sharedBus.Keyer = (*Task)(nil)
