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

	userDomain "github.com/davicafu/hexagolab/internal/user/domain"
	sharedDomain "github.com/davicafu/hexagolab/shared/domain"
	sharedQuery "github.com/davicafu/hexagolab/shared/platform/query"
	sharedUtils "github.com/davicafu/hexagolab/shared/utils"
)

type UserRepoSQLite struct {
	db *sql.DB
}

func NewUserRepoSQLite(db *sql.DB) *UserRepoSQLite {
	return &UserRepoSQLite{db: db}
}

// ------------------ Helper DRY para insertar en outbox ------------------

func insertOutboxTx(ctx context.Context, tx *sql.Tx, evt sharedDomain.OutboxEvent) error {
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
func (r *UserRepoSQLite) Create(ctx context.Context, u *userDomain.User, evt sharedDomain.OutboxEvent) error {
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
		u.ID.String(), u.Email, u.Nombre, u.BirthDate.Format(time.RFC3339), u.CreatedAt.Format(time.RFC3339),
	); err != nil {
		return err
	}

	if err := insertOutboxTx(ctx, tx, evt); err != nil {
		return err
	}

	return tx.Commit()
}

// Update actualiza usuario y crea evento en transacción
func (r *UserRepoSQLite) Update(ctx context.Context, u *userDomain.User, evt sharedDomain.OutboxEvent) error {
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
		u.Email, u.Nombre, u.BirthDate.Format(time.RFC3339), u.ID.String(),
	)
	if err != nil {
		return err
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
func (r *UserRepoSQLite) DeleteByID(ctx context.Context, id uuid.UUID, evt sharedDomain.OutboxEvent) error {
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
		return userDomain.ErrUserNotFound
	}

	if err := insertOutboxTx(ctx, tx, evt); err != nil {
		return fmt.Errorf("failed to insert outbox: %w", err)
	}

	return tx.Commit()
}

// ------------------ Lectura ------------------

func (r *UserRepoSQLite) GetByID(ctx context.Context, id uuid.UUID) (*userDomain.User, error) {
	query := `SELECT id, email, nombre, birth_date, created_at FROM users WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id.String())

	var u userDomain.User
	// ✅ 1. Leemos las fechas en variables de texto temporales
	var birthDateStr, createdAtStr string

	// ✅ 2. Usamos esas variables en el Scan
	if err := row.Scan(&u.ID, &u.Email, &u.Nombre, &birthDateStr, &createdAtStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, userDomain.ErrUserNotFound
		}
		return nil, fmt.Errorf("db error: %w", err)
	}

	// ✅ 3. Parseamos las fechas de texto a time.Time
	var err error
	u.BirthDate, err = time.Parse(time.RFC3339, birthDateStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing birth_date: %w", err)
	}
	u.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing created_at: %w", err)
	}

	return &u, nil
}

// Traduce criterios neutrales a SQL para Postgres (?, ?...)
func (r *UserRepoSQLite) applyCriteria(criteria sharedDomain.Criteria) (string, []interface{}) {
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
	criteria sharedDomain.Criteria,
	pagination sharedQuery.Pagination,
	sort sharedQuery.Sort,
) ([]*userDomain.User, error) {
	whereSQL, args := r.applyCriteria(criteria)

	query := "SELECT id, email, nombre, birth_date, created_at FROM users"
	if whereSQL != "" {
		query += " WHERE " + whereSQL
	}

	// --- Paginación según tipo ---
	switch p := pagination.(type) {
	case sharedQuery.OffsetPagination:
		query += fmt.Sprintf(" ORDER BY %s %s LIMIT ? OFFSET ?",
			sort.Field, sharedUtils.Ternary(sort.Desc, "DESC", "ASC"))
		args = append(args, p.Limit, p.Offset)
	case sharedQuery.CursorPagination:
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
			sharedUtils.Ternary(sort.Desc, "DESC", "ASC"),
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
		var idStr, birthDateStr, createdAtStr string

		if err := rows.Scan(&idStr, &u.Email, &u.Nombre, &birthDateStr, &createdAtStr); err != nil {
			return nil, err
		}
		u.ID, _ = uuid.Parse(idStr)
		u.BirthDate, err = time.Parse(time.RFC3339, birthDateStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing birth_date: %w", err)
		}
		u.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing created_at: %w", err)
		}

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
