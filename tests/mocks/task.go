package mocks

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	sharedDomain "github.com/davicafu/hexagolab/internal/shared/domain"
	sharedQuery "github.com/davicafu/hexagolab/internal/shared/infra/platform/query"
	taskDomain "github.com/davicafu/hexagolab/internal/task/domain"
	"github.com/google/uuid"
)

// InMemoryTaskRepo simula TaskRepository con outbox incluido.
type InMemoryTaskRepo struct {
	Tasks  map[uuid.UUID]*taskDomain.Task
	Outbox []sharedDomain.OutboxEvent
	mu     sync.Mutex
}

func NewInMemoryTaskRepo() *InMemoryTaskRepo {
	return &InMemoryTaskRepo{
		Tasks:  make(map[uuid.UUID]*taskDomain.Task),
		Outbox: []sharedDomain.OutboxEvent{},
	}
}

// --- Implementación de la interfaz TaskRepository ---

func (r *InMemoryTaskRepo) Create(ctx context.Context, t *taskDomain.Task, evt sharedDomain.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Tasks[t.ID]; ok {
		return taskDomain.ErrTaskAlreadyExists
	}
	r.Tasks[t.ID] = t
	r.Outbox = append(r.Outbox, evt)
	return nil
}

func (r *InMemoryTaskRepo) GetByID(ctx context.Context, id uuid.UUID) (*taskDomain.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.Tasks[id]
	if !ok {
		return nil, taskDomain.ErrTaskNotFound
	}
	return t, nil
}

func (r *InMemoryTaskRepo) Update(ctx context.Context, t *taskDomain.Task, evt sharedDomain.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Tasks[t.ID]; !ok {
		return taskDomain.ErrTaskNotFound
	}
	r.Tasks[t.ID] = t
	r.Outbox = append(r.Outbox, evt)
	return nil
}

func (r *InMemoryTaskRepo) DeleteByID(ctx context.Context, id uuid.UUID, evt sharedDomain.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Tasks[id]; !ok {
		return taskDomain.ErrTaskNotFound
	}
	delete(r.Tasks, id)
	r.Outbox = append(r.Outbox, evt)
	return nil
}

func (r *InMemoryTaskRepo) ListByCriteria(
	ctx context.Context,
	criteria sharedDomain.Criteria,
	pagination sharedQuery.Pagination,
	sorts sharedQuery.Sort,
) ([]*taskDomain.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var list []*taskDomain.Task
	for _, task := range r.Tasks {
		if criteria == nil || matchTaskCriterion(task, criteria.ToConditions()) {
			list = append(list, task)
		}
	}

	// Ordenar
	s := sorts
	sort.SliceStable(list, func(i, j int) bool {
		return compareTasks(list[i], list[j], s.Field, s.Desc)
	})

	// Paginar
	if p, ok := pagination.(sharedQuery.OffsetPagination); ok {
		start := p.Offset
		if start > len(list) {
			return []*taskDomain.Task{}, nil
		}
		end := start + p.Limit
		if end > len(list) {
			end = len(list)
		}
		return list[start:end], nil
	}

	return list, nil // Devuelve sin paginar si no es OffsetPagination
}

// --- Lógica de filtrado y ordenamiento del mock ---

func matchTaskCriterion(t *taskDomain.Task, conds []sharedDomain.Criterion) bool {
	for _, cond := range conds {
		field := strings.ToLower(cond.Field)
		op := strings.ToUpper(string(cond.Op))
		val := cond.Value

		var match bool
		switch field {
		case "status":
			match = string(t.Status) == fmt.Sprintf("%v", val)
		case "assignee_id":
			assigneeID, ok := val.(uuid.UUID)
			match = ok && t.AssigneeID == assigneeID
		case "title":
			title, ok := val.(string)
			if ok && (op == "ILIKE" || op == "LIKE") {
				pattern := strings.Trim(title, "%")
				match = strings.Contains(strings.ToLower(t.Title), strings.ToLower(pattern))
			}
		case "created_at":
			valTime, ok := val.(time.Time)
			if ok {
				if op == ">=" {
					match = t.CreatedAt.After(valTime) || t.CreatedAt.Equal(valTime)
				}
				if op == "<=" {
					match = t.CreatedAt.Before(valTime) || t.CreatedAt.Equal(valTime)
				}
			}
		}

		if !match {
			return false // Si una condición no coincide, el registro no pasa el filtro
		}
	}
	return true
}

func compareTasks(t1, t2 *taskDomain.Task, field string, desc bool) bool {
	var result bool
	switch strings.ToLower(field) {
	case "title":
		result = t1.Title < t2.Title
	case "status":
		result = t1.Status < t2.Status
	case "created_at":
		result = t1.CreatedAt.Before(t2.CreatedAt)
	default: // Orden por defecto
		result = t1.ID.String() < t2.ID.String()
	}
	if desc {
		return !result
	}
	return result
}

// --- Métodos de Outbox del mock ---

func (r *InMemoryTaskRepo) FetchPendingOutbox(ctx context.Context, limit int) ([]sharedDomain.OutboxEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Simulación simple: devuelve los primeros N eventos sin procesar
	var pending []sharedDomain.OutboxEvent
	for _, evt := range r.Outbox {
		// En un mock real, podrías añadir un campo `Processed` si fuera necesario
		pending = append(pending, evt)
		if len(pending) == limit {
			break
		}
	}
	return pending, nil
}

func (r *InMemoryTaskRepo) MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, evt := range r.Outbox {
		if evt.ID == id {
			// Eliminar de outbox para simular que se procesó
			r.Outbox = append(r.Outbox[:i], r.Outbox[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("outbox event not found: %s", id) // Error genérico
}
