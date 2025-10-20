// en internal/infra/db/sqlite/outbox_repository.go
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/davicafu/hexagolab/internal/shared/domain"
	"github.com/google/uuid"
)

// OutboxRepoSQLite implementa la interfaz shared.OutboxRepository.
type OutboxRepoSQLite struct {
	db *sql.DB
}

func NewOutboxRepoSQLite(db *sql.DB) *OutboxRepoSQLite {
	return &OutboxRepoSQLite{db: db}
}

// FetchPendingOutbox obtiene los eventos no procesados de la tabla outbox para SQLite.
// ✅ Nota: Ahora este método pertenece a OutboxRepoSQLite.
func (r *OutboxRepoSQLite) FetchPendingOutbox(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at
         FROM outbox
         WHERE processed = 0
         ORDER BY created_at
         LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.OutboxEvent
	for rows.Next() {
		var evt domain.OutboxEvent
		var payloadStr string // El payload se lee como string en SQLite

		if err := rows.Scan(&evt.ID, &evt.AggregateType, &evt.AggregateID, &evt.EventType, &payloadStr, &evt.CreatedAt); err != nil {
			return nil, err
		}

		// El ID en la base de datos se guarda como TEXT, por lo que lo parseamos de nuevo.
		parsedID, err := uuid.Parse(evt.AggregateID)
		if err != nil {
			return nil, fmt.Errorf("invalid UUID in outbox row: %w", err)
		}
		evt.AggregateID = parsedID.String()

		if err := json.Unmarshal([]byte(payloadStr), &evt.Payload); err != nil {
			return nil, fmt.Errorf("invalid JSON payload in outbox row %s: %w", evt.ID, err)
		}

		events = append(events, evt)
	}

	return events, nil
}

// MarkOutboxProcessed marca un evento como procesado para SQLite.
// ✅ Nota: Ahora este método pertenece a OutboxRepoSQLite.
func (r *OutboxRepoSQLite) MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `UPDATE outbox SET processed = 1 WHERE id = ?`, id)
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

// Verificación en tiempo de compilación.
var _ domain.OutboxRepository = (*OutboxRepoSQLite)(nil)
