package integration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sharedDomain "github.com/davicafu/hexagolab/internal/shared/domain"
	sharedQuery "github.com/davicafu/hexagolab/internal/shared/infra/platform/query"
	userDomain "github.com/davicafu/hexagolab/internal/user/domain"
	"github.com/davicafu/hexagolab/internal/user/infra/outbound/db/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err) // Usar require para detener el test si la DB falla

	// Crear tabla de usuarios
	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			nombre TEXT NOT NULL,
			birth_date TEXT NOT NULL,
			created_at TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	// Crear tabla de outbox (necesaria para el test)
	_, err = db.Exec(`
		CREATE TABLE outbox (
			id TEXT PRIMARY KEY,
			aggregate_type TEXT NOT NULL,
			aggregate_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			payload TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			processed BOOLEAN NOT NULL DEFAULT 0
		)
	`)
	require.NoError(t, err)

	return db
}

// verifyOutboxEvent es un helper para no repetir código de verificación
func verifyOutboxEvent(t *testing.T, db *sql.DB, aggregateID string, expectedEventType string, expectedCount int) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM outbox WHERE aggregate_id = ?", aggregateID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, expectedCount, count, "el número de eventos en outbox no es el esperado")

	var eventType string
	// Buscamos el último evento para este agregado
	err = db.QueryRow("SELECT event_type FROM outbox WHERE aggregate_id = ? ORDER BY created_at DESC LIMIT 1", aggregateID).Scan(&eventType)
	require.NoError(t, err)
	assert.Equal(t, expectedEventType, eventType, "el tipo de evento no coincide")
}

func TestUserSQLiteIntegration_CreateGetUpdateDelete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := sqlite.NewUserRepoSQLite(db)
	ctx := context.Background()

	// --- 1. Crear usuario y su evento ---
	user := &userDomain.User{
		ID:        uuid.New(),
		Email:     "integration@example.com",
		Nombre:    "Integrado",
		BirthDate: time.Date(1992, 6, 15, 0, 0, 0, 0, time.UTC),
		CreatedAt: time.Now().UTC(),
	}
	// Crear el evento de outbox asociado
	createdEvent := sharedDomain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "User",
		AggregateID:   user.ID.String(),
		EventType:     "UserCreated",
		Payload:       map[string]interface{}{"email": user.Email, "nombre": user.Nombre},
		CreatedAt:     time.Now().UTC(),
	}

	err := repo.Create(ctx, user, createdEvent)
	assert.NoError(t, err)
	// Verificar que el evento se ha creado en la tabla outbox
	verifyOutboxEvent(t, db, user.ID.String(), "UserCreated", 1)

	// --- 2. Obtener usuario (sin cambios) ---
	got, err := repo.GetByID(ctx, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, user.Email, got.Email)

	// --- 3. Actualizar usuario y su evento ---
	user.Nombre = "Actualizado"
	time.Sleep(2 * time.Millisecond)
	updatedEvent := sharedDomain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "User",
		AggregateID:   user.ID.String(),
		EventType:     "UserUpdated",
		Payload:       map[string]interface{}{"nombre": user.Nombre},
		CreatedAt:     time.Now().UTC(),
	}

	err = repo.Update(ctx, user, updatedEvent)
	assert.NoError(t, err)
	got, err = repo.GetByID(ctx, user.ID)
	assert.NoError(t, err)

	assert.Equal(t, "Actualizado", got.Nombre)
	// Verificar que AHORA hay dos eventos y el último es "UserUpdated"
	verifyOutboxEvent(t, db, user.ID.String(), "UserUpdated", 2)

	// --- 4. Listar usuarios (sin cambios) ---
	users, err := repo.ListByCriteria(ctx, sharedDomain.CompositeCriteria{}, sharedQuery.OffsetPagination{Limit: 10}, sharedQuery.Sort{Field: "created_at"})
	assert.NoError(t, err)
	assert.Len(t, users, 1)

	// --- 5. Eliminar usuario y su evento ---
	time.Sleep(2 * time.Millisecond)
	deletedEvent := sharedDomain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "User",
		AggregateID:   user.ID.String(),
		EventType:     "UserDeleted",
		Payload:       map[string]interface{}{"id": user.ID.String()},
		CreatedAt:     time.Now().UTC(),
	}

	err = repo.DeleteByID(ctx, user.ID, deletedEvent)
	assert.NoError(t, err)
	_, err = repo.GetByID(ctx, user.ID)
	assert.ErrorIs(t, err, userDomain.ErrUserNotFound)
	// Verificar que AHORA hay tres eventos y el último es "UserDeleted"
	verifyOutboxEvent(t, db, user.ID.String(), "UserDeleted", 3)
}
