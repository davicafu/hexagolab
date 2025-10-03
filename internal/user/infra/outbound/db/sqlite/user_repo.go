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
	"github.com/davicafu/hexagolab/internal/user/infra"
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

// ------------------ CRUD + Outbox ------------------

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

// Update actualiza usuario y crea evento en transacción
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
		return fmt.Errorf("failed to insert outbox: %w", err)
	}

	return tx.Commit()
}

// Delete elimina usuario y crea evento en transacción
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
		return fmt.Errorf("failed to insert outbox: %w", err)
	}

	return tx.Commit()
}

// ------------------ Lectura ------------------

func (r *UserRepoSQLite) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `SELECT id, email, nombre, birth_date, created_at FROM users WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id.String())

	var u domain.User
	var idStr string
	if err := row.Scan(&idStr, &u.Email, &u.Nombre, &u.BirthDate, &u.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("db error: %w", err)
	}

	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		return nil, domain.ErrInvalidUser
	}
	u.ID = parsedID

	return &u, nil
}

// Traduce criterios neutrales a SQL para Postgres (?, ?...)
func (r *UserRepoSQLite) applyCriteria(criteria domain.Criteria) (string, []interface{}) {
	conds := criteria.ToConditions()
	var clauses []string
	var args []interface{}
	for _, c := range conds {
		clauses = append(clauses, fmt.Sprintf("%s %s ?", c.Field, c.Op))
		args = append(args, c.Value)
	}
	return strings.Join(clauses, " AND "), args
}

func (r *UserRepoSQLite) ListByCriteria(
	ctx context.Context,
	criteria domain.Criteria,
	pagination domain.Pagination,
	sort domain.Sort,
) ([]*domain.User, error) {
	whereSQL, args := r.applyCriteria(criteria)

	query := "SELECT id, email, nombre, birth_date, created_at FROM users"
	if whereSQL != "" {
		query += " WHERE " + whereSQL
	}

	// --- Paginación según tipo ---
	switch p := pagination.(type) {
	case domain.OffsetPagination:
		query += fmt.Sprintf(" ORDER BY %s %s LIMIT ? OFFSET ?",
			sort.Field, infra.Ternary(sort.Desc, "DESC", "ASC"))
		args = append(args, p.Limit, p.Offset)
	case domain.CursorPagination:
		if p.Cursor != "" {
			// Separar p.Cursor en sortValue y ID
			parts := strings.SplitN(p.Cursor, "|", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid cursor format")
			}
			cursorSort := parts[0]
			cursorID := parts[1]

			// Construir la condición WHERE para cursor compuesto
			condition := fmt.Sprintf("(%s, id) > (?, ?)", sort.Field)
			if whereSQL != "" {
				query += " AND " + condition
			} else {
				query += " WHERE " + condition
			}

			// Agregar los valores al args
			args = append(args, cursorSort, cursorID)
		}

		// Ordenar primero por el sortField y luego por ID para mantener consistencia
		query += fmt.Sprintf(
			" ORDER BY %s %s, id %s LIMIT %d",
			sort.Field,
			infra.Ternary(sort.Desc, "DESC", "ASC"),
			infra.Ternary(sort.Desc, "DESC", "ASC"),
			p.Limit,
		)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var u domain.User
		var idStr string
		if err := rows.Scan(&idStr, &u.Email, &u.Nombre, &u.BirthDate, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.ID, _ = uuid.Parse(idStr)
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
