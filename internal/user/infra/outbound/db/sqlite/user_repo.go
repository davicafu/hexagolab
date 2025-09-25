package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	// _ "github.com/mattn/go-sqlite3" // better performance but requires gcc
	_ "modernc.org/sqlite"

	"github.com/davicafu/hexagolab/internal/user/domain"
)

type UserRepoSQLite struct {
	db *sql.DB
}

func NewUserRepoSQLite(db *sql.DB) *UserRepoSQLite {
	return &UserRepoSQLite{db: db}
}

// ------------------ Helper DRY para insertar en outbox ------------------

func insertOutboxTx(ctx context.Context, tx *sql.Tx, evt domain.OutboxEvent) error {
	payloadBytes, err := json.Marshal(evt.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal outbox payload: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO outbox (id,aggregate_type,aggregate_id,event_type,payload,created_at,processed)
		 VALUES (?,?,?,?,?,?,0)`,
		evt.ID.String(), evt.AggregateType, evt.AggregateID, evt.EventType, string(payloadBytes), evt.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return nil
}

// ------------------ Métodos ------------------

// Create inserta usuario y evento en transacción
func (r *UserRepoSQLite) Create(ctx context.Context, u *domain.User, evt domain.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO users (id,email,nombre,birth_date,created_at) VALUES (?,?,?,?,?)`,
		u.ID.String(), u.Email, u.Nombre, u.BirthDate, u.CreatedAt,
	); err != nil {
		return err
	}

	if err := insertOutboxTx(ctx, tx, evt); err != nil {
		return err
	}

	return tx.Commit()
}

// Update actualiza usuario y crea evento Outbox en transacción
func (r *UserRepoSQLite) Update(ctx context.Context, u *domain.User, evt domain.OutboxEvent) error {
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
		`UPDATE users SET email=?, nombre=?, birth_date=? WHERE id=?`,
		u.Email, u.Nombre, u.BirthDate, u.ID.String(),
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

// Delete elimina usuario y crea evento Outbox en transacción
func (r *UserRepoSQLite) DeleteByID(ctx context.Context, id uuid.UUID, evt domain.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	res, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id.String())
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

// GetByID con manejo de errores en uuid.Parse
func (r *UserRepoSQLite) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `SELECT id, email, nombre, birth_date, created_at FROM users WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id.String())

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

// List con manejo de errores en uuid.Parse y json.Unmarshal
func (r *UserRepoSQLite) List(ctx context.Context, f domain.UserFilter) ([]*domain.User, error) {
	var users []*domain.User
	var args []interface{}
	var conditions []string

	if f.ID != nil {
		conditions = append(conditions, "id = ?")
		args = append(args, f.ID.String())
	}
	if f.Email != nil {
		conditions = append(conditions, "email = ?")
		args = append(args, *f.Email)
	}
	if f.Nombre != nil {
		conditions = append(conditions, "nombre LIKE ?")
		args = append(args, "%"+*f.Nombre+"%")
	}
	if f.MinAge != nil {
		conditions = append(conditions, "birth_date <= ?")
		args = append(args, time.Now().AddDate(-*f.MinAge, 0, 0))
	}
	if f.MaxAge != nil {
		conditions = append(conditions, "birth_date >= ?")
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

	query := fmt.Sprintf(`SELECT id, email, nombre, birth_date, created_at
		FROM users %s ORDER BY %s LIMIT ? OFFSET ?`, where, orderBy)
	args = append(args, limit, offset)

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

// ------------------ Inicialización de DB ------------------

// InitSQLite crea la tabla users si no existe
func InitSQLite(db *sql.DB) error {
	// Tabla de usuarios
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id TEXT PRIMARY KEY,
            email TEXT UNIQUE NOT NULL,
            nombre TEXT NOT NULL,
            birth_date DATE NOT NULL,
            created_at DATETIME NOT NULL
        )
    `)
	if err != nil {
		return err
	}

	// Tabla de Outbox
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS outbox (
            id TEXT PRIMARY KEY,
            aggregate_type TEXT NOT NULL,
            aggregate_id TEXT NOT NULL,
            event_type TEXT NOT NULL,
            payload TEXT NOT NULL,
            created_at DATETIME NOT NULL,
            processed BOOLEAN NOT NULL DEFAULT 0
        )
    `)
	return err
}

// ---------------- Patrón Outbox en Eventos-----------------

func (r *UserRepoSQLite) SaveOutboxEvent(ctx context.Context, evt domain.OutboxEvent) error {
	payloadBytes, _ := json.Marshal(evt.Payload)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO outbox (id, aggregate_type, aggregate_id, event_type, payload, created_at, processed)
		 VALUES (?, ?, ?, ?, ?, ?, 0)`,
		evt.ID.String(), evt.AggregateType, evt.AggregateID, evt.EventType, string(payloadBytes), evt.CreatedAt,
	)
	return err
}

// FetchPendingOutbox obtiene eventos pendientes y maneja errores de UUID y JSON
func (r *UserRepoSQLite) FetchPendingOutbox(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
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
		var idStr, aggregateType, aggregateID, eventType, payloadStr string
		var createdAt time.Time

		if err := rows.Scan(&idStr, &aggregateType, &aggregateID, &eventType, &payloadStr, &createdAt); err != nil {
			return nil, err
		}

		parsedID, err := uuid.Parse(idStr)
		if err != nil {
			return nil, fmt.Errorf("invalid UUID in outbox row: %w", err)
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
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

// MarkOutboxProcessed marca un evento como procesado y devuelve error si falla
func (r *UserRepoSQLite) MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE outbox SET processed = 1 WHERE id = ?`,
		id.String(),
	)
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
