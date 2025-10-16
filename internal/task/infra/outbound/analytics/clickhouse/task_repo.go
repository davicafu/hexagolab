package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	taskDomain "github.com/davicafu/hexagolab/internal/task/domain"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// TaskAnalyticsRepo implementa la interfaz TaskAnalyticsRepository para ClickHouse.
type TaskAnalyticsRepo struct {
	db *sql.DB
}

// NewTaskAnalyticsRepo es el constructor.
func NewTaskAnalyticsRepo(addr string, dbName string) (*TaskAnalyticsRepo, error) {
	conn := clickhouse.OpenDB(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			// ... tus credenciales si son necesarias
			Database: dbName,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
	})

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("could not ping clickhouse: %w", err)
	}

	return &TaskAnalyticsRepo{db: conn}, nil
}

// LogBatch inserta un lote de tareas en ClickHouse. Esta es la forma más eficiente.
func (r *TaskAnalyticsRepo) LogBatch(ctx context.Context, tasks []*taskDomain.Task) error {
	// ClickHouse funciona mejor con inserciones en lotes.
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	// Preparamos la sentencia de inserción.
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO tasks_log (id, title, description, assignee_id, status, created_at, updated_at, event_time)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	eventTime := time.Now()
	for _, task := range tasks {
		if _, err := stmt.ExecContext(
			ctx,
			task.ID,
			task.Title,
			task.Description,
			task.AssigneeID,
			string(task.Status),
			task.CreatedAt,
			task.UpdatedAt,
			eventTime,
		); err != nil {
			// Si un registro falla, hacemos rollback de todo el lote.
			// Podrías añadir lógica para manejar registros fallidos individualmente.
			tx.Rollback()
			return fmt.Errorf("failed to exec statement for task %s: %w", task.ID, err)
		}
	}

	// Si todos los registros se añadieron al lote, hacemos commit.
	return tx.Commit()
}

func (r *TaskAnalyticsRepo) GetDailyTrend(ctx context.Context, start, end time.Time) ([]taskDomain.DailyTaskTrend, error) {
	query := `
		SELECT
			toStartOfDay(event_time) AS day,
			countIf(event_type = 'task.created') AS created,
			countIf(status = 'completed' AND event_type = 'task.updated') AS completed
		FROM tasks_log
		WHERE event_time BETWEEN ? AND ?
		GROUP BY day
		ORDER BY day
	`
	rows, err := r.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trends []taskDomain.DailyTaskTrend
	for rows.Next() {
		var trend taskDomain.DailyTaskTrend
		if err := rows.Scan(&trend.Day, &trend.CreatedCount, &trend.CompletedCount); err != nil {
			return nil, err
		}
		trends = append(trends, trend)
	}
	return trends, nil
}

func (r *TaskAnalyticsRepo) GetAverageCompletionTime(ctx context.Context, start, end time.Time) (time.Duration, error) {
	// Esta consulta es más avanzada. Busca el primer evento 'created' y el último 'completed' para cada ID
	// y calcula la diferencia de tiempo promedio.
	query := `
		SELECT
			avg(completion_time - creation_time) AS avg_completion_seconds
		FROM (
			SELECT
				id,
				minIf(updated_at, event_type = 'task.created') AS creation_time,
				maxIf(updated_at, status = 'completed') AS completion_time
			FROM tasks_log
			WHERE id IN (
				SELECT DISTINCT id FROM tasks_log WHERE status = 'completed' AND event_time BETWEEN ? AND ?
			)
			GROUP BY id
		)
		WHERE creation_time > 0 AND completion_time > 0
	`
	var avgSeconds sql.NullFloat64
	err := r.db.QueryRowContext(ctx, query, start, end).Scan(&avgSeconds)
	if err != nil {
		return 0, err
	}
	if !avgSeconds.Valid {
		return 0, nil // No hay datos para calcular
	}

	return time.Duration(avgSeconds.Float64) * time.Second, nil
}

// InitSchema crea la tabla en ClickHouse si no existe.
func (r *TaskAnalyticsRepo) InitSchema() error {
	// Esta tabla está optimizada para analítica.
	// Se particiona por mes y se ordena por campos comunes de consulta.
	query := `
		CREATE TABLE IF NOT EXISTS tasks_log (
			id          UUID,
			title       String,
			description String,
			assignee_id UUID,
			status      String,
			created_at  DateTime64(3),
			updated_at  DateTime64(3),
			event_time  DateTime64(3)
		) ENGINE = MergeTree()
		PARTITION BY toYYYYMM(event_time)
		ORDER BY (assignee_id, status, event_time);
	`
	_, err := r.db.Exec(query)
	return err
}

// Verificación estática de la interfaz.
var _ taskDomain.TaskAnalyticsRepository = (*TaskAnalyticsRepo)(nil)
