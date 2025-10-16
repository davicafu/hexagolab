// en internal/task/application/task_service_test.go
package application

import (
	"context"
	"testing"
	"time"

	taskDomain "github.com/davicafu/hexagolab/internal/task/domain"
	sharedDomain "github.com/davicafu/hexagolab/shared/domain"
	sharedQuery "github.com/davicafu/hexagolab/shared/platform/query"
	"github.com/davicafu/hexagolab/tests/mocks" // Importamos nuestros mocks/fakes
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// Nota: El InMemoryCache se definió en el ejemplo anterior.
// Asegúrate de que esté accesible para este test, ya sea en el mismo
// paquete de test o en el paquete 'mocks'.

func TestCreateTask_Success(t *testing.T) {
	// Arrange
	repo := mocks.NewInMemoryTaskRepo()
	cache := &mocks.DummyCache{}
	service := NewTaskService(repo, cache, zap.NewNop())
	assigneeID := uuid.New()

	// Act
	task, err := service.CreateTask(context.Background(), "Mi primera tarea", "Hacer algo importante", assigneeID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, "Mi primera tarea", task.Title)
	assert.Equal(t, taskDomain.TaskPending, task.Status)

	// Verificar que se creó un evento Outbox
	assert.Len(t, repo.Outbox, 1)
	assert.Equal(t, "task.created", repo.Outbox[0].EventType)
	assert.Equal(t, task.ID.String(), repo.Outbox[0].AggregateID)
}

func TestGetTask_NotFound(t *testing.T) {
	// Arrange
	repo := mocks.NewInMemoryTaskRepo()
	cache := &mocks.DummyCache{}
	service := NewTaskService(repo, cache, zap.NewNop())

	// Act
	_, err := service.GetTaskByID(context.Background(), uuid.New())

	// Assert
	assert.ErrorIs(t, err, taskDomain.ErrTaskNotFound)
}

func TestUpdateTask_Success(t *testing.T) {
	// Arrange
	repo := mocks.NewInMemoryTaskRepo()
	cache := &mocks.DummyCache{}
	service := NewTaskService(repo, cache, zap.NewNop())

	task, _ := service.CreateTask(context.Background(), "Tarea original", "desc", uuid.New())
	task.Title = "Título actualizado"
	task.Complete() // Usamos el método de dominio

	// Act
	err := service.UpdateTask(context.Background(), task)

	// Assert
	assert.NoError(t, err)

	// Comprobar que se actualizó en el repo
	updatedTask, _ := repo.GetByID(context.Background(), task.ID)
	assert.Equal(t, "Título actualizado", updatedTask.Title)
	assert.Equal(t, taskDomain.TaskCompleted, updatedTask.Status)

	// Verificar que se creó un evento Outbox adicional
	assert.Len(t, repo.Outbox, 2) // 1 de create, 1 de update
	assert.Equal(t, "task.updated", repo.Outbox[1].EventType)
}

func TestDeleteTask_Success(t *testing.T) {
	// Arrange
	repo := mocks.NewInMemoryTaskRepo()
	cache := &mocks.DummyCache{}
	service := NewTaskService(repo, cache, zap.NewNop())
	task, _ := service.CreateTask(context.Background(), "Tarea a borrar", "desc", uuid.New())

	// Act
	err := service.DeleteTask(context.Background(), task.ID)

	// Assert
	assert.NoError(t, err)

	// Verificar que la tarea fue eliminada del repo
	_, err = repo.GetByID(context.Background(), task.ID)
	assert.ErrorIs(t, err, taskDomain.ErrTaskNotFound)

	// Verificar que se creó un evento Outbox de eliminación
	assert.Len(t, repo.Outbox, 2)
	assert.Equal(t, "task.deleted", repo.Outbox[1].EventType)
}

// -------------------- GetTask con Cache --------------------

func TestGetTask_CacheHit(t *testing.T) {
	// Arrange
	taskID := uuid.New()
	task := &taskDomain.Task{ID: taskID, Title: "Tarea en caché"}

	// Pre-populamos la caché directamente
	repo := mocks.NewInMemoryTaskRepo()
	cache := &mocks.DummyCache{}
	cache.Set(context.Background(), taskDomain.TaskCacheKeyByID(taskID), task, 60)

	service := NewTaskService(repo, cache, zap.NewNop())

	// Act
	fetchedTask, err := service.GetTaskByID(context.Background(), taskID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, fetchedTask)
	assert.Equal(t, "Tarea en caché", fetchedTask.Title)
}

func TestGetTask_CacheMiss(t *testing.T) {
	// Arrange
	taskID := uuid.New()
	task := &taskDomain.Task{ID: taskID, Title: "Tarea en repo"}

	repo := mocks.NewInMemoryTaskRepo()
	repo.Create(context.Background(), task, sharedDomain.OutboxEvent{}) // Pre-populamos el repo
	cache := &mocks.DummyCache{}                                        // La caché está vacía

	service := NewTaskService(repo, cache, zap.NewNop())

	// Act
	fetchedTask, err := service.GetTaskByID(context.Background(), taskID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, fetchedTask)
	assert.Equal(t, task.ID, fetchedTask.ID)

	// Verificar que la caché se ha actualizado
	var cachedTask taskDomain.Task
	hit, _ := cache.Get(context.Background(), taskDomain.TaskCacheKeyByID(taskID), &cachedTask)
	assert.True(t, hit, "La caché debería haberse populado tras el 'miss'")
}

// ----------------- ListTasks / Search / Filter -----------------

func TestListPendingTasksForUser_Filtering(t *testing.T) {
	// Arrange
	repo := mocks.NewInMemoryTaskRepo()
	service := NewTaskService(repo, nil, zap.NewNop())
	userA := uuid.New()
	userB := uuid.New()

	// Creamos un escenario mixto de tareas
	repo.Create(context.Background(), &taskDomain.Task{ID: uuid.New(), AssigneeID: userA, Status: taskDomain.TaskPending}, sharedDomain.OutboxEvent{})
	repo.Create(context.Background(), &taskDomain.Task{ID: uuid.New(), AssigneeID: userA, Status: taskDomain.TaskCompleted}, sharedDomain.OutboxEvent{})
	repo.Create(context.Background(), &taskDomain.Task{ID: uuid.New(), AssigneeID: userB, Status: taskDomain.TaskPending}, sharedDomain.OutboxEvent{})

	// Act: Usamos el método específico del servicio
	results, err := service.ListPendingTasksForUser(
		context.Background(),
		userA,
		sharedQuery.OffsetPagination{Limit: 10, Offset: 0},
		sharedQuery.Sort{},
	)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, userA, results[0].AssigneeID)
	assert.Equal(t, taskDomain.TaskPending, results[0].Status)
}

func TestListTasks_PaginationAndSorting(t *testing.T) {
	// Arrange
	repo := mocks.NewInMemoryTaskRepo()
	service := NewTaskService(repo, nil, zap.NewNop())

	// Creamos 5 tareas para probar
	tasks := []*taskDomain.Task{
		{ID: uuid.New(), Title: "A - Tarea Alfa", CreatedAt: time.Now().Add(-5 * time.Hour)},
		{ID: uuid.New(), Title: "B - Tarea Beta", CreatedAt: time.Now().Add(-4 * time.Hour)},
		{ID: uuid.New(), Title: "C - Tarea Gamma", CreatedAt: time.Now().Add(-3 * time.Hour)},
		{ID: uuid.New(), Title: "D - Tarea Delta", CreatedAt: time.Now().Add(-2 * time.Hour)},
		{ID: uuid.New(), Title: "E - Tarea Epsilon", CreatedAt: time.Now().Add(-1 * time.Hour)},
	}
	for _, task := range tasks {
		repo.Create(context.Background(), task, sharedDomain.OutboxEvent{})
	}

	criteria := sharedDomain.CompositeCriteria{} // Sin filtro

	// --- 1. Paginación: Offset + Limit ---
	page1, err := service.ListTasks(
		context.Background(),
		criteria,
		sharedQuery.OffsetPagination{Limit: 2, Offset: 0},
		sharedQuery.Sort{Field: "title", Desc: false},
	)
	assert.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.Equal(t, "A - Tarea Alfa", page1[0].Title)
	assert.Equal(t, "B - Tarea Beta", page1[1].Title)

	page2, err := service.ListTasks(
		context.Background(),
		criteria,
		sharedQuery.OffsetPagination{Limit: 2, Offset: 2},
		sharedQuery.Sort{Field: "title", Desc: false},
	)
	assert.NoError(t, err)
	assert.Len(t, page2, 2)
	assert.Equal(t, "C - Tarea Gamma", page2[0].Title)
	assert.Equal(t, "D - Tarea Delta", page2[1].Title)

	// --- 2. Orden descendente ---
	descTasks, err := service.ListTasks(
		context.Background(),
		criteria,
		sharedQuery.OffsetPagination{Limit: 5, Offset: 0},
		sharedQuery.Sort{Field: "title", Desc: true},
	)
	assert.NoError(t, err)
	assert.Equal(t, "E - Tarea Epsilon", descTasks[0].Title)
	assert.Equal(t, "A - Tarea Alfa", descTasks[4].Title)
}
