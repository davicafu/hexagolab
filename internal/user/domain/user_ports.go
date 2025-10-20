package domain

import (
	"context"
	"errors"
	"fmt"

	sharedDomain "github.com/davicafu/hexagolab/internal/shared/domain"
	sharedQuery "github.com/davicafu/hexagolab/internal/shared/infra/platform/query"
	"github.com/google/uuid"
)

// ---------- Errores de dominio ----------
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidUser       = errors.New("invalid user")
)

// ---------- Interfaces (Ports) ----------

// UserRepository define las operaciones persistentes para User.
type UserRepository interface {
	// Debe devolver ErrUserAlreadyExists si la entidad ya existe (según la política del repo).
	Create(ctx context.Context, u *User, event sharedDomain.OutboxEvent) error

	// Debe devolver ErrUserNotFound si no existe.
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)

	// Debe devolver ErrUserNotFound si el usuario no existe.
	Update(ctx context.Context, u *User, evt sharedDomain.OutboxEvent) error

	// Debe devolver ErrUserNotFound si el usuario no existe.
	DeleteByID(ctx context.Context, id uuid.UUID, evt sharedDomain.OutboxEvent) error

	// List devuelve una lista de usuarios según el filtro (paginación, búsqueda, orden).
	// Si el filtro está vacío, debe devolver todos los usuarios.
	ListByCriteria(ctx context.Context, criteria sharedDomain.Criteria, pagination sharedQuery.Pagination, sort sharedQuery.Sort) ([]*User, error)
}

// ---------- Helpers comunes (cache keys, etc.) ----------

// CacheKeyByID forma una key consistente para cache usando ID.
func UserCacheKeyByID(id uuid.UUID) string {
	return fmt.Sprintf("user:id:%s", id.String())
}
