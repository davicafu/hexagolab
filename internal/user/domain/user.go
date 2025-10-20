package domain

import (
	"time"

	sharedBus "github.com/davicafu/hexagolab/internal/shared/infra/platform/bus"
	"github.com/google/uuid"
)

// User representa un usuario del sistema.
type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Nombre    string    `json:"nombre"`
	BirthDate time.Time `json:"birth_date"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *User) PartitionKey() string {
	return u.ID.String()
}

// Age calcula la edad del usuario a partir de su fecha de nacimiento.
func (u *User) Age() int {
	now := time.Now()
	years := now.Year() - u.BirthDate.Year()
	if now.YearDay() < u.BirthDate.YearDay() {
		years--
	}
	return years
}

// Verificación estática para asegurar que User implementa la interfaz
var _ sharedBus.Keyer = (*User)(nil)
