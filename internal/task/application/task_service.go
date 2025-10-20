// en internal/task/application/task_service.go
package application

import (
	"context"
	"errors"
	"time"

	// --- Importaciones del dominio y compartidas ---
	sharedDomain "github.com/davicafu/hexagolab/internal/shared/domain"
	sharedCache "github.com/davicafu/hexagolab/internal/shared/infra/platform/cache"
	sharedQuery "github.com/davicafu/hexagolab/internal/shared/infra/platform/query"
	sharedUtils "github.com/davicafu/hexagolab/internal/shared/infra/utils"
	taskDomain "github.com/davicafu/hexagolab/internal/task/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TaskService define los casos de uso relacionados con Task.
// Incorpora repositorio, caché y logger.
type TaskService struct {
	repo  taskDomain.TaskRepository
	cache sharedCache.Cache
	log   *zap.Logger
}

// NewTaskService es el constructor para el servicio de tareas.
func NewTaskService(repo taskDomain.TaskRepository, cache sharedCache.Cache, log *zap.Logger) *TaskService {
	return &TaskService{
		repo:  repo,
		cache: cache,
		log:   log,
	}
}

// CreateTask crea una nueva tarea, su evento de outbox y actualiza la caché.
func (s *TaskService) CreateTask(ctx context.Context, title, description string, assigneeID uuid.UUID) (*taskDomain.Task, error) {
	task := &taskDomain.Task{
		ID:          uuid.New(),
		Title:       title,
		Description: description,
		AssigneeID:  assigneeID,
		Status:      taskDomain.TaskPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	outboxEvent := sharedDomain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "task",
		AggregateID:   task.ID.String(),
		EventType:     taskDomain.TaskCreated,
		Payload:       task, // El payload es la entidad completa
		CreatedAt:     time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, task, outboxEvent); err != nil {
		s.log.Error("Failed to create task", zap.Error(err))
		return nil, err
	}

	// Actualizar caché en segundo plano
	sharedCache.AsyncCacheSet(ctx, s.cache, taskDomain.TaskCacheKeyByID(task.ID), task, 60, s.log)

	return task, nil
}

// UpdateTask actualiza una tarea, crea un evento y actualiza la caché.
func (s *TaskService) UpdateTask(ctx context.Context, t *taskDomain.Task) error {
	evt := sharedDomain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "task",
		AggregateID:   t.ID.String(),
		EventType:     taskDomain.TaskUpdated,
		Payload:       t,
		CreatedAt:     time.Now().UTC(),
	}

	if err := s.repo.Update(ctx, t, evt); err != nil {
		return err
	}

	// Actualizar caché en segundo plano
	sharedCache.AsyncCacheSet(ctx, s.cache, taskDomain.TaskCacheKeyByID(t.ID), t, 60, s.log)

	return nil
}

// DeleteTask elimina una tarea, crea un evento y limpia la caché.
func (s *TaskService) DeleteTask(ctx context.Context, id uuid.UUID) error {
	evt := sharedDomain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "task",
		AggregateID:   id.String(),
		EventType:     taskDomain.TaskDeleted,
		Payload:       map[string]interface{}{"id": id.String()},
		CreatedAt:     time.Now().UTC(),
	}

	if err := s.repo.DeleteByID(ctx, id, evt); err != nil {
		return err
	}

	// Eliminar de la caché en segundo plano
	sharedCache.AsyncCacheDelete(ctx, s.cache, taskDomain.TaskCacheKeyByID(id), s.log)

	return nil
}

// GetTaskByID obtiene una tarea, usando el patrón cache-aside con reintentos.
func (s *TaskService) GetTaskByID(ctx context.Context, id uuid.UUID) (*taskDomain.Task, error) {
	// 1. Intentar obtener de la caché
	if s.cache != nil {
		var t taskDomain.Task
		if hit, _ := s.cache.Get(ctx, taskDomain.TaskCacheKeyByID(id), &t); hit {
			return &t, nil
		}
	}

	// 2. Si es 'miss', ir al repositorio con reintentos
	var task *taskDomain.Task
	err := sharedUtils.Retry(ctx, 3, 100*time.Millisecond, func() error {
		var errRetry error
		task, errRetry = s.repo.GetByID(ctx, id)
		return errRetry
	})

	if err != nil {
		if errors.Is(err, taskDomain.ErrTaskNotFound) {
			s.log.Warn("Task not found", zap.String("task_id", id.String()))
		} else {
			s.log.Error("Failed to fetch task", zap.String("task_id", id.String()), zap.Error(err))
		}
		return nil, err
	}

	// 3. Actualizar caché en segundo plano para la próxima vez
	if s.cache != nil {
		go func(t *taskDomain.Task) {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			if err := s.cache.Set(cacheCtx, taskDomain.TaskCacheKeyByID(t.ID), t, 120); err != nil {
				s.log.Warn("⚠️ Cache update failed for task",
					zap.String("task_id", t.ID.String()),
					zap.Error(err),
				)
			}
		}(task)
	}

	return task, nil
}

// ListTasks es un pass-through al repositorio para listados genéricos.
func (s *TaskService) ListTasks(ctx context.Context, criteria sharedDomain.Criteria, pagination sharedQuery.Pagination, sorts sharedQuery.Sort) ([]*taskDomain.Task, error) {
	return s.repo.ListByCriteria(ctx, criteria, pagination, sorts)
}

func (s *TaskService) ListPendingTasksForUser(ctx context.Context, userID uuid.UUID, pagination sharedQuery.Pagination, sorts sharedQuery.Sort) ([]*taskDomain.Task, error) {
	criteria := sharedDomain.And(
		taskDomain.StatusCriteria{Status: taskDomain.TaskPending},
		taskDomain.AssigneeIDCriteria{ID: userID},
	)
	return s.repo.ListByCriteria(ctx, criteria, pagination, sorts)
}

func (s *TaskService) ListCompletedTasksForUser(ctx context.Context, userID uuid.UUID, pagination sharedQuery.Pagination, sorts sharedQuery.Sort) ([]*taskDomain.Task, error) {
	criteria := sharedDomain.And(
		taskDomain.StatusCriteria{Status: taskDomain.TaskCompleted},
		taskDomain.AssigneeIDCriteria{ID: userID},
	)
	return s.repo.ListByCriteria(ctx, criteria, pagination, sorts)
}
