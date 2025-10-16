package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	userDomain "github.com/davicafu/hexagolab/internal/user/domain"
	sharedDomain "github.com/davicafu/hexagolab/shared/domain"
	sharedQuery "github.com/davicafu/hexagolab/shared/platform/query"
	sharedUtils "github.com/davicafu/hexagolab/shared/utils"
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

// ------------------ CRUD + Outbox ------------------

// Create inserta usuario y evento en transacción
func (r *UserRepoPostgres) Create(ctx context.Context, u *userDomain.User, evt sharedDomain.OutboxEvent) error {
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

// Update actualiza usuario y crea evento en transacción
func (r *UserRepoPostgres) Update(ctx context.Context, u *userDomain.User, evt sharedDomain.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
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
		return fmt.Errorf("db error: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return userDomain.ErrUserNotFound
	}

	if err := insertOutboxTx(ctx, tx, evt); err != nil {
		return fmt.Errorf("failed to insert outbox: %w", err)
	}

	return tx.Commit()
}

// Delete elimina usuario y crea evento en transacción
func (r *UserRepoPostgres) DeleteByID(ctx context.Context, id uuid.UUID, evt sharedDomain.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	res, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("db error: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return userDomain.ErrUserNotFound
	}

	if err := insertOutboxTx(ctx, tx, evt); err != nil {
		return fmt.Errorf("failed to insert outbox: %w", err)
	}

	return tx.Commit()
}

// ------------------ Lectura ------------------

func (r *UserRepoPostgres) GetByID(ctx context.Context, id uuid.UUID) (*userDomain.User, error) {
	query := `SELECT id, email, nombre, birth_date, created_at FROM users WHERE id=$1`
	row := r.db.QueryRowContext(ctx, query, id)

	var u userDomain.User
	var idStr string
	if err := row.Scan(&idStr, &u.Email, &u.Nombre, &u.BirthDate, &u.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, userDomain.ErrUserNotFound
		}
		return nil, fmt.Errorf("db error: %w", err)
	}

	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		return nil, userDomain.ErrInvalidUser
	}
	u.ID = parsedID

	return &u, nil
}

// Traduce criterios neutrales a SQL para Postgres ($1, $2...)
func (r *UserRepoPostgres) applyCriteria(criteria sharedDomain.Criteria) (string, []interface{}) {
	conds := criteria.ToConditions()
	var clauses []string
	var args []interface{}
	for i, c := range conds {
		clauses = append(clauses, fmt.Sprintf("%s %s $%d", c.Field, c.Op, i+1))
		args = append(args, c.Value)
	}
	return strings.Join(clauses, " AND "), args
}

func (r *UserRepoPostgres) ListByCriteria(ctx context.Context, criteria sharedDomain.Criteria, pagination sharedQuery.Pagination, sort sharedQuery.Sort) ([]*userDomain.User, error) {
	whereSQL, args := r.applyCriteria(criteria)

	query := "SELECT id, email, nombre, birth_date, created_at FROM users"
	if whereSQL != "" {
		query += " WHERE " + whereSQL
	}

	// --- Paginación según tipo ---
	switch p := pagination.(type) {
	case sharedQuery.OffsetPagination:
		args = append(args, p.Limit, p.Offset)
		query += fmt.Sprintf(" ORDER BY %s %s LIMIT $%d OFFSET $%d",
			sort.Field, sharedUtils.Ternary(sort.Desc, "DESC", "ASC"), len(args)-1, len(args))
	case sharedQuery.CursorPagination:
		if p.Cursor != "" {
			parts := strings.SplitN(p.Cursor, "|", 2)
			cursorSort := parts[0]
			cursorID := parts[1]

			if whereSQL != "" {
				query += fmt.Sprintf(" AND (%s, id) > ($%d, $%d)", sort.Field, len(args)+1, len(args)+2)
			} else {
				query += fmt.Sprintf(" WHERE (%s, id) > ($%d, $%d)", sort.Field, len(args)+1, len(args)+2)
			}
			args = append(args, cursorSort, cursorID)
		}
		query += fmt.Sprintf(" ORDER BY %s %s, id %s LIMIT %d",
			sort.Field, sharedUtils.Ternary(sort.Desc, "DESC", "ASC"),
			sharedUtils.Ternary(sort.Desc, "DESC", "ASC"),
			p.Limit,
		)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*userDomain.User
	for rows.Next() {
		var u userDomain.User
		var idStr string
		if err := rows.Scan(&idStr, &u.Email, &u.Nombre, &u.BirthDate, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.ID, _ = uuid.Parse(idStr)
		users = append(users, &u)
	}

	return users, nil
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
