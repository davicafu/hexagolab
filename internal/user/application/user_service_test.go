package application

import (
	"context"
	"testing"
	"time"

	"github.com/davicafu/hexagolab/internal/user/domain"
	"github.com/davicafu/hexagolab/tests/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCreateUser_Success(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	cache := &mocks.DummyCache{}
	events := &mocks.DummyPublisher{}
	service := NewUserService(repo, cache, events)

	user, err := service.CreateUser(context.Background(), "test@example.com", "Pepe", time.Date(1990, 5, 10, 0, 0, 0, 0, time.UTC))
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Pepe", user.Nombre)

	// ✅ Verificar que se creó un evento Outbox
	assert.Len(t, repo.Outbox, 1)
	assert.Equal(t, "user.created", repo.Outbox[0].EventType)
	assert.Equal(t, user.ID.String(), repo.Outbox[0].AggregateID)
}

func TestCreateUser_AlreadyExists(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	cache := &mocks.DummyCache{}
	events := &mocks.DummyPublisher{}
	service := NewUserService(repo, cache, events)

	user, _ := service.CreateUser(context.Background(), "dup@example.com", "Juan", time.Now())
	// Intentar crear de nuevo con el mismo ID usando repo directamente
	err := repo.Create(context.Background(), user, domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "User",
		AggregateID:   user.ID.String(),
		EventType:     "user.created",
		Payload:       map[string]interface{}{"email": user.Email},
		CreatedAt:     time.Now(),
	})
	assert.ErrorIs(t, err, domain.ErrUserAlreadyExists)
}

func TestGetUser_NotFound(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	cache := &mocks.DummyCache{}
	events := &mocks.DummyPublisher{}
	service := NewUserService(repo, cache, events)

	_, err := service.GetUser(context.Background(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestUpdateUser_Success(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	cache := &mocks.DummyCache{}
	events := &mocks.DummyPublisher{}
	service := NewUserService(repo, cache, events)

	user, _ := service.CreateUser(context.Background(), "update@example.com", "Ana", time.Now())
	user.Nombre = "Ana Actualizada"

	err := service.UpdateUser(context.Background(), user)
	assert.NoError(t, err)

	// Comprobar que se actualizó en el repo
	u2, _ := repo.GetByID(context.Background(), user.ID)
	assert.Equal(t, "Ana Actualizada", u2.Nombre)

	// ✅ Verificar que se creó un evento Outbox adicional
	assert.Len(t, repo.Outbox, 2)
	assert.Equal(t, "user.updated", repo.Outbox[1].EventType)
	assert.Equal(t, user.ID.String(), repo.Outbox[1].AggregateID)
}

func TestDeleteUser_Success(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	cache := &mocks.DummyCache{}
	events := &mocks.DummyPublisher{}
	service := NewUserService(repo, cache, events)

	user, _ := service.CreateUser(context.Background(), "delete@example.com", "Borrar", time.Now())

	err := service.DeleteUser(context.Background(), user.ID)
	assert.NoError(t, err)

	// Verificar que el usuario fue eliminado
	_, err = repo.GetByID(context.Background(), user.ID)
	assert.ErrorIs(t, err, domain.ErrUserNotFound)

	// ✅ Verificar que se creó un evento Outbox adicional
	assert.Len(t, repo.Outbox, 2)
	assert.Equal(t, "user.deleted", repo.Outbox[1].EventType)
	assert.Equal(t, user.ID.String(), repo.Outbox[1].AggregateID)
}

// -------------------- GetUser con Cache --------------------
func TestGetUser_CacheHit(t *testing.T) {
	id := uuid.New()
	user := &domain.User{
		ID:     id,
		Email:  "cache@example.com",
		Nombre: "CacheUser",
	}

	cache := mocks.NewDummyCache()
	cache.SetForTest(domain.CacheKeyByID(id), user) // método de test que inserta directamente

	repo := mocks.NewInMemoryUserRepo()
	service := NewUserService(repo, cache, nil)

	u, err := service.GetUser(context.Background(), id)
	assert.NoError(t, err)
	assert.NotNil(t, u)
	assert.Equal(t, "CacheUser", u.Nombre)
}

func TestGetUser_CacheMiss(t *testing.T) {
	id := uuid.New()
	user := &domain.User{
		ID:     id,
		Email:  "miss@example.com",
		Nombre: "MissUser",
	}

	repo := mocks.NewInMemoryUserRepo()
	repo.Create(context.Background(), user, domain.OutboxEvent{})
	cache := mocks.NewDummyCache() // cache vacía

	service := NewUserService(repo, cache, nil)

	u, _ := service.GetUser(context.Background(), id)
	assert.NotNil(t, u)
	assert.Equal(t, id, u.ID)
}

// ----------------- ListUsers / Search / Filter -----------------

func TestSearchUsersByName(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	user1 := &domain.User{ID: uuid.New(), Nombre: "Ana"}
	user2 := &domain.User{ID: uuid.New(), Nombre: "Juan"}
	repo.Create(context.Background(), user1, domain.OutboxEvent{})
	repo.Create(context.Background(), user2, domain.OutboxEvent{})

	service := NewUserService(repo, nil, nil)

	results, err := service.SearchUsersByName(context.Background(), "Ana")
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Ana", results[0].Nombre)
}

func TestFilterUsers(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	user1 := &domain.User{ID: uuid.New(), Nombre: "Ana", Email: "ana@example.com"}
	user2 := &domain.User{ID: uuid.New(), Nombre: "Juan", Email: "juan@example.com"}
	repo.Create(context.Background(), user1, domain.OutboxEvent{})
	repo.Create(context.Background(), user2, domain.OutboxEvent{})

	service := NewUserService(repo, nil, nil)

	results, err := service.FilterUsers(context.Background(), 0, 100, "ana@example.com")
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Ana", results[0].Nombre)
}

func TestListUsers(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	cache := &mocks.DummyCache{}
	events := &mocks.DummyPublisher{}
	service := NewUserService(repo, cache, events)

	user1, _ := service.CreateUser(context.Background(), "a@example.com", "Ana", time.Now())
	user2, _ := service.CreateUser(context.Background(), "b@example.com", "Bob", time.Now())

	users, err := service.ListUsers(context.Background(), domain.UserFilter{})
	assert.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Contains(t, users, user1)
	assert.Contains(t, users, user2)
}
