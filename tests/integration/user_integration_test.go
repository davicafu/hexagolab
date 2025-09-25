package integration

import (
    "context"
    "database/sql"
    "testing"
    "time"

    "github.com/davicafu/hexagolab/internal/user/domain"
    "github.com/davicafu/hexagolab/internal/user/infra/outbound/db/sqlite"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    _ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("sqlite", ":memory:")
    assert.NoError(t, err)

    _, err = db.Exec(`
        CREATE TABLE users (
            id TEXT PRIMARY KEY,
            email TEXT NOT NULL,
            nombre TEXT NOT NULL,
            birth_date TEXT NOT NULL,
            created_at TEXT NOT NULL
        )
    `)
    assert.NoError(t, err)
    return db
}

func TestUserSQLiteIntegration_CreateGetUpdateDelete(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    repo := sqlite.NewUserRepoSQLite(db)

    // Crear usuario
    user := &domain.User{
        ID:        uuid.New(),
        Email:     "integration@example.com",
        Nombre:    "Integrado",
        BirthDate: time.Date(1992, 6, 15, 0, 0, 0, 0, time.UTC),
        CreatedAt: time.Now(),
    }
    err := repo.Create(context.Background(), user)
    assert.NoError(t, err)

    // Obtener usuario
    got, err := repo.GetByID(context.Background(), user.ID)
    assert.NoError(t, err)
    assert.Equal(t, user.Email, got.Email)
    assert.Equal(t, user.Nombre, got.Nombre)

    // Actualizar usuario
    user.Nombre = "Actualizado"
    err = repo.Update(context.Background(), user)
    assert.NoError(t, err)
    got, err = repo.GetByID(context.Background(), user.ID)
    assert.NoError(t, err)
    assert.Equal(t, "Actualizado", got.Nombre)

    // Listar usuarios
    users, err := repo.List(context.Background(), domain.UserFilter{})
    assert.NoError(t, err)
    assert.Len(t, users, 1)

    // Eliminar usuario
    err = repo.DeleteByID(context.Background(), user.ID)
    assert.NoError(t, err)
    _, err = repo.GetByID(context.Background(), user.ID)
    assert.ErrorIs(t, err, domain.ErrUserNotFound)
}