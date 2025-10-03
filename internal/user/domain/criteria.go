package domain

import (
	"time"

	"github.com/google/uuid"
)

// ---------------- Operadores ----------------

type Operator string

const (
	OpEq    Operator = "="
	OpGt    Operator = ">"
	OpGte   Operator = ">="
	OpLt    Operator = "<"
	OpLte   Operator = "<="
	OpLike  Operator = "LIKE"
	OpILike Operator = "ILIKE"
)

type LogicalOperator string

const (
	OpAnd LogicalOperator = "AND"
	OpOr  LogicalOperator = "OR"
)

// ---------------- Criterion ----------------

// Criterion describe una condición neutral de filtrado
type Criterion struct {
	Field string
	Op    Operator
	Value interface{}
}

// ---------------- Criteria interface ----------------

// Criteria permite transformar filtros a condiciones neutrales
type Criteria interface {
	ToConditions() []Criterion
}

// ---------------- Implementaciones concretas ----------------

// Filtrado por ID exacto
type IDCriteria struct {
	ID uuid.UUID
}

func (c IDCriteria) ToConditions() []Criterion {
	return []Criterion{{Field: "id", Op: OpEq, Value: c.ID}}
}

// Filtrado por email exacto
type EmailCriteria struct {
	Email string
}

func (c EmailCriteria) ToConditions() []Criterion {
	return []Criterion{{Field: "email", Op: OpEq, Value: c.Email}}
}

// Filtrado por nombre LIKE / ILIKE
type NameLikeCriteria struct {
	Name string
}

func (c NameLikeCriteria) ToConditions() []Criterion {
	return []Criterion{{Field: "nombre", Op: OpILike, Value: "%" + c.Name + "%"}}
}

// Filtrado por rango de edad
type AgeRangeCriteria struct {
	Min *int
	Max *int
}

func (c AgeRangeCriteria) ToConditions() []Criterion {
	var now time.Time = time.Now()
	var conds []Criterion
	if c.Min != nil {
		conds = append(conds, Criterion{
			Field: "birth_date",
			Op:    OpLte,
			Value: now.AddDate(-*c.Min, 0, 0),
		})
	}
	if c.Max != nil {
		conds = append(conds, Criterion{
			Field: "birth_date",
			Op:    OpGte,
			Value: now.AddDate(-*c.Max, 0, 0),
		})
	}
	return conds
}

// ---------------- Composite Criteria ----------------

type CompositeCriteria struct {
	Operator  LogicalOperator
	Criterias []Criteria
}

func (c CompositeCriteria) ToConditions() []Criterion {
	var all []Criterion
	for _, crit := range c.Criterias {
		all = append(all, crit.ToConditions()...)
	}
	return all
}

// ---------------- Helpers ----------------

// And crea un CompositeCriteria con operador AND
func And(criterias ...Criteria) CompositeCriteria {
	return CompositeCriteria{Operator: OpAnd, Criterias: criterias}
}

// Or crea un CompositeCriteria con operador OR
func Or(criterias ...Criteria) CompositeCriteria {
	return CompositeCriteria{Operator: OpOr, Criterias: criterias}
}
