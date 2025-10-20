package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	// --- Importaciones del dominio y compartidas ---
	sharedDomain "github.com/davicafu/hexagolab/internal/shared/domain"
	sharedQuery "github.com/davicafu/hexagolab/internal/shared/infra/platform/query"
	sharedUtils "github.com/davicafu/hexagolab/internal/shared/infra/utils"
	taskDomain "github.com/davicafu/hexagolab/internal/task/domain"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib" // Driver de PostgreSQL
)

// TaskRepoPostgres implementa la interfaz TaskRepository para PostgreSQL.
type TaskRepoPostgres struct {
	db *sql.DB
}

// NewTaskRepoPostgres es el constructor del repositorio.
func NewTaskRepoPostgres(db *sql.DB) *TaskRepoPostgres {
	return &TaskRepoPostgres{db: db}
}

// ------------------ CRUD + Outbox ------------------

// Create inserta una tarea y un evento en una transacción.
func (r *TaskRepoPostgres) Create(ctx context.Context, t *taskDomain.Task, evt sharedDomain.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // Se ignora si el Commit() es exitoso

	_, err = tx.ExecContext(ctx,
		`INSERT INTO tasks (id, title, description, assignee_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		t.ID, t.Title, t.Description, t.AssigneeID, t.Status, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return err
	}

	if err := insertOutboxTx(ctx, tx, evt); err != nil {
		return err
	}

	return tx.Commit()
}

// Update actualiza una tarea y crea un evento en una transacción.
func (r *TaskRepoPostgres) Update(ctx context.Context, t *taskDomain.Task, evt sharedDomain.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE tasks SET title=$1, description=$2, assignee_id=$3, status=$4, updated_at=$5 WHERE id=$6`,
		t.Title, t.Description, t.AssigneeID, t.Status, t.UpdatedAt, t.ID,
	)
	if err != nil {
		return fmt.Errorf("db error: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return taskDomain.ErrTaskNotFound
	}

	if err := insertOutboxTx(ctx, tx, evt); err != nil {
		return fmt.Errorf("failed to insert outbox: %w", err)
	}

	return tx.Commit()
}

// DeleteByID elimina una tarea y crea un evento en una transacción.
func (r *TaskRepoPostgres) DeleteByID(ctx context.Context, id uuid.UUID, evt sharedDomain.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `DELETE FROM tasks WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("db error: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return taskDomain.ErrTaskNotFound
	}

	if err := insertOutboxTx(ctx, tx, evt); err != nil {
		return fmt.Errorf("failed to insert outbox: %w", err)
	}

	return tx.Commit()
}

// ------------------ Lectura ------------------

// GetByID recupera una tarea de la base de datos por su ID.
func (r *TaskRepoPostgres) GetByID(ctx context.Context, id uuid.UUID) (*taskDomain.Task, error) {
	query := `SELECT id, title, description, assignee_id, status, created_at, updated_at FROM tasks WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)

	var t taskDomain.Task
	err := row.Scan(
		&t.ID, &t.Title, &t.Description, &t.AssigneeID, &t.Status, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, taskDomain.ErrTaskNotFound
		}
		return nil, fmt.Errorf("db scan error: %w", err)
	}

	return &t, nil
}

// applyCriteria traduce criterios a SQL para Postgres ($1, $2...).
func (r *TaskRepoPostgres) applyCriteria(criteria sharedDomain.Criteria) (string, []interface{}) {
	conds := criteria.ToConditions()
	if len(conds) == 0 {
		return "", nil
	}
	var clauses []string
	var args []interface{}
	for i, c := range conds {
		clauses = append(clauses, fmt.Sprintf("%s %s $%d", c.Field, c.Op, i+1))
		args = append(args, c.Value)
	}
	return strings.Join(clauses, " AND "), args
}

// ListByCriteria recupera una lista de tareas aplicando filtros, paginación y ordenamiento.
func (r *TaskRepoPostgres) ListByCriteria(ctx context.Context, criteria sharedDomain.Criteria, pagination sharedQuery.Pagination, sort sharedQuery.Sort) ([]*taskDomain.Task, error) {
	whereSQL, args := r.applyCriteria(criteria)

	query := "SELECT id, title, description, assignee_id, status, created_at, updated_at FROM tasks"
	if whereSQL != "" {
		query += " WHERE " + whereSQL
	}

	// Añadir ordenamiento y paginación
	argOffset := len(args)
	query += fmt.Sprintf(" ORDER BY %s %s", sort.Field, sharedUtils.Ternary(sort.Desc, "DESC", "ASC"))

	if p, ok := pagination.(sharedQuery.OffsetPagination); ok {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argOffset+1, argOffset+2)
		args = append(args, p.Limit, p.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*taskDomain.Task
	for rows.Next() {
		var t taskDomain.Task
		err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.AssigneeID, &t.Status, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, &t)
	}

	return tasks, nil
}

// ------------------ Inicialización del Esquema ------------------

// InitPostgresTaskSchema crea la tabla 'tasks' y 'outbox' si no existen.
func InitPostgresTaskSchema(db *sql.DB) error {
	_, err := db.Exec(`
    CREATE TABLE IF NOT EXISTS tasks (
        id UUID PRIMARY KEY,
        title TEXT NOT NULL,
        description TEXT,
        assignee_id UUID,
        status TEXT NOT NULL,
        created_at TIMESTAMP WITH TIME ZONE NOT NULL,
        updated_at TIMESTAMP WITH TIME ZONE NOT NULL
    )`)
	if err != nil {
		return fmt.Errorf("failed to create tasks table: %w", err)
	}

	// La tabla Outbox es compartida, pero la definimos aquí por completitud.
	// En una aplicación real, la inicialización del esquema podría estar centralizada.
	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS outbox (
        id UUID PRIMARY KEY,
        aggregate_type TEXT NOT NULL,
        aggregate_id TEXT NOT NULL,
        event_type TEXT NOT NULL,
        payload JSONB NOT NULL,
        created_at TIMESTAMP WITH TIME ZONE NOT NULL,
        processed BOOLEAN NOT NULL DEFAULT FALSE
    )`)
	return err
}

// ---------------- Patrón Outbox (Idéntico al de User) -----------------

// FetchPendingOutbox obtiene los eventos no procesados.
func (r *TaskRepoPostgres) FetchPendingOutbox(ctx context.Context, limit int) ([]sharedDomain.OutboxEvent, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at
		 FROM outbox
		 WHERE processed=false
		 ORDER BY created_at
		 LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []sharedDomain.OutboxEvent
	for rows.Next() {
		var idStr, aggregateType, aggregateID, eventType string
		var payloadBytes []byte
		var createdAt time.Time

		if err := rows.Scan(&idStr, &aggregateType, &aggregateID, &eventType, &payloadBytes, &createdAt); err != nil {
			return nil, err
		}

		parsedID, err := uuid.Parse(idStr)
		if err != nil {
			return nil, fmt.Errorf("invalid UUID in outbox row: %w", err)
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			return nil, fmt.Errorf("invalid JSON payload in outbox row %s: %w", parsedID, err)
		}

		events = append(events, sharedDomain.OutboxEvent{
			ID:            parsedID,
			AggregateType: aggregateType,
			AggregateID:   aggregateID,
			EventType:     eventType,
			Payload:       payload,
			CreatedAt:     createdAt,
			Processed:     false,
		})
	}

	return events, nil
}

func (r *TaskRepoPostgres) MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `UPDATE outbox SET processed=true WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("db error: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get RowsAffected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("outbox event not found: %s", id)
	}

	return nil
}

// ------------------ Helper DRY para insertar en outbox ------------------
func insertOutboxTx(ctx context.Context, tx *sql.Tx, evt sharedDomain.OutboxEvent) error {
	payloadBytes, err := json.Marshal(evt.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal outbox payload: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO outbox (id, aggregate_type, aggregate_id, event_type, payload, created_at, processed)
		 VALUES ($1, $2, $3, $4, $5, $6, false)`,
		evt.ID, evt.AggregateType, evt.AggregateID, evt.EventType, payloadBytes, evt.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}
	return nil
}
