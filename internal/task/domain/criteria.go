// en internal/task/domain/task_criteria.go
package domain

import (
	"time"

	"github.com/google/uuid"
	// Importamos el "sistema" de Criterios genérico y le damos un alias
	shared "github.com/davicafu/hexagolab/internal/shared/domain"
)

// --- Criterios Específicos para el Dominio Task ---

// StatusCriteria busca tareas por su estado (pending, completed, etc.).
type StatusCriteria struct {
	Status TaskStatus
}

// ToConditions implementa la interfaz shared.Criteria.
func (c StatusCriteria) ToConditions() []shared.Criterion {
	return []shared.Criterion{
		{Field: "status", Op: shared.OpEq, Value: c.Status},
	}
}

// -----------------------------------------------------------

// AssigneeIDCriteria busca tareas asignadas a un usuario específico.
type AssigneeIDCriteria struct {
	ID uuid.UUID
}

// ToConditions implementa la interfaz shared.Criteria.
func (c AssigneeIDCriteria) ToConditions() []shared.Criterion {
	return []shared.Criterion{
		{Field: "assignee_id", Op: shared.OpEq, Value: c.ID},
	}
}

// -----------------------------------------------------------

// TitleLikeCriteria busca tareas cuyo título contenga un texto.
type TitleLikeCriteria struct {
	Title string
}

// ToConditions implementa la interfaz shared.Criteria.
func (c TitleLikeCriteria) ToConditions() []shared.Criterion {
	return []shared.Criterion{
		// Usamos ILIKE para búsquedas insensibles a mayúsculas/minúsculas
		{Field: "title", Op: shared.OpILike, Value: "%" + c.Title + "%"},
	}
}

// -----------------------------------------------------------

// CreatedAtRangeCriteria busca tareas creadas en un rango de fechas.
// Usamos punteros para que los filtros de fecha de inicio y fin sean opcionales.
type CreatedAtRangeCriteria struct {
	Start *time.Time
	End   *time.Time
}

// ToConditions implementa la interfaz shared.Criteria.
func (c CreatedAtRangeCriteria) ToConditions() []shared.Criterion {
	var conds []shared.Criterion
	if c.Start != nil {
		conds = append(conds, shared.Criterion{Field: "created_at", Op: shared.OpGte, Value: *c.Start})
	}
	if c.End != nil {
		conds = append(conds, shared.Criterion{Field: "created_at", Op: shared.OpLte, Value: *c.End})
	}
	return conds
}
