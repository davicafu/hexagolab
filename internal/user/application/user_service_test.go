package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/davicafu/hexagolab/internal/user/domain"
	"github.com/davicafu/hexagolab/tests/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestCreateUser_Success(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	cache := &mocks.DummyCache{}
	events := &mocks.DummyPublisher{}
	service := NewUserService(repo, cache, events, zap.NewNop())

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
	service := NewUserService(repo, cache, events, zap.NewNop())

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
	service := NewUserService(repo, cache, events, zap.NewNop())

	_, err := service.GetUser(context.Background(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestUpdateUser_Success(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	cache := &mocks.DummyCache{}
	events := &mocks.DummyPublisher{}
	service := NewUserService(repo, cache, events, zap.NewNop())

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
	service := NewUserService(repo, cache, events, zap.NewNop())

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
	service := NewUserService(repo, cache, nil, zap.NewNop())

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

	service := NewUserService(repo, cache, nil, zap.NewNop())

	u, _ := service.GetUser(context.Background(), id)
	assert.NotNil(t, u)
	assert.Equal(t, id, u.ID)
}

// ----------------- ListUsers / Search / Filter -----------------

func TestListUsersByName(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	_ = repo.Create(context.Background(), &domain.User{ID: uuid.New(), Nombre: "Ana"}, domain.OutboxEvent{})
	_ = repo.Create(context.Background(), &domain.User{ID: uuid.New(), Nombre: "Juan"}, domain.OutboxEvent{})

	service := NewUserService(repo, nil, nil, zap.NewNop())

	criteria := domain.CompositeCriteria{
		Operator: domain.OpAnd,
		Criterias: []domain.Criteria{
			domain.NameLikeCriteria{Name: "Ana"},
		},
	}

	results, err := service.ListUsers(
		context.Background(),
		criteria,
		domain.OffsetPagination{Limit: 10, Offset: 0},
		domain.Sort{Field: "created_at", Desc: false},
	)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Ana", results[0].Nombre)
}

func TestListUsers(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	cache := &mocks.DummyCache{}
	events := &mocks.DummyPublisher{}
	service := NewUserService(repo, cache, events, zap.NewNop())

	user1, _ := service.CreateUser(context.Background(), "a@example.com", "Ana", time.Now())
	user2, _ := service.CreateUser(context.Background(), "b@example.com", "Bob", time.Now())

	criteria := domain.CompositeCriteria{Operator: domain.OpAnd, Criterias: []domain.Criteria{}}

	users, err := service.ListUsers(
		context.Background(),
		criteria,
		domain.OffsetPagination{Limit: 20, Offset: 0},
		domain.Sort{Field: "created_at", Desc: false},
	)
	assert.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Contains(t, users, user1)
	assert.Contains(t, users, user2)
}

func TestListAdultUsers(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	cache := &mocks.DummyCache{}
	events := &mocks.DummyPublisher{}
	service := NewUserService(repo, cache, events, zap.NewNop())

	// Crear usuarios de distintas edades
	// BirthDate calculado para que user1 tenga 20 años y user2 tenga 15 años
	user1, _ := service.CreateUser(context.Background(), "a@example.com", "Ana", time.Now().AddDate(-20, 0, 0))
	user2, _ := service.CreateUser(context.Background(), "b@example.com", "Bob", time.Now().AddDate(-15, 0, 0))
	user3, _ := service.CreateUser(context.Background(), "c@example.com", "Carlos", time.Now().AddDate(-25, 0, 0))

	users, err := service.ListAdultUsers(
		context.Background(),
		domain.OffsetPagination{Limit: 10, Offset: 0},
		domain.Sort{Field: "created_at", Desc: false},
	)
	assert.NoError(t, err)

	// Debe devolver solo los usuarios mayores de 18 años
	assert.Len(t, users, 2)
	assert.Contains(t, users, user1)
	assert.Contains(t, users, user3)
	assert.NotContains(t, users, user2)
}

func TestListUsers_PaginationOffsetAndSorting(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	service := NewUserService(repo, nil, nil, zap.NewNop())

	// Crear 5 usuarios con distintos nombres y emails
	users := []*domain.User{
		{ID: uuid.New(), Nombre: "Ana", Email: "ana@example.com", CreatedAt: time.Now().Add(-5 * time.Hour)},
		{ID: uuid.New(), Nombre: "Bob", Email: "bob@example.com", CreatedAt: time.Now().Add(-4 * time.Hour)},
		{ID: uuid.New(), Nombre: "Carlos", Email: "carlos@example.com", CreatedAt: time.Now().Add(-3 * time.Hour)},
		{ID: uuid.New(), Nombre: "Dave", Email: "Dave@example.com", CreatedAt: time.Now().Add(-2 * time.Hour)},
		{ID: uuid.New(), Nombre: "Eva", Email: "eva@example.com", CreatedAt: time.Now().Add(-1 * time.Hour)},
	}
	for _, u := range users {
		_ = repo.Create(context.Background(), u, domain.OutboxEvent{})
	}

	criteria := domain.CompositeCriteria{Operator: domain.OpAnd, Criterias: []domain.Criteria{}}

	// --- 1. Paginación: Offset + Limit ---
	page1, err := service.ListUsers(
		context.Background(),
		criteria,
		domain.OffsetPagination{Limit: 2, Offset: 0},
		domain.Sort{Field: "nombre", Desc: false},
	)
	assert.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.Equal(t, "Ana", page1[0].Nombre)
	assert.Equal(t, "Bob", page1[1].Nombre)

	page2, err := service.ListUsers(
		context.Background(),
		criteria,
		domain.OffsetPagination{Limit: 2, Offset: 2},
		domain.Sort{Field: "nombre", Desc: false},
	)
	assert.NoError(t, err)
	assert.Len(t, page2, 2)
	assert.Equal(t, "Carlos", page2[0].Nombre)
	assert.Equal(t, "Dave", page2[1].Nombre)

	// --- 2. Orden descendente ---
	descUsers, err := service.ListUsers(
		context.Background(),
		criteria,
		domain.OffsetPagination{Limit: 5, Offset: 0},
		domain.Sort{Field: "nombre", Desc: true},
	)
	assert.NoError(t, err)
	assert.Equal(t, "Eva", descUsers[0].Nombre)
	assert.Equal(t, "Dave", descUsers[1].Nombre)
	assert.Equal(t, "Carlos", descUsers[2].Nombre)
	assert.Equal(t, "Bob", descUsers[3].Nombre)
	assert.Equal(t, "Ana", descUsers[4].Nombre)

	// --- 3. Filtro combinado + paginación ---
	filterCriteria := domain.CompositeCriteria{
		Operator: domain.OpAnd,
		Criterias: []domain.Criteria{
			domain.NameLikeCriteria{Name: "a"}, // Ana, Carlos, Dave, Eva
		},
	}
	filteredPage, err := service.ListUsers(
		context.Background(),
		filterCriteria,
		domain.OffsetPagination{Limit: 2, Offset: 1},
		domain.Sort{Field: "nombre", Desc: false},
	)
	assert.NoError(t, err)
	assert.Len(t, filteredPage, 2)
	assert.Equal(t, "Carlos", filteredPage[0].Nombre)
	assert.Equal(t, "Dave", filteredPage[1].Nombre)

	// --- 4. Offset fuera de rango → array vacío ---
	outOfRange, err := service.ListUsers(
		context.Background(),
		criteria,
		domain.OffsetPagination{Limit: 2, Offset: 10},
		domain.Sort{Field: "nombre", Desc: false},
	)
	assert.NoError(t, err)
	assert.Len(t, outOfRange, 0)
}

func TestListUsers_CursorPagination(t *testing.T) {
	repo := mocks.NewInMemoryUserRepo()
	service := NewUserService(repo, nil, nil, zap.NewNop())

	// Crear 5 usuarios con distintos nombres y created_at
	users := []*domain.User{
		{ID: uuid.New(), Nombre: "Ana", CreatedAt: time.Now().Add(-5 * time.Hour)},
		{ID: uuid.New(), Nombre: "Bob", CreatedAt: time.Now().Add(-4 * time.Hour)},
		{ID: uuid.New(), Nombre: "Carlos", CreatedAt: time.Now().Add(-3 * time.Hour)},
		{ID: uuid.New(), Nombre: "Dave", CreatedAt: time.Now().Add(-2 * time.Hour)},
		{ID: uuid.New(), Nombre: "Eva", CreatedAt: time.Now().Add(-1 * time.Hour)},
	}
	for _, u := range users {
		_ = repo.Create(context.Background(), u, domain.OutboxEvent{})
	}

	criteria := domain.CompositeCriteria{Operator: domain.OpAnd, Criterias: []domain.Criteria{}}

	// Helper para construir cursor compuesto
	buildCursor := func(u *domain.User, sortField string) string {
		var val string
		switch sortField {
		case "nombre":
			val = u.Nombre
		case "email":
			val = u.Email
		case "created_at":
			val = u.CreatedAt.Format(time.RFC3339Nano) // coincide con el mock
		default:
			val = u.ID.String()
		}
		return fmt.Sprintf("%s|%s", val, u.ID.String())
	}

	// --- 1. Primer "page" usando cursor vacío ---
	cursor := ""
	page1, err := service.ListUsers(
		context.Background(),
		criteria,
		domain.CursorPagination{
			Limit:     2,
			Cursor:    cursor,
			SortField: "created_at",
			SortDesc:  false,
		},
		domain.Sort{Field: "created_at", Desc: false},
	)
	assert.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.Equal(t, "Ana", page1[0].Nombre)
	assert.Equal(t, "Bob", page1[1].Nombre)

	// --- 2. Segunda "page" usando cursor del último elemento de la primera ---
	cursor = buildCursor(page1[len(page1)-1], "created_at")
	page2, err := service.ListUsers(
		context.Background(),
		criteria,
		domain.CursorPagination{
			Limit:     2,
			Cursor:    cursor,
			SortField: "created_at",
			SortDesc:  false,
		},
		domain.Sort{Field: "created_at", Desc: false},
	)
	assert.NoError(t, err)
	assert.Len(t, page2, 2)
	assert.Equal(t, "Carlos", page2[0].Nombre)
	assert.Equal(t, "Dave", page2[1].Nombre)

	// --- 3. Última "page" (menos elementos que limit) ---
	cursor = buildCursor(page2[len(page2)-1], "created_at")
	page3, err := service.ListUsers(
		context.Background(),
		criteria,
		domain.CursorPagination{
			Limit:     2,
			Cursor:    cursor,
			SortField: "created_at",
			SortDesc:  false,
		},
		domain.Sort{Field: "created_at", Desc: false},
	)
	assert.NoError(t, err)
	assert.Len(t, page3, 1)
	assert.Equal(t, "Eva", page3[0].Nombre)

	// --- 4. Orden descendente usando cursor ---
	cursor = ""
	descPage, err := service.ListUsers(
		context.Background(),
		criteria,
		domain.CursorPagination{
			Limit:     3,
			Cursor:    cursor,
			SortField: "created_at",
			SortDesc:  true,
		},
		domain.Sort{Field: "created_at", Desc: true},
	)
	assert.NoError(t, err)
	assert.Len(t, descPage, 3)
	assert.Equal(t, "Eva", descPage[0].Nombre)
	assert.Equal(t, "Dave", descPage[1].Nombre)
	assert.Equal(t, "Carlos", descPage[2].Nombre)
}
