package domain

import (
	"time"

	sharedDomain "github.com/davicafu/hexagolab/internal/shared/domain"
	"github.com/google/uuid"
)

// ---------------- Implementaciones concretas ----------------

// Filtrado por ID exacto
type IDCriteria struct {
	ID uuid.UUID
}

func (c IDCriteria) ToConditions() []sharedDomain.Criterion {
	return []sharedDomain.Criterion{{Field: "id", Op: sharedDomain.OpEq, Value: c.ID}}
}

// Filtrado por email exacto
type EmailCriteria struct {
	Email string
}

func (c EmailCriteria) ToConditions() []sharedDomain.Criterion {
	return []sharedDomain.Criterion{{Field: "email", Op: sharedDomain.OpEq, Value: c.Email}}
}

// Filtrado por nombre LIKE / ILIKE
type NameLikeCriteria struct {
	Name string
}

func (c NameLikeCriteria) ToConditions() []sharedDomain.Criterion {
	return []sharedDomain.Criterion{{Field: "nombre", Op: sharedDomain.OpILike, Value: "%" + c.Name + "%"}}
}

// Filtrado por rango de edad
type AgeRangeCriteria struct {
	Min *int
	Max *int
}

func (c AgeRangeCriteria) ToConditions() []sharedDomain.Criterion {
	var now time.Time = time.Now()
	var conds []sharedDomain.Criterion
	if c.Min != nil {
		conds = append(conds, sharedDomain.Criterion{
			Field: "birth_date",
			Op:    sharedDomain.OpLte,
			Value: now.AddDate(-*c.Min, 0, 0),
		})
	}
	if c.Max != nil {
		conds = append(conds, sharedDomain.Criterion{
			Field: "birth_date",
			Op:    sharedDomain.OpGte,
			Value: now.AddDate(-*c.Max, 0, 0),
		})
	}
	return conds
}
