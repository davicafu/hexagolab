package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	sharedDomain "github.com/davicafu/hexagolab/shared/domain"
	"github.com/google/uuid"
)

// OutboxRepoPostgres implementa la interfaz sharedDomain.OutboxRepository.
type OutboxRepoPostgres struct {
	db *sql.DB
}

func NewOutboxRepoPostgres(db *sql.DB) *OutboxRepoPostgres {
	return &OutboxRepoPostgres{db: db}
}

// FetchPendingOutbox obtiene los eventos no procesados de la tabla outbox para Postgres.
// ✅ Nota: Ahora este método pertenece a OutboxRepoPostgres.
func (r *OutboxRepoPostgres) FetchPendingOutbox(ctx context.Context, limit int) ([]sharedDomain.OutboxEvent, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at
		 FROM outbox WHERE processed=false ORDER BY created_at LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []sharedDomain.OutboxEvent
	for rows.Next() {
		var evt sharedDomain.OutboxEvent
		var payloadBytes []byte // El payload se lee como JSONB

		if err := rows.Scan(&evt.ID, &evt.AggregateType, &evt.AggregateID, &evt.EventType, &payloadBytes, &evt.CreatedAt); err != nil {
			return nil, err
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			return nil, fmt.Errorf("invalid JSON payload in outbox row %s: %w", evt.ID, err)
		}
		evt.Payload = payload

		events = append(events, evt)
	}

	return events, nil
}

// MarkOutboxProcessed marca un evento como procesado para Postgres.
// ✅ Nota: Ahora este método pertenece a OutboxRepoPostgres.
func (r *OutboxRepoPostgres) MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error {
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

// Verificación en tiempo de compilación.
var _ sharedDomain.OutboxRepository = (*OutboxRepoPostgres)(nil)
