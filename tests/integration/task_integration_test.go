package integration

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	// --- Importaciones del dominio Task ---
	taskDomain "github.com/davicafu/hexagolab/internal/task/domain"
	infraTask "github.com/davicafu/hexagolab/internal/task/infra/outbound/db/postgre"

	// --- Importaciones compartidas ---
	sharedDomain "github.com/davicafu/hexagolab/shared/domain"
	sharedQuery "github.com/davicafu/hexagolab/shared/platform/query"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Driver de PostgreSQL
	_ "github.com/jackc/pgx/v5/stdlib"
)

// setupPostgresTestDB se conecta a Postgres, crea el esquema y limpia las tablas.
func setupPostgresTestDB(t *testing.T) *sql.DB {
	// Lee la cadena de conexión desde una variable de entorno
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		t.Skip("DATABASE_URL no está configurada, saltando test de integración con Postgres")
	}

	db, err := sql.Open("pgx", connStr)
	require.NoError(t, err)
	require.NoError(t, db.Ping())

	// Crear el esquema de la tabla de tareas (adaptado para Postgres)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id UUID PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT,
			assignee_id UUID,
			status TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL
		)
	`)
	require.NoError(t, err)

	// Crear el esquema de la tabla de outbox (adaptado para Postgres)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS outbox (
			id UUID PRIMARY KEY,
			aggregate_type TEXT NOT NULL,
			aggregate_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			payload JSONB NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			processed BOOLEAN NOT NULL DEFAULT FALSE
		)
	`)
	require.NoError(t, err)

	// ❗ MUY IMPORTANTE: Limpiar las tablas antes de cada test para asegurar el aislamiento
	_, err = db.Exec(`TRUNCATE TABLE tasks, outbox RESTART IDENTITY`)
	require.NoError(t, err)

	return db
}

// verifyOutboxEventPostgres es el helper adaptado para la sintaxis de Postgres.
func verifyOutboxEventPostgres(t *testing.T, db *sql.DB, aggregateID string, expectedEventType string, expectedCount int) {
	var count int
	// Se usa $1 como placeholder en lugar de ?
	err := db.QueryRow("SELECT COUNT(*) FROM outbox WHERE aggregate_id = $1", aggregateID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, expectedCount, count, "el número de eventos en outbox no es el esperado")

	var eventType string
	err = db.QueryRow("SELECT event_type FROM outbox WHERE aggregate_id = $1 ORDER BY created_at DESC LIMIT 1", aggregateID).Scan(&eventType)
	require.NoError(t, err)
	assert.Equal(t, expectedEventType, eventType, "el tipo de evento no coincide")
}

func TestTaskPostgresIntegration_CRUD(t *testing.T) {
	db := setupPostgresTestDB(t)
	defer db.Close()

	// ❗ Usamos la implementación del repositorio para Postgres
	repo := infraTask.NewTaskRepoPostgres(db)
	ctx := context.Background()
	assigneeID := uuid.New()

	// --- 1. Crear Tarea y su evento ---
	task := &taskDomain.Task{
		ID:          uuid.New(),
		Title:       "Tarea de integración en Postgres",
		Description: "Descripción inicial",
		AssigneeID:  assigneeID,
		Status:      taskDomain.TaskPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	createdEvent := sharedDomain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "Task",
		AggregateID:   task.ID.String(),
		EventType:     "TaskCreated",
		Payload:       map[string]interface{}{"title": task.Title, "assigneeId": task.AssigneeID.String()},
		CreatedAt:     time.Now().UTC(),
	}

	err := repo.Create(ctx, task, createdEvent)
	require.NoError(t, err)
	// Verificar que el evento se ha creado en la tabla outbox
	verifyOutboxEventPostgres(t, db, task.ID.String(), "TaskCreated", 1)

	// --- 2. Obtener Tarea (Read) ---
	got, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.Title, got.Title)
	assert.Equal(t, taskDomain.TaskPending, got.Status)

	// --- 3. Actualizar Tarea y su evento ---
	task.Complete()
	task.Title = "Tarea completada en Postgres"
	updatedEvent := sharedDomain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "Task",
		AggregateID:   task.ID.String(),
		EventType:     "TaskUpdated",
		Payload:       map[string]interface{}{"status": string(task.Status)},
		CreatedAt:     time.Now().UTC(),
	}

	err = repo.Update(ctx, task, updatedEvent)
	require.NoError(t, err)
	got, err = repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, "Tarea completada en Postgres", got.Title)
	assert.Equal(t, taskDomain.TaskCompleted, got.Status)
	verifyOutboxEventPostgres(t, db, task.ID.String(), "TaskUpdated", 2)

	// --- 4. Listar Tareas por Criterio ---
	tasks, err := repo.ListByCriteria(
		ctx,
		sharedDomain.CompositeCriteria{},
		sharedQuery.OffsetPagination{Limit: 10},
		sharedQuery.Sort{Field: "created_at"},
	)
	require.NoError(t, err)
	assert.Len(t, tasks, 1)

	// --- 5. Eliminar Tarea y su evento ---
	deletedEvent := sharedDomain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "Task",
		AggregateID:   task.ID.String(),
		EventType:     "TaskDeleted",
		Payload:       map[string]interface{}{"id": task.ID.String()},
		CreatedAt:     time.Now().UTC(),
	}

	err = repo.DeleteByID(ctx, task.ID, deletedEvent)
	require.NoError(t, err)
	_, err = repo.GetByID(ctx, task.ID)
	assert.ErrorIs(t, err, taskDomain.ErrTaskNotFound)
	verifyOutboxEventPostgres(t, db, task.ID.String(), "TaskDeleted", 3)
}
