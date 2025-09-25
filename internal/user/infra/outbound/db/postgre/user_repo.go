package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/davicafu/hexagolab/internal/user/domain"
	"github.com/google/uuid"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type UserRepoPostgres struct {
	db *sql.DB
}

func NewUserRepoPostgres(db *sql.DB) *UserRepoPostgres {
	return &UserRepoPostgres{db: db}
}

// ------------------ Helper DRY para insertar en outbox ------------------

func insertOutboxTx(ctx context.Context, tx *sql.Tx, evt domain.OutboxEvent) error {
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

// ------------------ CRUD + Outbox ------------------

func (r *UserRepoPostgres) Create(ctx context.Context, u *domain.User, evt domain.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO users (id, email, nombre, birth_date, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		u.ID, u.Email, u.Nombre, u.BirthDate, u.CreatedAt,
	)
	if err != nil {
		return err
	}

	if err := insertOutboxTx(ctx, tx, evt); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *UserRepoPostgres) Update(ctx context.Context, u *domain.User, evt domain.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	res, err := tx.ExecContext(ctx,
		`UPDATE users SET email=$1, nombre=$2, birth_date=$3 WHERE id=$4`,
		u.Email, u.Nombre, u.BirthDate, u.ID,
	)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return domain.ErrUserNotFound
	}

	if err := insertOutboxTx(ctx, tx, evt); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *UserRepoPostgres) DeleteByID(ctx context.Context, id uuid.UUID, evt domain.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	res, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, id)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return domain.ErrUserNotFound
	}

	if err := insertOutboxTx(ctx, tx, evt); err != nil {
		return err
	}

	return tx.Commit()
}

// ------------------ Lectura ------------------

func (r *UserRepoPostgres) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `SELECT id, email, nombre, birth_date, created_at FROM users WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)

	var u domain.User
	var idStr string
	if err := row.Scan(&idStr, &u.Email, &u.Nombre, &u.BirthDate, &u.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}

	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID in DB: %w", err)
	}
	u.ID = parsedID

	return &u, nil
}

func (r *UserRepoPostgres) List(ctx context.Context, f domain.UserFilter) ([]*domain.User, error) {
	var users []*domain.User
	var args []interface{}
	var conditions []string

	if f.ID != nil {
		conditions = append(conditions, "id = $1")
		args = append(args, f.ID)
	}
	if f.Email != nil {
		conditions = append(conditions, fmt.Sprintf("email = $%d", len(args)+1))
		args = append(args, *f.Email)
	}
	if f.Nombre != nil {
		conditions = append(conditions, fmt.Sprintf("nombre ILIKE $%d", len(args)+1))
		args = append(args, "%"+*f.Nombre+"%")
	}
	if f.MinAge != nil {
		conditions = append(conditions, fmt.Sprintf("birth_date <= $%d", len(args)+1))
		args = append(args, time.Now().AddDate(-*f.MinAge, 0, 0))
	}
	if f.MaxAge != nil {
		conditions = append(conditions, fmt.Sprintf("birth_date >= $%d", len(args)+1))
		args = append(args, time.Now().AddDate(-*f.MaxAge, 0, 0))
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	orderBy := "created_at DESC"
	if f.Sort.Field != "" {
		dir := "ASC"
		if f.Sort.Desc {
			dir = "DESC"
		}
		orderBy = fmt.Sprintf("%s %s", f.Sort.Field, dir)
	}

	limit := f.Pagination.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := f.Pagination.Offset

	args = append(args, limit, offset)
	query := fmt.Sprintf(`SELECT id, email, nombre, birth_date, created_at FROM users %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		where, orderBy, len(args)-1, len(args),
	)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var u domain.User
		var idStr string
		if err := rows.Scan(&idStr, &u.Email, &u.Nombre, &u.BirthDate, &u.CreatedAt); err != nil {
			return nil, err
		}

		parsedID, err := uuid.Parse(idStr)
		if err != nil {
			return nil, fmt.Errorf("invalid UUID in DB: %w", err)
		}
		u.ID = parsedID

		users = append(users, &u)
	}

	return users, nil
}

// ------------------ Outbox ------------------

func (r *UserRepoPostgres) FetchPendingOutbox(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
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

	var events []domain.OutboxEvent
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

		events = append(events, domain.OutboxEvent{
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

func (r *UserRepoPostgres) MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE outbox SET processed=true WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("failed to mark outbox event %s as processed: %w", id, err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get RowsAffected for outbox event %s: %w", id, err)
	}
	if rows == 0 {
		return fmt.Errorf("no outbox event found with id %s", id)
	}

	return nil
}

// ------------------ Inicialización ------------------

func InitPostgres(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		nombre TEXT NOT NULL,
		birth_date DATE NOT NULL,
		created_at TIMESTAMP NOT NULL
	)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS outbox (
		id UUID PRIMARY KEY,
		aggregate_type TEXT NOT NULL,
		aggregate_id UUID NOT NULL,
		event_type TEXT NOT NULL,
		payload JSONB NOT NULL,
		created_at TIMESTAMP NOT NULL,
		processed BOOLEAN NOT NULL DEFAULT FALSE
	)`)
	return err
}
