// en internal/task/infra/inbound/events/task_consumer.go
package events

import (
	"context"
	"encoding/json"
	"errors" // Necesario para la comprobación de errores
	"time"

	"go.uber.org/zap"

	taskDomain "github.com/davicafu/hexagolab/internal/task/domain"
	"github.com/google/uuid"

	// --- Importaciones compartidas ---
	sharedEvents "github.com/davicafu/hexagolab/internal/shared/domain/events"
	sharedUtils "github.com/davicafu/hexagolab/internal/shared/infra/utils"
)

// TaskService es la interfaz que define los métodos que el consumidor necesita.
type TaskService interface {
	CreateTask(ctx context.Context, title, description string, assigneeID uuid.UUID) (*taskDomain.Task, error)
	UpdateTask(ctx context.Context, t *taskDomain.Task) error
	GetTaskByID(ctx context.Context, id uuid.UUID) (*taskDomain.Task, error)
}

// TaskConsumer maneja la lógica para procesar eventos de Task.
type TaskConsumer struct {
	service TaskService
	log     *zap.Logger
}

// NewTaskConsumer es el constructor.
func NewTaskConsumer(service TaskService, logger *zap.Logger) *TaskConsumer {
	return &TaskConsumer{
		service: service,
		log:     logger,
	}
}

// HandleMessage es el punto de entrada para un nuevo mensaje/evento.
func (c *TaskConsumer) HandleMessage(ctx context.Context, key string, payload []byte) {
	var base sharedEvents.IntegrationEvent
	if err := json.Unmarshal(payload, &base); err != nil {
		c.log.Warn("Failed to unmarshal integration event for task", zap.String("key", key), zap.Error(err))
		return
	}

	// Usamos las constantes de eventos compartidas
	switch base.Type {
	case taskDomain.TaskCreated:
		sharedUtils.UnmarshalAndHandle[sharedEvents.TaskCreated](c.log, base.Data, func(evt sharedEvents.TaskCreated) {
			c.withContext(ctx, evt.ID, func(ctxTask context.Context) error {
				// LÓGICA DE IDEMPOTENCIA: "Buscar antes de Crear"
				_, err := c.service.GetTaskByID(ctxTask, evt.ID)
				if err == nil {
					c.log.Info("Evento 'TaskCreated' duplicado ignorado", zap.String("task_id", evt.ID.String()))
					return nil
				}
				if !errors.Is(err, taskDomain.ErrTaskNotFound) {
					return err
				}

				// Si no existe, lo creamos.
				_, err = c.service.CreateTask(ctxTask, evt.Title, evt.Description, evt.AssigneeID)
				return err
			}, "Task created via event", evt)
		})

	case taskDomain.TaskUpdated:
		sharedUtils.UnmarshalAndHandle[sharedEvents.TaskUpdated](c.log, base.Data, func(evt sharedEvents.TaskUpdated) {
			c.withContext(ctx, evt.ID, func(ctxTask context.Context) error {
				task, err := c.service.GetTaskByID(ctxTask, evt.ID)
				if err != nil {
					return err
				}
				// Aplicamos los cambios del evento a la entidad
				task.Title = evt.Title
				task.Description = evt.Description
				task.Status = taskDomain.TaskStatus(evt.Status)
				task.UpdatedAt = time.Now().UTC()
				return c.service.UpdateTask(ctxTask, task)
			}, "Task updated via event", evt)
		})

	default:
		c.log.Warn("Unknown task event type", zap.String("type", base.Type), zap.String("key", key))
	}
}

// Helper para ejecutar acción con contexto limitado y log.
func (c *TaskConsumer) withContext(ctx context.Context, id uuid.UUID, action func(ctx context.Context) error, successMsg string, evt interface{}) {
	ctxTask, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	if err := action(ctxTask); err != nil {
		// Alternativa de idempotencia: si el error es que ya existe, lo tratamos como un éxito.
		if errors.Is(err, taskDomain.ErrTaskAlreadyExists) {
			c.log.Info("Evento 'TaskCreated' duplicado gestionado por la BBDD", zap.String("task_id", id.String()))
			return
		}

		c.log.Warn("Failed to process task event",
			zap.String("task_id", id.String()),
			zap.Any("event", evt),
			zap.Error(err),
		)
	} else {
		c.log.Info(successMsg,
			zap.String("task_id", id.String()),
			zap.Any("event", evt),
		)
	}
}

// BackgroundConsumerChan inicia una goroutine para consumir eventos de un canal.
func BackgroundConsumerChan(ctx context.Context, ch <-chan interface{}, consumer *TaskConsumer) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				consumer.log.Info("TaskConsumer stopped")
				return
			case msg := <-ch:
				// Hacemos una aserción de tipo para asegurarnos de que es un []byte
				if payload, ok := msg.([]byte); ok {
					// La 'key' no es relevante en el bus en memoria, pasamos una vacía.
					consumer.HandleMessage(ctx, "", payload)
				}
			}
		}
	}()
}
