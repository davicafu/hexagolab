package events

import (
	"time"

	"github.com/google/uuid"
)

// Estos son contratos de integración, NO entidades del dominio
// Se definen planos para intercambio entre contextos.
type UserCreated struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Nombre    string    `json:"nombre"`
	BirthDate time.Time `json:"birth_date"`
}

type UserUpdated struct {
	ID     uuid.UUID `json:"id"`
	Email  string    `json:"email"`
	Nombre string    `json:"nombre"`

	BirthDate time.Time `json:"birth_date"`
}
