package domain

import (
	"context"
	"errors"
	"fmt"

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
	Create(ctx context.Context, u *User, event OutboxEvent) error

	// Debe devolver ErrUserNotFound si no existe.
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)

	// Debe devolver ErrUserNotFound si el usuario no existe.
	Update(ctx context.Context, u *User, evt OutboxEvent) error

	// Debe devolver ErrUserNotFound si el usuario no existe.
	DeleteByID(ctx context.Context, id uuid.UUID, evt OutboxEvent) error

	// List devuelve una lista de usuarios según el filtro (paginación, búsqueda, orden).
	// Si el filtro está vacío, debe devolver todos los usuarios.
	List(ctx context.Context, f UserFilter) ([]*User, error)

	// FetchPendingOutbox obtiene los eventos no procesados, hasta un máximo
	FetchPendingOutbox(ctx context.Context, limit int) ([]OutboxEvent, error)

	// MarkOutboxProcessed marca un evento como procesado
	MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error
}

type UserCache interface {
	// Get intenta poblar dest (puntero) con el valor asociado a la key.
	// Devuelve (true, nil) si hay hit y dest fue rellenado.
	// Devuelve (false, nil) si es miss.
	Get(ctx context.Context, key string, dest interface{}) (bool, error)

	// Set serializa y guarda el valor con TTL en segundos.
	Set(ctx context.Context, key string, val interface{}, ttlSecs int) error

	// Delete elimina la key del cache.
	Delete(ctx context.Context, key string) error
}

// La semántica de topic/nombre y formato del payload la decides en los adapters.
type EventPublisher interface {
	Publish(ctx context.Context, topic string, event interface{}) error
}

// ---------- Tipos de filtrado / paginación / ordenamiento ----------

// Pagination describe límite y offset.
type Pagination struct {
	Limit  int
	Offset int
}

// Sort indica campo y dirección.
type Sort struct {
	Field string // ej. "created_at", "nombre", "email"
	Desc  bool
}

// UserFilter agrupa criterios de búsqueda que puede usar UserRepository.List.
type UserFilter struct {
	// Búsquedas básicas
	ID     *uuid.UUID // si se pasa, filtra por ID exacto
	Email  *string    // búsqueda por email (opcional)
	Nombre *string    // búsqueda por nombre (puede interpretarse como LIKE en el repo)

	// Rangos por edad (calcular con BirthDate en el repo)
	MinAge *int
	MaxAge *int

	// Paginación y orden
	Pagination Pagination
	Sort       Sort
}

// ---------- Helpers comunes (cache keys, etc.) ----------

// CacheKeyByID forma una key consistente para cache usando ID.
func CacheKeyByID(id uuid.UUID) string {
	return fmt.Sprintf("user:id:%s", id.String())
}
