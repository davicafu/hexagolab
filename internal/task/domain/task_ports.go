package domain

import (
	"context"
	"fmt"
	"time"

	"errors"

	sharedDomain "github.com/davicafu/hexagolab/internal/shared/domain"
	sharedQuery "github.com/davicafu/hexagolab/internal/shared/infra/platform/query"
	"github.com/google/uuid"
)

var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskAlreadyExists  = errors.New("task already exists")
	ErrInvalidTask        = errors.New("invalid task")
	ErrTaskCannotComplete = errors.New("task cannot be marked as completed")
)

// --- Repositorio de Tasks ---
type TaskRepository interface {
	Create(ctx context.Context, t *Task, evt sharedDomain.OutboxEvent) error
	Update(ctx context.Context, t *Task, evt sharedDomain.OutboxEvent) error
	GetByID(ctx context.Context, id uuid.UUID) (*Task, error)
	ListByCriteria(ctx context.Context, criteria sharedDomain.Criteria, pagination sharedQuery.Pagination, sort sharedQuery.Sort) ([]*Task, error)
	DeleteByID(ctx context.Context, id uuid.UUID, evt sharedDomain.OutboxEvent) error
}

// DTO para transportar los resultados de la consulta de tendencia.
type DailyTaskTrend struct {
	Day            time.Time
	CreatedCount   int
	CompletedCount int
}

type TaskAnalyticsRepository interface {
	LogBatch(ctx context.Context, tasks []*Task) error
	GetAverageCompletionTime(ctx context.Context, start, end time.Time) (time.Duration, error)
	GetDailyTrend(ctx context.Context, start, end time.Time) ([]DailyTaskTrend, error)
}

// ---------- Helpers comunes (cache keys, etc.) ----------

// Esto sí estaría bien dentro de task_ports.go
func TaskCacheKeyByID(id uuid.UUID) string {
	return fmt.Sprintf("task:id:%s", id.String())
}
